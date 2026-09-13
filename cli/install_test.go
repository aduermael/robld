package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillWritesEmbeddedSkill(t *testing.T) {
	body := skillBody()
	if strings.TrimSpace(body) == "" {
		t.Fatal("embedded skill is empty")
	}
	for _, need := range []string{
		"robld",
		"Script Sync",
		"git init",
		"--install",
		"--version",
		"--update",
		"READY:",
		"NEED_PLACE:",
		"NOT_READY:",
		"UniqueId",
	} {
		if !strings.Contains(body, need) {
			t.Errorf("embedded skill missing %q", need)
		}
	}

	dir := t.TempDir()
	wrote, err := installSkill(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(wrote) == 0 {
		t.Fatal("installSkill wrote nothing")
	}
	for _, rel := range skillInstallPaths {
		path := filepath.Join(dir, rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(raw) != body {
			t.Fatalf("%s does not match embedded skill (%d bytes vs %d)", rel, len(raw), len(body))
		}
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "robld --install") {
		t.Fatalf("AGENTS.md missing install hint: %s", agents)
	}
}

func TestInstallSkillLeavesExistingAgents(t *testing.T) {
	dir := t.TempDir()
	want := "keep me\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installSkill(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("AGENTS.md overwritten: %q", got)
	}
}
