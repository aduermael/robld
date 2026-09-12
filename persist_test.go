package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodePlaceKeyPath(t *testing.T) {
	got := encodePlaceKeyPath("/Users/aduermael/Documents/repos/misc/place.rbxlx")
	want := ".Users.aduermael.Documents.repos.misc.place·rbxlx"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniqueIDToUUID(t *testing.T) {
	got := uniqueIDToUUID("684fc65d7df217a30ab726ca000003b9")
	want := "684fc65d-7df2-17a3-0ab7-26ca000003b9"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParsePlaceInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "place.rbxlx")
	xml := `<?xml version="1.0"?>
<roblox version="4">
	<Item class="Workspace" referent="W">
		<Properties>
			<string name="Name">Workspace</string>
			<UniqueId name="UniqueId">684fc65d7df217a30ab726ca00000002</UniqueId>
		</Properties>
	</Item>
	<Item class="ReplicatedStorage" referent="RS">
		<Properties>
			<string name="Name">ReplicatedStorage</string>
			<UniqueId name="UniqueId">684fc65d7df217a30ab726ca000003b9</UniqueId>
		</Properties>
		<Item class="Folder" referent="F">
			<Properties>
				<string name="Name">shared</string>
				<UniqueId name="UniqueId">67fac8367b92ca860ab7400000000705</UniqueId>
			</Properties>
		</Item>
	</Item>
	<Item class="StarterPlayer" referent="SP">
		<Properties>
			<string name="Name">StarterPlayer</string>
			<UniqueId name="UniqueId">aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa</UniqueId>
		</Properties>
		<Item class="StarterPlayerScripts" referent="SPS">
			<Properties>
				<string name="Name">StarterPlayerScripts</string>
				<UniqueId name="UniqueId">bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb</UniqueId>
			</Properties>
			<Item class="Folder" referent="C">
				<Properties>
					<string name="Name">src</string>
					<UniqueId name="UniqueId">67fac8367b92ca860ab7400000000708</UniqueId>
				</Properties>
			</Item>
		</Item>
	</Item>
</roblox>
`
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, byPath, err := parsePlaceInstances(path)
	if err != nil {
		t.Fatal(err)
	}
	if ws != "684fc65d-7df2-17a3-0ab7-26ca00000002" {
		t.Fatalf("workspace uid %q", ws)
	}
	shared := byPath["ReplicatedStorage.shared"]
	if shared.Class != "Folder" || uniqueIDToUUID(shared.UniqueID) != "67fac836-7b92-ca86-0ab7-400000000705" {
		t.Fatalf("shared %+v", shared)
	}
	src := byPath["StarterPlayer.StarterPlayerScripts.src"]
	if src.Class != "Folder" || !strings.HasPrefix(src.UniqueID, "67fac836") {
		t.Fatalf("src %+v", src)
	}
}

func TestParseRepoPlaceServiceRoots(t *testing.T) {
	path := "place.rbxlx"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no place.rbxlx in module root")
	}
	ws, byPath, err := parsePlaceInstances(path)
	if err != nil {
		t.Fatal(err)
	}
	if ws != "684fc65d-7df2-17a3-0ab7-26ca00000002" {
		t.Fatalf("workspace uid %q", ws)
	}
	want := map[string]string{
		"ServerScriptService":                "ServerScriptService",
		"ReplicatedStorage":                  "ReplicatedStorage",
		"StarterPlayer.StarterPlayerScripts": "StarterPlayerScripts",
	}
	for path, class := range want {
		inst, ok := byPath[path]
		if !ok || inst.Class != class || inst.UniqueID == "" {
			t.Fatalf("%s %+v ok=%v", path, inst, ok)
		}
		if !strings.HasPrefix(uniqueIDToUUID(inst.UniqueID), "684fc65d-") {
			t.Fatalf("%s uuid %s", path, uniqueIDToUUID(inst.UniqueID))
		}
	}
}
