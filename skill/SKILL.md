---
name: robld
description: >
  Use whenever the user wants to create or work on a Roblox game. Gives the
  agent full control of Roblox Studio while keeping a local working directory
  as the source of truth for scripts (easy to version with git) and driving
  playtest/world through Studio MCP. Triggers: Roblox, Studio, Luau, game,
  Script Sync, playtest, MCP, DataModel, robld.
---

# Roblox: robld + Script Sync + Studio MCP

`robld` (say *robuild*) bootstraps Studio Script Sync and MCP so you can edit Luau on disk while Studio stays open for playtest and the world.

**Never ask the user to run `robld` commands or flags** unless they asked you to. You run them. User-facing talk is not a CLI cheat-sheet.

## When the user must click

These are the only human steps. Name the UI; do not invite them to run CLI.

1. **Studio login** if Studio shows a sign-in window.
2. **macOS Accessibility** for the app that runs robld (Terminal / Grok / Cursor / …): System Settings → Privacy & Security → Accessibility — needed for File → Save / menu keystrokes, not for screenshots.

**MCP is expected on.** `robld` assumes Studio’s MCP server is active for playtest/world work. Do **not** make “enable MCP” a routine ask. Only if `list_roblox_studios` is missing/empty or Studio is unreachable: tell them Assistant → … → Manage MCP Servers → enable **Studio as MCP server**. (`robld` will write that preference before launch once the GlobalSettings property name is confirmed from a dump.)

Do not ask the user to click **Save / Don't Save / Cancel** on restart. robld saves, sends Enter, waits, then force-quits if Studio is still up.

## CLI (you run this; do not paste it at the user)

Same command every time, from the game folder. Install from https://robld.com/ if `robld` is missing.

| Command | What it does |
|---|---|
| `robld` | Open the place in `place.json`, wait until Script Sync is up. Re-run until `READY:`. Restarts Studio only if prefs, plugin, or resume records need a write. |
| `robld --new` / `robld --new "Name"` | Create a local `place.rbxlx` and `place.json` (UniqueIds seeded), then launch Studio. |
| `robld <place-id-or-url>` | Bind an existing cloud place (or `robld --file place.rbxlx`). |
| `robld --install` | Write this skill into the current project (Claude, Grok, Codex, Cursor, `.agents`). |
| `robld --version` | Print the version. |
| `robld --update` | Update robld and reinstall the skill. |
| `robld save` | macOS: File → **Save to File** (Cmd+S only if mtime still does not change). No restart. |
| `robld prefs` / `robld prefs apply` | Show or write this machine's Script Sync Studio Settings. |
| `robld dump` | Copy Studio settings/logs/sync clues into `robuild-dump/` (unsliced plugin dump, not the truncated log line). |
| `robld scan` | Stdout-only version of the dump clues. |
| `robld --help` | Usage text. |

If a command prints that a new version is available, you run `robld --update`.

## Bootstrap

You run `robld` from the game folder. Pass ids as args; do not prompt. Do not start Script Sync from MCP or Luau. Do not use `open -a RobloxStudio` (it can spawn a second Studio or close the place).

| Exit | What you do |
|---|---|
| 0 and `READY: Script Sync + MCP` | Script Sync and Studio MCP tools/list are live. This is the normal path — continue. |
| 0 and `READY: Script Sync` | Script Sync is up but MCP probe failed. Unusual. Surface `NEED_USER:` (enable Studio as MCP server) once, then prefer fixing MCP over living in the playtest fallback. |
| 1 `NEED_PLACE:` | Create a local place or bind a place id / game URL. Re-run. |
| 1 `ERROR:` | Show the message (usually Studio missing). Do not invent a connection. |
| 2 `NOT_READY:` | Show the printed Sync to… steps. After the user does them, re-run. |

Studio is required (macOS or Windows). If Studio was not found, stop. If Studio shows a login window, the user must sign in, then you continue.

`READY:` does **not** claim MCP unless a real Studio MCP `tools/list` is non-empty and Studio is reachable. Writing `.mcp.json` does not hot-load tools into this agent process.

Script Sync **preferences** are per-machine; `robld` writes them and may restart Studio **only when prefs, the plugin, or resume records actually need a write**. Before that quit it sends File → **Save to File**. **`robuild-sync.json`** is the per-project folder map — honor it; games are not all `ServerScriptService/` on disk. Do not overwrite a custom map. The generated default maps `src/server|shared|client` to **services** (that is what Studio auto-resumes). New Luau goes next to existing siblings. If `robld` exits `NOT_READY`, show the printed Sync to… paths. `robld dump` → read `robuild-dump/REPORT.txt` and `rbx-storage.txt`.

Hand-wiring StudioMCP: newline-delimited JSON-RPC, not LSP `Content-Length`.

## Git

If `git` is installed and this folder is not already a git repository, run `git init` and make an initial commit. Commit often (Luau, the place file, and other project files). Do not force-push or rewrite history unless asked.

Gitignore place lockfiles (e.g. `*.rbxlx.lock`) and `robuild-dump/` (dump copies logs and `rbx-storage.db`).

Roblox-API-free modules can be unit-tested with Homebrew `luau` (`require("../src/shared/…")`).

## What lives where

- **Luau:** files in this tree (Script Sync). Create/edit/delete on disk only.
- **World / instances:** the local place file pointed at by `place.json` (often `place.rbxlx`, but the path is whatever is configured). After MCP creates parts/meshes, File → **Save to File** (macOS). Do not rewrite the place XML by hand or restart Studio to persist.
- **Ids / local path / name:** `place.json` (source of truth for which place file to open).
- **Folder map:** `robuild-sync.json`. Edit this file for a different layout; `robld` will not overwrite an existing map.
- **MCP:** playtest, console, screenshots, inspect, meshes, marketplace inserts — **only if the tools exist**.

| In git | Owned by |
|---|---|
| `*.luau` under synced folders | Script Sync (Studio ↔ disk) |
| Local place file (from `place.json`) | Studio **Save to File** — maps, instances, lighting |
| `place.json` | `robld` |
| `robuild-sync.json` | `robld` |

On-disk artifact names still use the `robuild-*` prefix. The command you type is `robld`.

`robld` always opens the **local** place file when `place.json` has `localPlaceFile`. Cloud place/universe ids are optional (after **File → Publish to Roblox**).

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

Under **StarterPlayerScripts** (the default `src/client` root), use `*.local.luau`. `*.client.luau` (RunContext Client) in that container runs multiple times. Use `.client.luau` only if the parent is **not** a starter-player script container.

- Put new scripts next to existing siblings so they land under the already-synced Studio folder.
- MCP `script_read` / `script_search` / `script_grep` are fine for **reading** live source.
- Do not use MCP `multi_edit` (or `execute_luau` that assigns `Source`) on a script that lives on disk.
- Do not create scripts via MCP under parts, GUIs, or other non-Folder instances if they should be committed — Script Sync will not pick them up.

After adding a ModuleScript, wait until plugin dump `instances[]` lists it (`robld dump` / unsliced `robuildDump`, not the truncated `ROBUILD_JSON` log line). Then playtest.

## MCP — expected on

Call `list_roblox_studios` once per session if you do not have `studio_id`.

`robld` expects Studio MCP to be active. After a true `READY: Script Sync + MCP`, use the tools. If `list_roblox_studios` is **missing** or returns **“Unable to reach Roblox Studio”**, **stop** using `start_stop_play`, `get_console_output`, and `screen_capture` — surface the enable-MCP `NEED_USER` once, then use the playtest fallback only as a bridge. Do not assume tools exist just because `.mcp.json` was written.

### Use MCP for

- **Playtest:** `start_stop_play`, `get_console_output`, `get_studio_state`
- **Screenshots:** MCP `screen_capture` (in-agent; do not ask the user to take a screenshot)
- **Inspect:** `search_game_tree`, `inspect_instance`
- **Queries / setup:** `execute_luau` with `datamodel_type` Edit (or Client/Server only while playing)
- **World:** instances, `insert_asset`, `search_asset`, `generate_mesh`, `generate_material`, `generate_procedural_model`
- **Coarse character placement:** `character_navigation` (go to a position or instance) when you need the avatar somewhere to assert state
- **Discrete UI / keys:** `user_mouse_input` / `user_keyboard_input` for one-shot UI (click a button, type in a field) — not continuous locomotion
- **Long scenarios:** MCP `subagent` type `playtest` when available

### Player movement — do not fake WASD via MCP

Studio MCP is **turn-based**. `user_keyboard_input` / `user_mouse_input` send discrete actions between agent turns. That is a **poor** way to simulate smooth, human-like walking, camera look, or strafing (tested: feels wrong, burns turns, unreliable for “does this feel good?”).

- Do **not** drive continuous locomotion with long WASD / mouse-move MCP sequences.
- Prefer **`character_navigation`** or **`execute_luau`** (`HumanoidRootPart.CFrame`, `Humanoid:MoveTo`, teleport, checkpoint spawn) when you only need the character placed to test systems.
- Prefer **console + `screen_capture` + inspect** to verify outcomes without piloting.
- For **continuous / reactive control** (chase AI, auto-run to a point, hold a key while something happens, environment-driven locomotion): **write Luau on disk** that handles input or control live in play (`UserInputService`, `ContextActionService`, `Humanoid:Move` / `MoveTo`, RunService steppers, etc.). That runs every frame in Studio — MCP only starts play, watches console/screenshots, and asserts. Put those scripts under the synced tree like any other game code.
- If a human’s control *feel* still matters, ask the **user** to play that pass; keep MCP for setup, assertions, and world edits.

Official tool overview: [Studio MCP](https://create.roblox.com/docs/studio/mcp).

## Playtest fallback (no MCP tools)

There is no Test menu item named **Play**.

- Start: Test menu **Start Test Session**, or **F5** (key code 96) for Play Solo.
- Stop: Test menu **Stop** / End Session.
- Console: `FLog::CreatorOutput` in the **latest** `~/Library/Logs/Roblox/*Studio*_last.log`. A new Studio launch creates a new log; grepping an old `*_last.log` looks like play never started. `robld dump` copies logs; the live file is under `~/Library/Logs/Roblox/`.
- Screenshots: MCP `screen_capture` only (do not ask the user to capture the screen).
- Wrap Apple Events / `osascript` in a timeout (do not hang on `activate`). Do not use `open -a RobloxStudio`.

## Script Sync on this machine

Studio Settings → Script Sync are **not** in the place file. They live in Studio’s `GlobalSettings_13.xml` (macOS `~/Library/Roblox/`, Windows `%LOCALAPPDATA%\Roblox\`).

`robld` applies agent defaults when it **launches** Studio (not when Studio is already open, unless a restart is required). Before a required quit it sends File → **Save to File**, Enter for a Save dialog, then force-quits if still stuck. Defaults:

| Setting | Value |
|---|---|
| Auto resume sync on place open | on |
| Resume conflicted sync on place open | Always keep local |
| Keep local files/directories after Stop Sync | Keep local files |
| File extension | `.luau` |
| Enable Studio as MCP server | on (planned GlobalSettings write; property name TBD from dump) |

The main `robld` command also:

- Writes **`robuild-sync.json`**. For an empty repo it seeds `src/server`, `src/shared`, `src/client` mapped to **Studio services** (`ServerScriptService`, `ReplicatedStorage`, `StarterPlayer.StarterPlayerScripts`). Studio auto-resume binds those service UniqueIds; nested Folder records in the plist do not start sync.
- Installs a Studio plugin that seeds missing scripts **only on first `--new`** and prints `ROBUILD_JSON` / `robuildDump` for wait. It does **not** embed full game source afterward, so deleting a synced script on disk cannot be resurrected by a later run.
- When Studio is **closed**, writes Script Sync **resume records** into Studio preferences (service UniqueId ↔ disk path). `--new` seeds those UniqueIds in `place.rbxlx` so the first macOS launch can resume.

Conflict dialog: **Keep Disk** = this repo wins; **Keep Studio** = the place wins.

## Persist the world (macOS)

MCP and Script Sync change the **open Studio session**, not the on-disk place file, until Studio saves. Official MCP cannot save to disk. `robld save` focuses Roblox Studio and sends **File → Save to File**. Cmd+S is not enough on unpublished local places; it is the fallback only if mtime still does not change. No restart, no rewriting XML.

Grant **macOS Accessibility** to the app that runs robld (Terminal / iTerm / Grok / Cursor): System Settings → Privacy & Security → Accessibility. If the place file mtime does not change, exit 2 — click the Studio window and retry.

Use this after MCP world edits (parts, meshes, lighting). Luau still goes through Script Sync files; you do not need save for script-only commits.

## Loop

1. `robld` until `READY:`.
2. Edit Luau on disk.
3. After adding a ModuleScript, wait until dump `instances[]` lists it.
4. Playtest via MCP (expected). Prefer navigation/teleport, or Luau that drives control live — not WASD spam via MCP. If MCP is down, F5 / Start Test Session and the latest `*_last.log` as a bridge.
5. Read console / screenshot / inspect.
6. Stop play. Fix files on disk, not play-mode script source.
7. If the DataModel changed (not just Luau), `robld save` then confirm the place file changed.
8. Commit scripts + the place file.

Do not `multi_edit` a script in the same turn you wrote that file.

Official docs: [Script Sync](https://create.roblox.com/docs/scripting/sync) · [Studio MCP](https://create.roblox.com/docs/studio/mcp) · [Studio CLI](https://create.roblox.com/docs/studio/command-line-interface).
