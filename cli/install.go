package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed skill/SKILL.md
var embeddedSkill string

// Skill destinations for the agents people actually use with a project folder.
var skillInstallPaths = []string{
	".claude/skills/robld/SKILL.md",
	".grok/skills/robld/SKILL.md",
	".agents/skills/robld/SKILL.md",
	".cursor/skills/robld/SKILL.md",
	".codex/skills/robld/SKILL.md",
}

func runInstall() {
	body := strings.TrimSpace(embeddedSkill) + "\n"
	if body == "\n" || embeddedSkill == "" {
		fail(exitError, "ERROR: embedded skill is missing from this robld build")
	}
	root, err := os.Getwd()
	if err != nil {
		fail(exitError, "ERROR: %v", err)
	}
	var wrote []string
	for _, rel := range skillInstallPaths {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail(exitError, "ERROR: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			fail(exitError, "ERROR: could not write %s: %v", rel, err)
		}
		wrote = append(wrote, rel)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agents); os.IsNotExist(err) {
		note := "# Roblox agent workspace (robld)\n\n" +
			"Run `robld` from this folder until stdout contains `READY:`.\n" +
			"Skill: `.claude/skills/robld/SKILL.md` (also installed for Grok, Codex, Cursor, and `.agents`).\n" +
			"Full how-to: https://aduermael.github.io/robld/\n"
		if err := os.WriteFile(agents, []byte(note), 0o644); err != nil {
			fail(exitError, "ERROR: could not write AGENTS.md: %v", err)
		}
		wrote = append(wrote, "AGENTS.md")
	}
	info("READY: installed robld skill into this folder")
	for _, rel := range wrote {
		fmt.Printf("  %s\n", rel)
	}
	info("Next: run robld --new \"My Game\" (or robld with a place URL), then robld until READY.")
}
