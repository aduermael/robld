package main

import (
	"fmt"
	"os"
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

func skillBody() string {
	return strings.TrimSpace(skill.Markdown) + "\n"
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
			"Run `robld` from this folder until stdout contains `READY:`.\n" +
			"If the skill is missing, run `robld --install`. If a new version is available, run `robld --update`.\n" +
			"Skill: `.claude/skills/robld/SKILL.md` (also installed for Grok, Codex, Cursor, and `.agents`).\n" +
			"Full how-to: https://aduermael.github.io/robld/\n"
		if err := os.WriteFile(agents, []byte(note), 0o644); err != nil {
			return wrote, fmt.Errorf("could not write AGENTS.md: %w", err)
		}
		wrote = append(wrote, "AGENTS.md")
	}
	return wrote, nil
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
	info("READY: installed robld skill into this folder")
	for _, rel := range wrote {
		fmt.Printf("  %s\n", rel)
	}
	info("Next: run robld --new \"My Game\" (or robld with a place URL), then robld until READY.")
}
