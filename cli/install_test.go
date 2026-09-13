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
		"When the user must click",
		"StarterPlayerScripts",
		".local.luau",
		"Save to File",
		"Start Test Session",
		"F5",
		"FLog::CreatorOutput",
		"*_last.log",
		"list_roblox_studios",
		"start_stop_play",
		"Unable to reach Roblox Studio",
		"What game would you like to work on now?",
		"Never ask the user to run",
		"wait for an agent restart",
		"place.rbxlx.lock",
		"robuild-dump/",
		"Homebrew `luau`",
		"newline-delimited JSON-RPC",
		"Content-Length",
		"Screen Recording",
		"open -a RobloxStudio",
		"Manage MCP Servers",
		"force-quits",
		"key code 96",
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
	if !strings.Contains(string(agents), "Never ask the user to run robld") {
		t.Fatalf("AGENTS.md missing never-ask-user rule: %s", agents)
	}
}

func TestInstallCompleteText(t *testing.T) {
	got := installCompleteText([]string{".grok/skills/robld/SKILL.md"})
	for _, need := range []string{
		"READY: robld is installed",
		".grok/skills/robld/SKILL.md",
		"this session",
		"What game would you like to work on now?",
		"Never ask the user to run robld commands or flags",
		"When the user must click",
		"Save to File",
	} {
		if !strings.Contains(got, need) {
			t.Errorf("install complete missing %q\n%s", need, got)
		}
	}
}

func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	for _, p := range []string{name, filepath.Join("..", name)} {
		b, err := os.ReadFile(p)
		if err == nil {
			return string(b)
		}
	}
	t.Fatalf("could not read %s", name)
	return ""
}

func TestInstallPromptLoadsSkillInSession(t *testing.T) {
	s := readRepoFile(t, "INSTALL-PROMPT.md")
	for _, need := range []string{
		"What game would you like to work on now?",
		"Do not wait for an agent restart",
		"Never ask the user to run robld commands or flags",
	} {
		if !strings.Contains(s, need) {
			t.Errorf("INSTALL-PROMPT.md missing %q", need)
		}
	}
	llms := readRepoFile(t, "docs/llms.txt")
	if !strings.Contains(llms, "What game would you like to work on now?") {
		t.Error("docs/llms.txt missing post-install game prompt")
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
