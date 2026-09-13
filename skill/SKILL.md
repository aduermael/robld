---
name: robld
description: >
  Develop this Roblox game with robld, Studio Script Sync (Luau on disk),
  a git-tracked place.rbxlx, and Studio MCP (playtest, world, assets). Use
  when writing Luau, playtesting, inspecting the DataModel, inserting models,
  generating meshes, or working in Roblox Studio. Triggers: Roblox, Studio,
  Luau, Script Sync, playtest, MCP, DataModel, robld.
---

# Roblox: robld + Script Sync + Studio MCP

`robld` (say *robuild*) bootstraps Studio Script Sync and MCP so an agent can edit Luau on disk while Studio stays open for playtest and the world.

The skill in this folder is the how-to. If it is missing in a project, run `robld --install`.

## CLI

`robld` should already be on PATH (agent install from https://aduermael.github.io/robld/). Run it from the game folder. Same command every time. Prefer a release binary over building from source.

| Command | What it does |
|---|---|
| `robld` | Open the place in `place.json`, wait until Script Sync + MCP are up. Re-run until `READY:`. |
| `robld --new` / `robld --new "Name"` | Create a local `place.rbxlx` and `place.json`, then launch Studio. |
| `robld <place-id-or-url>` | Bind an existing cloud place (or `robld --file place.rbxlx`). |
| `robld --install` | Write this skill into the current project (Claude, Grok, Codex, Cursor, `.agents`). |
| `robld --version` | Print the build version embedded in this binary. |
| `robld --update` | Replace this binary with the latest GitHub release for this OS/arch **and** run `--install` so the skill matches that binary. |
| `robld save` | macOS: focus Studio and File→Save / Cmd+S so `place.rbxlx` updates. No restart. |
| `robld prefs` / `robld prefs apply` | Show or write this machine's Script Sync Studio Settings. |
| `robld dump` | Copy Studio settings/logs/sync clues into `robuild-dump/` for the agent to read. |
| `robld scan` | Stdout-only version of the dump clues. |
| `robld --help` | Usage text. |

If a command prints that a new version is available, run `robld --update`. Do not invent an install path; `--update` downloads the matching release asset and refreshes the skill.

Build from this repo (untagged builds report version `dev`):

```bash
cd cli && go build -o robld .
```

Windows: `cd cli` then `go build -o robld.exe .`.

## Bootstrap

Run `robld` from the game folder. Pass ids as args; do not prompt. Do not start Script Sync from MCP or Luau.

| Exit | What you do |
|---|---|
| 0 and `READY:` | Treat Script Sync + MCP as on. Continue the task. |
| 1 `NEED_PLACE:` | `robld --new` / `robld --new "Name"`, or a place id / game URL. Re-run. |
| 1 `ERROR:` | Show the message (usually Studio missing). Do not invent a connection. |
| 2 `NOT_READY:` | Show the printed Sync to… steps. After the user does them, re-run `robld`. |

Studio is required (macOS or Windows). If `robld` says Studio was not found, stop.

Script Sync **preferences** are per-machine; main `robld` writes them and may restart Studio. **`robuild-sync.json`** is the per-project folder map — honor it; games are not all `ServerScriptService/` on disk. Do not overwrite a custom map. The generated default maps `src/server|shared|client` to **services** (that is what Studio auto-resumes). New Luau goes next to existing siblings. Do not start Sync to… from MCP. If `robld` exits `NOT_READY`, show the printed Sync to… paths. `robld dump` → read `robuild-dump/REPORT.txt` and `rbx-storage.txt`.

## Git

If `git` is installed and this folder is not already a git repository, run `git init` and make an initial commit. History and versioning help a lot with any code project. Commit often as you work (Luau, `place.rbxlx`, and other project files). Do not force-push or rewrite history unless asked.

## What lives where

- **Luau:** files in this tree (Script Sync). Create/edit/delete on disk only.
- **World / instances:** `place.rbxlx`. After MCP creates parts/meshes, run `robld save` (macOS: File→Save / Cmd+S into Studio). Do not rewrite the XML or restart Studio to persist.
- **Ids / local path / name:** `place.json` (written by `robld`).
- **Folder map:** `robuild-sync.json` (Studio instance path ↔ disk folder). Edit this file for a different layout; `robld` will not overwrite an existing map.
- **MCP:** playtest, console, screenshots, inspect, meshes, marketplace inserts.

| In git | Owned by |
|---|---|
| `*.luau` under synced folders | Script Sync (Studio ↔ disk) |
| `place.rbxlx` | Studio **Save** — maps, instances, lighting |
| `place.json` | `robld` |
| `robuild-sync.json` | `robld` |

On-disk artifact names still use the `robuild-*` prefix. The command you type is `robld`.

`robld` always opens the **local** place file when `place.json` has `localPlaceFile`, so world edits can land in git. Cloud place/universe ids are optional (after **File → Publish to Roblox**).

## Scripts — disk only

| File on disk | Studio instance |
|---|---|
| `Name.luau` | ModuleScript |
| `Name.server.luau` | Script, RunContext Server |
| `Name.client.luau` | Script, RunContext Client |
| `Name.local.luau` | LocalScript |
| `Name.legacy.luau` | Script, RunContext Legacy |
| `Name/` | Folder |
| `Name/init.*.luau` | Script with children (type from the suffix) |

- Put new scripts next to existing siblings so they land under the already-synced Studio folder.
- MCP `script_read` / `script_search` / `script_grep` are fine for **reading** live source.
- Do not use MCP `multi_edit` (or `execute_luau` that assigns `Source`) on a script that lives on disk.
- Do not create scripts via MCP under parts, GUIs, or other non-Folder instances if they should be committed — Script Sync will not pick them up.

## MCP — Studio world and playtest

Every MCP call needs `studio_id`. Call `list_roblox_studios` once per session if you do not have it.

Use MCP for:

- Playtest: `start_stop_play`, `get_console_output`, `screen_capture`, `get_studio_state`
- Input in play: `character_navigation`, `user_keyboard_input`, `user_mouse_input`
- Inspect: `search_game_tree`, `inspect_instance`
- Queries: `execute_luau` with `datamodel_type` Edit (or Client/Server only while playing)
- World: instances, `insert_asset`, `search_asset`, `generate_mesh`, `generate_material`, `generate_procedural_model`

Enable Studio as MCP server: Assistant → **…** → Manage MCP Servers.

## Script Sync on this machine

Studio Settings → Script Sync are **not** in `place.rbxlx`. They live in Studio’s `GlobalSettings_13.xml` (macOS `~/Library/Roblox/`, Windows `%LOCALAPPDATA%\Roblox\`).

`robld` applies agent defaults when it **launches** Studio (not when Studio is already open). `robld prefs apply` writes them too (Studio must be closed). Defaults:

| Setting | Value |
|---|---|
| Auto resume sync on place open | on |
| Resume conflicted sync on place open | Always keep local |
| Keep local files/directories after Stop Sync | Keep local files |
| File extension | `.luau` |

The main `robld` command also:

- Writes **`robuild-sync.json`**. For an empty repo it seeds `src/server`, `src/shared`, `src/client` mapped to **Studio services** (`ServerScriptService`, `ReplicatedStorage`, `StarterPlayer.StarterPlayerScripts`). Studio auto-resume binds those service UniqueIds; nested Folder records in the plist do not start sync.
- Installs a Studio plugin that creates missing scripts under those services and prints `ROBUILD_JSON` for `robld` to wait on.
- When Studio is **closed**, writes Script Sync **resume records** into Studio preferences (service UniqueId ↔ disk path) so the next open can auto-resume without Sync to… in the UI.

Conflict dialog: **Keep Disk** = this repo wins; **Keep Studio** = the place wins.

## Persist the world (macOS)

MCP and Script Sync change the **open Studio session**, not `place.rbxlx`, until Studio saves. Official MCP cannot save to disk. `robld save` focuses Roblox Studio and sends **File → Save** (falls back to Cmd+S). No restart, no rewriting XML.

Grant **Accessibility** to the app that runs `robld` (Terminal / iTerm / Grok): System Settings → Privacy & Security → Accessibility. If `place.rbxlx` mtime does not change, exit 2 — click the Studio window and retry.

Use this after MCP world edits (parts, meshes, lighting). Luau still goes through Script Sync files; you do not need save for script-only commits.

## Loop

1. `robld` until `READY:`.
2. Edit Luau on disk.
3. Give Script Sync a moment to apply.
4. Playtest via MCP (Edit datamodel for lasting instance changes, then start play).
5. Read console / screenshot / inspect.
6. Stop play. Fix files on disk, not play-mode script source.
7. If the DataModel changed (not just Luau), `robld save` then confirm `place.rbxlx` changed.
8. Commit scripts + `place.rbxlx`.

Do not `multi_edit` a script in the same turn you wrote that file.

Official docs: [Script Sync](https://create.roblox.com/docs/scripting/sync) · [Studio MCP](https://create.roblox.com/docs/studio/mcp) · [Studio CLI](https://create.roblox.com/docs/studio/command-line-interface).
