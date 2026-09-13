---
name: roblox-script-sync-mcp
description: >
  Develop this Roblox game with robld, Studio Script Sync (Luau on disk),
  a git-tracked place.rbxlx, and Studio MCP (playtest, world, assets). Use
  when writing Luau, playtesting, inspecting the DataModel, inserting models,
  generating meshes, or working in Roblox Studio. Triggers: Roblox, Studio,
  Luau, Script Sync, playtest, MCP, DataModel, robld.
---

# Roblox: robld + Script Sync + Studio MCP

## Bootstrap

`robld` should already be on PATH (agent install from https://aduermael.github.io/robld/). Run it from the game folder. Same command every time.

If the skill is missing in this project, run `robld --install`.

| Exit | What you do |
|---|---|
| 0 and `READY:` | Treat Script Sync + MCP as on. Continue the task. |
| 1 `NEED_PLACE:` | `robld --new` / `robld --new "Name"`, or a place id / game URL. Re-run. |
| 1 `ERROR:` | Show the message (usually Studio missing). Do not invent a connection. |
| 2 `NOT_READY:` | Show the printed Sync to… steps. After the user does them, re-run `robld`. |

Do not try to start Script Sync from MCP or Luau. Pass ids as args; do not prompt.

Script Sync **preferences** are per-machine; main `robld` writes them and may restart Studio. **`robuild-sync.json`** is the per-project folder map — honor it; games are not all `ServerScriptService/` on disk. Do not overwrite a custom map. The generated default maps `src/server|shared|client` to **services** (that is what Studio auto-resumes). New Luau goes next to existing siblings. Do not start Sync to… from MCP. If `robld` exits `NOT_READY`, show the printed Sync to… paths. `robld dump` → read `robuild-dump/REPORT.txt` and `rbx-storage.txt`.

## What lives where

- **Luau:** files in this tree (Script Sync). Create/edit/delete on disk only.
- **World / instances:** `place.rbxlx`. After MCP creates parts/meshes, run `robld save` (macOS: File→Save / Cmd+S into Studio). Do not rewrite the XML or restart Studio to persist.
- **MCP:** playtest, console, screenshots, inspect, meshes, marketplace inserts.

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

## Loop

1. Edit Luau on disk.
2. Give Script Sync a moment to apply.
3. Playtest via MCP (Edit datamodel for lasting instance changes, then start play).
4. Read console / screenshot / inspect.
5. Stop play. Fix files on disk, not play-mode script source.
6. If the DataModel changed (not just Luau), `robld save` then confirm `place.rbxlx` changed.

Do not `multi_edit` a script in the same turn you wrote that file.
