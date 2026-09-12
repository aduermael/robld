# robuild

Agent-first Roblox workflow: **Luau on disk**, **place file in git**, **Studio MCP for playtest and the world**. One Go CLI on macOS and Windows. Re-run until it prints `READY`.

```bash
go build -o robuild .
./robuild --new "My Game"
```

Windows:

```powershell
go build -o robuild.exe .
.\robuild.exe --new "My Game"
```

| Exit | Meaning |
|---|---|
| **0** `READY:` | Studio is on the place, Luau is in this folder, MCP config written. |
| **1** `NEED_PLACE:` / `ERROR:` | Pass `--new`, a game URL, or fix Studio install, then re-run. |
| **2** `NOT_READY:` | Do **Sync to…** in Explorer, then re-run `robuild`. |

`go run . -- --new` works too; a built `robuild` binary is the usual command.

### Persist the world (macOS)

MCP and Script Sync change the **open Studio session**, not `place.rbxlx`, until Studio saves. Official MCP cannot save to disk. `robuild save` hacks that: it focuses Roblox Studio and sends **File → Save** (falls back to Cmd+S). No restart, no rewriting XML.

```bash
./robuild save
```

Grant **Accessibility** to the app that runs `robuild` (Terminal / iTerm / Grok): System Settings → Privacy & Security → Accessibility. If `place.rbxlx` mtime does not change, exit 2 — click the Studio window and retry.

Use this after MCP world edits (parts, meshes, lighting). Luau still goes through Script Sync files; you do not need save for script-only commits.

### Script Sync preferences (this machine)

Studio Settings → Script Sync are **not** in `place.rbxlx`. They live in Studio’s `GlobalSettings_13.xml` (macOS `~/Library/Roblox/`, Windows `%LOCALAPPDATA%\Roblox\`).

```bash
./robuild prefs          # show current values
./robuild prefs apply    # write agent defaults (Studio must be closed)
```

`robuild` also applies those defaults when it **launches** Studio (not when Studio is already open). Agent defaults:

| Setting | Value |
|---|---|
| Auto resume sync on place open | on |
| Resume conflicted sync on place open | Always keep local |
| Keep local files/directories after Stop Sync | Keep local files |
| File extension | `.luau` |

The main `robuild` command:

- Writes those prefs (and **restarts Studio** on macOS if it was already open so they take).
- Writes **`robuild-sync.json`** — the per-project Explorer folder ↔ disk folder map. Edit this when a game uses different names (`src/server`, `ReplicatedStorage.Shared`, …). `robuild` will not overwrite a map you already saved.
- For an empty repo it seeds `src/server`, `src/shared`, `src/client` and maps them to **Studio services** (`ServerScriptService`, `ReplicatedStorage`, `StarterPlayer.StarterPlayerScripts`). Studio auto-resume binds those service UniqueIds; nested Folder records in the plist do not start sync.
- Installs a Studio plugin that creates missing scripts under those services and prints `ROBUILD_JSON` for `robuild` to wait on.
- When Studio is **closed**, writes Script Sync **resume records** into `~/Library/Preferences/com.roblox.RobloxStudio.plist` (service UniqueId ↔ disk path) so the next open can auto-resume without Sync to… in the UI. Services already exist in `place.rbxlx`, so the first launch can persist.

### Dump this machine for the agent

```bash
./robuild dump
```

Writes `robuild-dump/REPORT.txt` plus copies of GlobalSettings, placeIDEState, plists, and Script Sync log lines. Leave that folder in the project so the agent can read it. `./robuild scan` is the stdout-only version.

## What you version

| In git | Owned by |
|---|---|
| `*.luau` under synced folders | Script Sync (Studio ↔ disk) |
| `place.rbxlx` | Studio **Save** (Cmd+S / Ctrl+S) — maps, instances, lighting |
| `place.json` | `robuild` (ids, local file path, name) |
| `robuild-sync.json` | `robuild` (Studio folder ↔ disk folder map; per project) |

`robuild` always opens the **local** place file when `place.json` has `localPlaceFile`, so world edits can land in git. Cloud place/universe ids are optional (after **File → Publish to Roblox**).

## New game (agent from day one)

```bash
./robuild --new
./robuild --new "My Game"
```

Creates `place.rbxlx` in this folder, writes `place.json` with a **relative** path (clones work on Mac and Windows), opens Studio, writes MCP config.

Then `./robuild` until `READY`. Empty projects get `src/server` ↔ `ServerScriptService` (see `robuild-sync.json`). If auto-resume still does not bind, Studio needs **Sync to…** once per root (Keep Disk). After that, auto-resume + Keep local should bring it back.

Enable Studio as MCP server: Assistant → **…** → Manage MCP Servers.

## Existing cloud game

```bash
./robuild https://www.roblox.com/games/123456789/My-Game
./robuild --file ./place.rbxlx
```

If you also want that place as a git file: **File → Save to File** as `place.rbxlx` here, then `robuild` will prefer it.

## Everyday

```bash
./robuild
```

Opens the same place, resumes Script Sync, refreshes MCP config. Keep Studio open. Edit Luau in the agent; use MCP for playtest, screenshots, parts, assets. Save in Studio before you commit world changes.

Conflict dialog: **Keep Disk** = this repo wins; **Keep Studio** = the place wins.

Studio Settings → Script Sync: `robuild prefs apply` (Studio closed) so auto-resume stays on and conflicts keep the files in this repo.

Docs: [Script Sync](https://create.roblox.com/docs/scripting/sync) · [Studio MCP](https://create.roblox.com/docs/studio/mcp) · [Studio CLI](https://create.roblox.com/docs/studio/command-line-interface)

Studio internals we reverse-engineered (plist keys, service vs Folder resume, plugin Loaded hang, FLog truncation): **[NOTES.md](NOTES.md)**.
