Install robld (Robuild — Roblox Studio interface for agents) on this machine, then set up this project folder.

Roblox Studio must already be installed. Studio is not available on Linux, so robld cannot work on Linux. If this machine is Linux, or Studio is missing, stop and tell the user.

1. Confirm the OS is macOS or Windows. If Linux, stop.
2. Detect CPU arch.
3. Download the matching binary from the latest GitHub release of https://github.com/aduermael/robld
   Asset names:
   - robld-darwin-arm64
   - robld-darwin-amd64
   - robld-windows-amd64.exe
   Use: https://github.com/aduermael/robld/releases/latest/download/<asset>
4. Install it on PATH in the usual place for this OS:
   - macOS: ~/.local/bin/robld (create the dir if needed; chmod +x)
   - Windows: %LOCALAPPDATA%\Programs\robld\robld.exe and ensure that folder is on PATH
5. From this project folder, run: robld --install
6. Confirm `robld` works (`robld --help` and `robld --version`) and that skill files exist under .claude/skills/robld, .grok/skills/robld, .agents/skills/robld, .cursor/skills/robld, and .codex/skills/robld.

Do not ask me to run these steps by hand unless a permission dialog needs a human. Prefer the release binary over building from source. Later, if robld prints that a new version is available, run `robld --update` (that replaces the binary and reinstalls the skill).
