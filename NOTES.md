# Studio Script Sync — what we actually learned

User-facing how-to is `START.md`. This file is the reverse-engineering log so we do not re-learn it. Observations are from **Roblox Studio 0.738** on **macOS arm64** (2026-09-12), place `place.rbxlx`, repo path often a symlink (`/Users/…/repos/misc` → `…/Documents/repos/misc`).

Official docs: [Script Sync](https://create.roblox.com/docs/scripting/sync) · [Studio MCP](https://create.roblox.com/docs/studio/mcp) · [Studio CLI](https://create.roblox.com/docs/studio/command-line-interface).

---

## Split of ownership

| What | Where it lives | How it moves |
|---|---|---|
| Luau | files on disk (`src/…`) | Studio **Script Sync** (two-way) |
| World / instances / lighting | `place.rbxlx` | Studio **File → Save**. Official MCP **cannot** save to a path |
| Playtest, inspect, meshes, inserts | live DataModel | Studio **MCP** |
| Script Sync UI settings (auto-resume, Keep Local, `.luau`) | `~/Library/Roblox/GlobalSettings_13.xml` | per **machine**, not in the place file |
| Per-place “which folder is bound to which disk dir” | macOS `~/Library/Preferences/com.roblox.RobloxStudio.plist` | per **machine + place path**, not in `place.rbxlx` |

There is **no public `StartSync`**. `InstanceFileSyncService` is query-only (`GetStatus`, `GetSyncedInstance`, `GetAllInstances`). Explorer **Sync to…** is the UI; auto-resume is the only start path we can drive from outside.

Script Sync will sync **Script, LocalScript, ModuleScript, Folder** only. Scripts parented under Parts/GUIs/etc. will not be picked up.

---

## CLI flow (`./robld`)

1. Write MCP config (`.mcp.json`, `.grok/config.toml`, `.codex/config.toml`).
2. Ensure `robuild-sync.json` (do not overwrite a custom map; migrate the old generated Folder map — see below).
3. Install `~/Documents/Roblox/Plugins/robuild_agent.lua`.
4. If Studio is running **and** prefs/plugin/persist need a write: best-effort Cmd+S, quit, continue.
5. Patch `GlobalSettings_13.xml` Script Sync prefs (Studio must be closed for a reliable write).
6. Write plist resume records (`defaults write` + `killall cfprefsd`).
7. Launch Studio with `--task EditFile --localPlaceFile <evalsymlinks path>`.
8. Wait until logs show resume **or** the plugin reports `SyncedAsRoot`.

Exit `0` READY, `1` NEED_PLACE/ERROR, `2` NOT_READY (user must Sync to… once).

`robld save` (macOS): Accessibility keystrokes, File → Save / Cmd+S, then check `place.rbxlx` mtime. Does **not** restart Studio.

---

## Prefs (`GlobalSettings_13.xml`)

Item class `Studio`, properties:

| Property | Token/bool we want | UI |
|---|---|---|
| `AutoResumeSyncOnPlaceOpen` | `true` | Auto resume sync on place open |
| `ActionOnAutoResumeSync` | `2` | Always keep local (disk wins on resume conflict) |
| `ActionOnStopSync` | `1` | Keep local files after Stop Sync |
| `DefaultScriptSyncFileType` | `1` | `.luau` |
| `ReloadLocalPluginsOnChange` | `true` | so `robuild_agent.lua` reloads |

`robld prefs` / `robld prefs apply`. Main `robld` applies the same set when it **launches** Studio.

A user plugin **cannot** set most of these at runtime (`lacking capability RobloxScript` / `RobloxEngine`). XML patch is the real write. `ActionOnStopSync` did succeed from the plugin once; do not rely on that.

---

## Resume store (the thing that actually starts sync)

**Not** in `place.rbxlx`, `placeIDEState`, `rbx-storage.db`, or `appStorage.json`.

macOS domain **`com.roblox.RobloxStudio`**
file `~/Library/Preferences/com.roblox.RobloxStudio.plist`

### Key

```
File_Sync_Persistence_Record_V1:<encoded-place-path>:<workspace-uuid>
```

Place path encoding (confirmed):

- Take the **EvalSymlinks** absolute path (Studio opens the resolved file, not the symlink).
- `/Users/foo/Documents/repos/misc/place.rbxlx`
- → `.Users.foo.Documents.repos.misc.place·rbxlx`
- slashes → dots; **last** `.` of the filename (the extension dot) → middle dot **U+00B7** (`·`); prefix `.`.

Workspace UUID is the **Workspace** instance `UniqueId` (not DataModel), 32 hex chars in rbxlx → `8-4-4-4-12` lowercase.

Example that worked:

```
File_Sync_Persistence_Record_V1:.Users.aduermael.Documents.repos.misc.place·rbxlx:684fc65d-7df2-17a3-0ab7-26ca00000002
```

Sibling keys on the same prefix:

- `_lastUsedDir` — parent of the first binding’s `filePath` (string)
- `_timeLastUsed` — unix **milliseconds** (integer)

Also:

- `File_Sync_Persistence_SafetyBit` — bool. Observed **False** both after a successful UI Sync to… and after our write. We write `false`. Meaning is not fully known; do not set `true` without a new dump.

### Value (JSON array, pretty-printed 4-space indent + trailing newline)

```json
[
    {
        "className": "ServerScriptService",
        "filePath": "/Users/aduermael/Documents/repos/misc/src/server",
        "scriptId": "684fc65d-7df2-17a3-0ab7-26ca000003ba",
        "status": "Syncing"
    },
    {
        "className": "ReplicatedStorage",
        "filePath": "/Users/aduermael/Documents/repos/misc/src/shared",
        "scriptId": "684fc65d-7df2-17a3-0ab7-26ca000003b9",
        "status": "Syncing"
    },
    {
        "className": "StarterPlayerScripts",
        "filePath": "/Users/aduermael/Documents/repos/misc/src/client",
        "scriptId": "684fc65d-7df2-17a3-0ab7-26ca000003da",
        "status": "Syncing"
    }
]
```

Field meanings:

- `className` — instance **ClassName** (exact). `DFFlagFileSyncExactClassMatch` is True.
- `filePath` — EvalSymlinks absolute disk directory.
- `scriptId` — that instance’s UniqueId as UUID. For SSS/RS/SPS these share the place’s UniqueId prefix (`684fc65d-…`).
- `status` — `"Syncing"` is what Studio itself writes.

### How to write so Studio sees it

Writing the plist file with Python/`plutil` **does not stick**. `cfprefsd` overwrites it.

Must:

```
defaults write com.roblox.RobloxStudio <key> -string '<json>'
defaults write … _lastUsedDir -string …
defaults write … _timeLastUsed -integer <unix-ms>
defaults write … File_Sync_Persistence_SafetyBit -bool false
killall cfprefsd
sleep ~300ms
then launch Studio
```

Go `exec.Command` (no shell) is fine with spaces/quotes in the JSON.

Windows: **not implemented**. Resume records are macOS-only in this CLI today (`persist_other.go`).

---

## Service roots vs Folder roots (the bug that blocked us)

Studio **will** auto-resume records whose `className` is a service (or service-like container):

- `ServerScriptService`
- `ReplicatedStorage`
- `StarterPlayerScripts` (child of StarterPlayer; ClassName is still `StarterPlayerScripts`)

Studio **did not** auto-resume synthetic records with `className: "Folder"` even when:

- UniqueIds matched Folders in `place.rbxlx`
- `filePath` existed
- prefs auto-resume + KeepLocal were on
- the plist key was correct

Evidence:

- UI Sync to… on **whole ReplicatedStorage** → Studio wrote a service record → log `overwriting 1 synced hierarchies`.
- We replaced that with three Folder records (`SSS.src`, `RS.shared`, `SPS.src`) → next session logged **nothing** File_Sync-related (silent skip; `FFlagLDP719DontReportErrorsOnResume` is True).
- Service records for SSS/RS/SPS → log `overwriting 3 synced hierarchies` and plugin `SyncedAsRoot`.

`FFlagLDP788SyncServiceRoots` is True on this Studio.

Default `robuild-sync.json` therefore maps **services**, not nested Folders:

| Disk | Studio instance path |
|---|---|
| `src/server` | `ServerScriptService` |
| `src/shared` | `ReplicatedStorage` |
| `src/client` | `StarterPlayer.StarterPlayerScripts` |

The old generated map (`ServerScriptService.src` ↔ `src/server`, etc.) is rewritten in place when `robld` sees that exact triple (`migrateLegacyGreenfieldMap`). Custom maps are left alone.

KeepLocal + service bind **flattens** the DataModel to match disk: extra `src` / `shared` Folders under the service go away; `Hello.luau` becomes `ReplicatedStorage.Hello`. **Cmd+S** after first successful resume so `place.rbxlx` matches. Until then git still has the old Folders.

### Syncing a whole service from the UI

If the user Sync to… **ReplicatedStorage** into a folder also named `ReplicatedStorage`, Studio creates a nested disk tree `ReplicatedStorage/ReplicatedStorage/…`. That is leftover, not in `robuild-sync.json`. Safe to delete once the `src/shared` bind is live.

---

## UniqueId details

- rbxlx: `<UniqueId name="UniqueId">684fc65d7df217a30ab726ca000003b9</UniqueId>` (32 hex, no dashes).
- Plist `scriptId`: `684fc65d-7df2-17a3-0ab7-26ca000003b9`.
- Workspace UniqueId is the plist key suffix. Services in a Studio-made place share that prefix; plugin-created Folders in our test place used a **different** prefix (`67fac836-…`). Folder resume still failed even with those IDs in the file — className was the blocker, not the prefix.
- `FFlagLDP626FileSyncUseIdLookup` is True: resume looks up by UniqueId.

First launch can persist **without** a prior Save because SSS/RS/SPS already exist in a normal place. Nested Folders needed plugin-create → Save → second launch; that path is obsolete for the default map.

---

## Paths and symlinks

Launch and plist `filePath` must use `filepath.EvalSymlinks`. Studio’s place session id looked like:

```
…/Users/aduermael/Documents/repos/misc/place.rbxlx
```

even when the shell cwd was `/Users/aduermael/repos/misc`.

The plugin currently embeds the **non-resolved** project path (`/Users/…/repos/misc/src/server`). `GetSyncedInstance` still returned `mappedName` — Studio canonicalizes. Prefer EvalSymlinks in the plugin too if we touch it again.

---

## Plugin (`robuild_agent.lua`)

Installed as a **user** plugin: `~/Documents/Roblox/Plugins/robuild_agent.lua`. Studio logs it as `user_robuild_agent.lua`.

### Do not `game.Loaded:Wait()`

If `Loaded` already fired, that Wait hangs until the plugin is unloaded. Then `warn` / `SetSetting` never run. Symptom: `PluginLoadingEnhanced Running plugin … took 0.3ms` and no `ROBUILD_*` lines.

Use `task.defer` / `task.delay` only. Studio Edit can `GetService` immediately.

Emit without creating instances first (let File_Sync resume bind). Create missing scripts a few seconds later if still absent.

### Logging

- `warn("ROBUILD_PLUGIN_LOADED")` and `warn("ROBUILD_JSON:"..encoded)` → `FLog::CreatorWarning` (this **is** in `~/Library/Logs/Roblox/*Studio*.log`).
- `print("ROBUILD_JSON:"..)` → `FLog::CreatorOutput` (also in the log).
- **FLog truncates each line at ~1022 characters.** A full `ROBUILD_JSON` does not parse from the log. `SyncedAsRoot` for the first root often still appears in the truncated prefix.
- `plugin:SetSetting("robuildDump", …)` did **not** land in `Documents/Roblox/0/InstalledPlugins/0/settings.json` (stayed `{}`). Do not wait on that file for user `.lua` plugins.

### Prefs from Lua

Most `settings().Studio[…] =` writes fail capability checks. XML patch remains source of truth.

### Reload

`ReloadLocalPluginsOnChange` fires `Detected add/change: user_robuild_agent.lua` if we rewrite the plugin around launch. Harmless but noisy; skip write when bytes are equal (already done).

Probing `StartSync` / friends on `InstanceFileSyncService` found no callable start method.

---

## How `robld` knows sync started

Wait loop (~90s, poll 3s):

1. Newest Studio log containing `ROBUILD_JSON:` **and** `SyncedAsRoot` (may fail to JSON-parse because of the 1022-char cap).
2. Plugin settings `robuildDump` (unreliable for user plugins — see above).
3. Log line **`overwriting N synced hierarchies`** — this is the engine signal that auto-resume actually ran. Working session: **N=3**.

`did not resume syncing` in a log must **not** count as success.

READY on the overwrite warning is what unblocked the successful run (`Studio logs show Script Sync resume.`).

---

## Dump (`./robld dump`)

Copies into `robuild-dump/`: newest Studio logs (force-copy, not keyword-gated), GlobalSettings, rbx-storage, plists, plugin lua, plugin settings, placeIDEState, REPORT.txt.

Always copy recent logs even if they have no keywords — a failed Folder-resume session had **zero** File_Sync lines and was previously dropped.

REPORT now includes the plist key plus want/have bindings (`defaults read` on darwin).

Leave the dump folder in the **game** repo when asking an agent to debug; it is gitignored in this CLI repo.

---

## Flags seen on this Studio (ClientSettings/StudioAppSettings.json)

Relevant True flags (not a complete list):

- `FFlagCollab7848InstanceFileSyncServiceEnabled`
- `FFlagLDP811UseInstanceFileSyncService2`
- `FFlagLDP626FileSyncUseIdLookup`
- `FFlagLDP788SyncServiceRoots`
- `FFlagLDP719DontReportErrorsOnResume` — resume failures are silent
- `FFlagLDP608WarnOnStartSync`
- `FFlagLDP1035FileSyncResumeOnPluginReload`
- `DFFlagFileSyncExactClassMatch`
- `FFlagScriptSyncM2_5BetaFeature`
- `FFlagLuaExplorerFileSync2`

---

## Known-good session (2026-09-12T13:01 PDT)

- Studio 0.738, `studioRunning=true`, lock on `place.rbxlx`.
- Prefs already correct in GlobalSettings.
- Plist want == have, three services, `status=Syncing`, SafetyBit False.
- Log `b52bd`: plugin loaded → `ROBUILD_PLUGIN_LOADED` → **overwriting 3 synced hierarchies** at t≈2.5s → `ROBUILD_JSON` with `SyncedAsRoot` + `mappedName` for all three (truncated).
- `place.rbxlx` mtime can lag until Cmd+S; live DM is already flattened.

---

## What we have not proven

- Windows / WSL resume store (where Studio keeps the equivalent of `File_Sync_Persistence_Record_V1`).
- Whether a **Folder** record that Studio itself wrote via UI Sync to… on that Folder (not the parent service) would auto-resume. We never captured a Studio-authored Folder JSON; we only know **our** Folder JSON did not start sync.
- Exact meaning of `File_Sync_Persistence_SafetyBit`.
- Team Create / cloud places (we only exercised a local `place.rbxlx`).
- Whether `status` must be `"Syncing"` vs `"SyncedAsRoot"` in the plist (Studio writes `"Syncing"`).
- A supported way to start sync without resume records (no public API found).

---

## Practical rules

1. Bind **services** in `robuild-sync.json` and in the plist, not wrapper Folders.
2. EvalSymlinks the place path and disk roots before writing plist keys / launching.
3. `defaults write` + `killall cfprefsd`; never raw-edit the plist.
4. Do not `Wait()` on `game.Loaded` in the plugin.
5. Treat `overwriting N synced hierarchies` as READY; do not require a parseable `ROBUILD_JSON`.
6. After first KeepLocal resume, **Save** so `place.rbxlx` matches disk.
7. `robld dump` after any sync mystery; read `REPORT.txt` plus the newest `*Studio*_last.log`.
