package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pluginFilesJSON(src string) string {
	const marker = "local FILES = HttpService:JSONDecode([==["
	i := strings.Index(src, marker)
	if i < 0 {
		return ""
	}
	rest := src[i+len(marker):]
	j := strings.Index(rest, "]==]")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func TestShouldSeedPlugin(t *testing.T) {
	if shouldSeedPlugin(false, filepath.Join(t.TempDir(), "missing.lua")) {
		t.Fatal("non --new must not seed")
	}
	missing := filepath.Join(t.TempDir(), "robuild_agent.lua")
	if !shouldSeedPlugin(true, missing) {
		t.Fatal("--new with no plugin should seed")
	}
	exists := filepath.Join(t.TempDir(), "robuild_agent.lua")
	if err := os.WriteFile(exists, []byte("-- old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if shouldSeedPlugin(true, exists) {
		t.Fatal("must not re-seed after the plugin exists")
	}
}

func TestPluginOmitsGameSourceAfterSeed(t *testing.T) {
	root = t.TempDir()
	m := greenfieldManifest()
	saveSyncManifest(m)
	seedGreenfield(m)
	combat := filepath.Join(root, "src", "shared", "Combat.luau")
	if err := os.WriteFile(combat, []byte("return {slashWavesCombatUnique=true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	seeded, err := renderPluginSource(true)
	if err != nil {
		t.Fatal(err)
	}
	seedFiles := pluginFilesJSON(seeded)
	if !strings.Contains(seedFiles, "robld server") {
		t.Fatal("first --new seed should embed the greenfield server stub")
	}
	if !strings.Contains(seedFiles, "LocalScript") {
		t.Fatal("first seed should include the client LocalScript")
	}
	if strings.Contains(seeded, "slashWavesCombatUnique") {
		t.Fatal("seed must not embed later game script bodies")
	}

	later, err := renderPluginSource(false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(later, "slashWavesCombatUnique") {
		t.Fatal("after seed, plugin must not contain later game script bodies")
	}
	if strings.Contains(pluginFilesJSON(later), "robld server") {
		t.Fatal("after seed, plugin must not embed script bodies")
	}
	raw := pluginFilesJSON(later)
	var files []luaFile
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		t.Fatalf("FILES JSON %q: %v", raw, err)
	}
	if len(files) != 0 {
		t.Fatalf("applyFiles would recreate %d files from empty-disk run: %+v", len(files), files)
	}
	if !strings.Contains(later, "FILES == nil or #FILES == 0") {
		t.Fatal("applyFiles must no-op when FILES is empty so deleted scripts stay gone")
	}
}

func TestPluginEnablesExternalMCP(t *testing.T) {
	root = t.TempDir()
	src, err := renderPluginSource(false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, `plugin:SetSetting("Assistant-ExternalMCPEnabled", true)`) {
		t.Fatal("plugin must persist Assistant-ExternalMCPEnabled so Studio MCP stays on")
	}
}

func TestCollectSeedFilesLocalScript(t *testing.T) {
	root = t.TempDir()
	m := greenfieldManifest()
	saveSyncManifest(m)
	seedGreenfield(m)
	files := collectSeedFiles(m)
	var client *luaFile
	for i := range files {
		if files[i].Name == "main" && files[i].Class == "LocalScript" {
			client = &files[i]
		}
	}
	if client == nil {
		t.Fatalf("expected LocalScript seed, got %+v", files)
	}
	if client.Parent != "StarterPlayer.StarterPlayerScripts" {
		t.Fatalf("client parent %q", client.Parent)
	}
}
