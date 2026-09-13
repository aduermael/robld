Install robld (Robuild — Roblox Studio interface for agents) on this machine, then set up this project folder.

Roblox Studio must already be installed. Studio is not available on Linux, so robld cannot work on Linux. If this machine is Linux, or Studio is missing, stop and tell the user.

1. Confirm the OS is macOS or Windows. If Linux, stop.
2. Detect CPU arch.
3. Download the matching binary:
   - macOS arm64 (Apple Silicon): https://robld.com/releases/latest/robld-darwin-arm64
   - macOS amd64 (Intel): https://robld.com/releases/latest/robld-darwin-amd64
   - Windows amd64: https://robld.com/releases/latest/robld-windows-amd64.exe
4. Verify the file's sha256 against https://robld.com/releases/latest.json (the `sha256` field for that asset). Do not install if it does not match.
5. Install it on PATH in the usual place for this OS:
   - macOS: ~/.local/bin/robld (create the dir if needed; chmod +x)
   - Windows: %LOCALAPPDATA%\Programs\robld\robld.exe and ensure that folder is on PATH
6. From this project folder, run: robld --install
7. Confirm `robld` works (`robld --help` and `robld --version`) and that skill files exist under .claude/skills/robld, .grok/skills/robld, .agents/skills/robld, .cursor/skills/robld, and .codex/skills/robld.

Do not ask me to run these steps by hand unless a permission dialog needs a human. Later, if robld prints that a new version is available, run `robld --update`.
