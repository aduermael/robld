package main

import (
	"fmt"
	"os"
	"runtime"
	"path/filepath"
	"strings"

	"robld/skill"
)

// Skill destinations for the agents people actually use with a project folder.
var skillInstallPaths = []string{
	".claude/skills/robld/SKILL.md",
	".grok/skills/robld/SKILL.md",
	".agents/skills/robld/SKILL.md",
	".cursor/skills/robld/SKILL.md",
	".codex/skills/robld/SKILL.md",
}

// Canonical skill text is written for macOS. On Windows install, swap the
// host UI-permission bullet so agents see the right OS wording.
const skillHostUIPermissionDarwin = "**macOS Accessibility** for the app that runs robld (Terminal / Grok / Cursor / …): System Settings → Privacy & Security → Accessibility — needed for File → Save / menu keystrokes, not for screenshots."

const skillHostUIPermissionWindows = "**Windows UI automation** for the app that runs robld (allow it to control other apps when Windows prompts) — needed for Studio menu/save automation when available, not for screenshots."

const skillHostUIPermissionOther = "**Host OS UI-control permission** for the app that runs robld (so it can send Studio menu/save keystrokes) — not for screenshots."

func hostUIPermissionLine() string {
	switch runtime.GOOS {
	case "windows":
		return skillHostUIPermissionWindows
	case "darwin":
		return skillHostUIPermissionDarwin
	default:
		return skillHostUIPermissionOther
	}
}

func skillBody() string {
	body := strings.TrimSpace(skill.Markdown) + "\n"
	line := hostUIPermissionLine()
	if line != skillHostUIPermissionDarwin {
		body = strings.ReplaceAll(body, skillHostUIPermissionDarwin, line)
	}
	return body
}

func installSkill(dest string) ([]string, error) {
	body := skillBody()
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("embedded skill is missing from this robld build")
	}
	var wrote []string
	for _, rel := range skillInstallPaths {
		path := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return wrote, err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return wrote, fmt.Errorf("could not write %s: %w", rel, err)
		}
		wrote = append(wrote, rel)
	}
	agents := filepath.Join(dest, "AGENTS.md")
	if _, err := os.Stat(agents); os.IsNotExist(err) {
		note := "# Roblox agent workspace (robld)\n\n" +
			"After install, load `.grok/skills/robld/SKILL.md` (or `.claude/skills/robld/SKILL.md`) into this session now.\n" +
			"Never ask the user to run robld commands or flags unless they asked; you run them.\n" +
			"Run `robld` from this folder until stdout contains `READY:`.\n" +
			"If the skill is missing, run `robld --install`. If a new version is available, run `robld --update`.\n" +
			"Skill: `.claude/skills/robld/SKILL.md` (also installed for Grok, Codex, Cursor, and `.agents`).\n" +
			"Full how-to: https://robld.com/\n"
		if err := os.WriteFile(agents, []byte(note), 0o644); err != nil {
			return wrote, fmt.Errorf("could not write AGENTS.md: %w", err)
		}
		wrote = append(wrote, "AGENTS.md")
	}
	return wrote, nil
}

func installCompleteText(wrote []string) string {
	var b strings.Builder
	b.WriteString("READY: robld is installed\n")
	for _, rel := range wrote {
		b.WriteString("  " + rel + "\n")
	}
	b.WriteString("\n")
	b.WriteString("AGENT: Read the skill you just wrote into this session now — do not wait for an agent restart:\n")
	b.WriteString("  .grok/skills/robld/SKILL.md\n")
	b.WriteString("AGENT: Tell the user robld is installed, then ask: \"What game would you like to work on now?\"\n")
	b.WriteString("AGENT: Never ask the user to run robld commands or flags unless they asked; you run them.\n")
	b.WriteString("\n----- skill/SKILL.md -----\n")
	b.WriteString(skillBody())
	return b.String()
}

func runInstall() {
	dest, err := os.Getwd()
	if err != nil {
		fail(exitError, "ERROR: %v", err)
	}
	wrote, err := installSkill(dest)
	if err != nil {
		fail(exitError, "ERROR: %v", err)
	}
	fmt.Print(installCompleteText(wrote))
}
