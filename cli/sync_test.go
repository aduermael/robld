package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferInstancePath(t *testing.T) {
	cases := map[string]string{
		"src/server":               "ServerScriptService",
		"src/shared":               "ReplicatedStorage",
		"src/client":               "StarterPlayer.StarterPlayerScripts",
		"ServerScriptService":      "ServerScriptService",
		"StarterPlayerScripts":     "StarterPlayer.StarterPlayerScripts",
		"lib/util":                 "ServerScriptService.lib.util",
		"ReplicatedStorage/Shared": "ReplicatedStorage.Shared",
	}
	for in, want := range cases {
		if got := inferInstancePath(in); got != want {
			t.Errorf("inferInstancePath(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDiskRootForScript(t *testing.T) {
	cases := map[string]string{
		"src/server/main.server.luau": "src/server",
		"src/shared/Hello.luau":       "src/shared",
		"ServerScriptService/a.luau":  "ServerScriptService",
		"foo/bar/baz.luau":            "foo",
	}
	for in, want := range cases {
		if got := diskRootForScript(in); got != want {
			t.Errorf("diskRootForScript(%q)=%q want %q", in, got, want)
		}
	}
}

func TestResolveSyncManifestKeepsCustom(t *testing.T) {
	root = t.TempDir()
	custom := syncManifest{
		Version: 1,
		Roots:   []syncRoot{{Instance: "ReplicatedStorage.Foo", Disk: "foo"}},
	}
	saveSyncManifest(custom)
	got := resolveSyncManifest()
	if len(got.Roots) != 1 || got.Roots[0].Disk != "foo" || got.Roots[0].Instance != "ReplicatedStorage.Foo" {
		t.Fatalf("custom manifest overwritten: %+v", got)
	}
}

func TestDiscoverSyncRootsFromDisk(t *testing.T) {
	root = t.TempDir()
	path := filepath.Join(root, "src", "server", "main.server.luau")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("print(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := discoverSyncRootsFromDisk()
	if len(got) != 1 || got[0].Disk != "src/server" || got[0].Instance != "ServerScriptService" {
		t.Fatalf("got %+v", got)
	}
}

func TestMigrateLegacyGreenfieldMap(t *testing.T) {
	root = t.TempDir()
	old := syncManifest{
		Version:   1,
		Generated: true,
		Roots: []syncRoot{
			{Instance: "ServerScriptService.src", Disk: "src/server"},
			{Instance: "ReplicatedStorage.shared", Disk: "src/shared"},
			{Instance: "StarterPlayer.StarterPlayerScripts.src", Disk: "src/client"},
		},
	}
	saveSyncManifest(old)
	got := resolveSyncManifest()
	if len(got.Roots) != 3 || got.Roots[0].Instance != "ServerScriptService" || got.Roots[1].Instance != "ReplicatedStorage" {
		t.Fatalf("did not migrate: %+v", got)
	}
	if got.Roots[2].Instance != "StarterPlayer.StarterPlayerScripts" {
		t.Fatalf("client root %+v", got.Roots[2])
	}
}

func TestSeedGreenfieldUsesLocalScript(t *testing.T) {
	root = t.TempDir()
	seedGreenfield(greenfieldManifest())
	local := filepath.Join(root, "src", "client", "main.local.luau")
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("expected LocalScript seed: %v", err)
	}
	client := filepath.Join(root, "src", "client", "main.client.luau")
	if _, err := os.Stat(client); err == nil {
		t.Fatal("must not seed .client.luau under StarterPlayerScripts")
	}
}

func TestScriptInstanceName(t *testing.T) {
	n, c, r := scriptInstanceName("main.server.luau")
	if n != "main" || c != "Script" || r != "Server" {
		t.Fatalf("%s %s %s", n, c, r)
	}
	n, c, r = scriptInstanceName("Hello.luau")
	if n != "Hello" || c != "ModuleScript" || r != "" {
		t.Fatalf("%s %s %s", n, c, r)
	}
	n, c, r = scriptInstanceName("main.local.luau")
	if n != "main" || c != "LocalScript" || r != "" {
		t.Fatalf("%s %s %s", n, c, r)
	}
}

func TestSyncJSONReady(t *testing.T) {
	js := map[string]any{
		"sync": map[string]any{
			"roots": []any{
				map[string]any{"instance": "ServerScriptService.src", "mappedName": "ServerScriptService.src"},
			},
		},
	}
	if !syncJSONReady(js) {
		t.Fatal("expected ready")
	}
	js = map[string]any{
		"sync": map[string]any{
			"roots": []any{
				map[string]any{"instance": "ServerScriptService.src", "status": "Enum.InstanceFileSyncStatus.NotSynced"},
			},
		},
	}
	if syncJSONReady(js) {
		t.Fatal("expected not ready")
	}
}
