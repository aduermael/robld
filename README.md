# Robuild

**Agent-first Roblox development.** Luau lives in git. The place file lives in git. Studio stays open for playtest and the world — driven by MCP.

The CLI is `robld` (say *robuild*). One binary on macOS and Windows. Point it at a new game or an existing place, re-run until it prints `READY`, then let your agent edit scripts on disk while Studio Script Sync and MCP handle the rest.

## Why it exists

Roblox Studio is great for seeing the world. It is awkward as the source of truth for an AI coding agent. Robuild flips that:

- **Scripts** are normal `.luau` files in the repo (Script Sync keeps Studio honest).
- **World** state (instances, lighting, maps) is `place.rbxlx`, saved into git.
- **Playtest, inspect, parts, assets** go through Studio’s MCP server.

Your agent does not fight the Studio UI to start sync. `robld` bootstraps prefs, the sync map, a small Studio plugin, and MCP config — then waits until the place is actually ready.

## Quick start

```bash
cd cli && go build -o robld .
cd ..
./cli/robld --new "My Game"
# re-run until READY
./cli/robld
```

Windows:

```powershell
cd cli
go build -o robld.exe .
cd ..
.\cli\robld.exe --new "My Game"
.\cli\robld.exe
```

| Exit | Meaning |
|---|---|
| **0** `READY:` | Studio is on the place, Luau is on disk, MCP config written. |
| **1** `NEED_PLACE:` / `ERROR:` | Pass `--new`, a game URL, or fix Studio. |
| **2** `NOT_READY:` | Do **Sync to…** once in Explorer, then re-run `robld`. |

How-to detail: **[START.md](START.md)**. Agent rules: **[AGENTS.md](AGENTS.md)**. Studio internals we reverse-engineered: **[NOTES.md](NOTES.md)**.

## What you version

| In git | Owned by |
|---|---|
| `*.luau` under synced folders | Script Sync (Studio ↔ disk) |
| `place.rbxlx` | Studio **Save** — maps, instances, lighting |
| `place.json` | `robld` (ids, local path, name) |
| `robuild-sync.json` | `robld` (Studio folder ↔ disk folder map) |

On-disk artifact names still use the `robuild-*` prefix so existing projects keep working. The command you type is `robld`.

## Everyday loop

1. `robld` until `READY`.
2. Agent edits Luau in the tree (not via MCP script writes for synced files).
3. MCP for playtest, screenshots, parts, meshes.
4. After world edits on macOS: `robld save` (Accessibility keystrokes → File → Save).
5. Commit scripts + `place.rbxlx`.

Built for agents that already live in your editor — Grok, Claude, Codex, Cursor, and friends. Roblox stays Roblox. The repo becomes the product.
