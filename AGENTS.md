# Roblox agent workspace (robld)

If the skill is missing, run `robld --install`. Run `robld` from the game folder until stdout contains `READY:`. Build with `cd cli && go build -o robld .` (Windows: `cd cli; go build -o robld.exe .`). Non-interactive, re-runnable, state in `place.json`. `robld --version` prints the stamped build; if output says a new version is available, run `robld --update`.

- Exit **0** `READY:` — Script Sync + MCP are up. Proceed.
- Exit **1** `NEED_PLACE:` — `robld --new` / `robld --new "Name"`, or a place id / game URL. Re-run.
- Exit **1** `ERROR:` — show the message (usually Studio missing).
- Exit **2** `NOT_READY:` — user Sync to… in Explorer, then re-run `robld`. Do not start Script Sync yourself.
- After MCP **world** edits (parts, meshes), run `robld save` (macOS) so `place.rbxlx` updates. Do not restart Studio to persist. If Accessibility blocks keystrokes, tell the user to enable it.
- Script Sync Studio Settings live in `GlobalSettings_13.xml`. Main `robld` writes them and may **restart Studio** on macOS so they apply. `robld dump` writes `robuild-dump/` for inspection (includes `rbx-storage.txt` when that SQLite file exists).
- **`robuild-sync.json` is the per-project map** (Studio instance path ↔ disk folder). Different games use different trees; edit this file, do not assume `ServerScriptService/` on disk. `robld` does not overwrite an existing map. Put new Luau next to siblings under an already-mapped folder.
- Script Sync resume bindings live in `com.roblox.RobloxStudio.plist` (`File_Sync_Persistence_Record_V1`). `robld` writes them when Studio is closed, from UniqueIds in `place.rbxlx` (`--new` seeds them) plus `robuild-sync.json`. Bind **services** (not nested Folders) — that is what Studio actually auto-resumes. If `NOT_READY`, user Sync to… once per root.

Once READY:

- **Luau:** create/edit/delete files in this tree only. Do not MCP-write scripts that live on disk.
- **World:** `place.rbxlx` (and Studio Save). MCP for playtest, inspect, instances, assets, meshes.
- Full split and file naming: `.grok/skills/robld/SKILL.md` (source of truth in this repo: `skill/SKILL.md`)
