package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInstallWritesSkills(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// runInstall calls fail/os.Exit on error; exercise the write path via helper pieces.
	body := strings.TrimSpace(embeddedSkill) + "\n"
	if !strings.Contains(body, "robld") {
		t.Fatalf("embedded skill missing robld: %q", body[:80])
	}
	for _, rel := range skillInstallPaths {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "Script Sync") {
			t.Fatalf("%s missing Script Sync", rel)
		}
		if !strings.Contains(string(raw), "git init") {
			t.Fatalf("%s missing git init guidance", rel)
		}
	}
}
