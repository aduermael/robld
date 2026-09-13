package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatUniqueIDMatchesStudioLayout(t *testing.T) {
	// Workspace UniqueId from this repo's Studio-saved place.rbxlx.
	var random [8]byte
	b, err := hex.DecodeString("684fc65d7df217a3")
	if err != nil {
		t.Fatal(err)
	}
	copy(random[:], b)
	got := formatUniqueID(random, 0x0ab726ca, 2)
	want := "684fc65d7df217a30ab726ca00000002"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUniqueIDTimeEpoch(t *testing.T) {
	if got := uniqueIDTime(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)); got != 0 {
		t.Fatalf("epoch got %d", got)
	}
	// Same instant as the Studio-saved Workspace UniqueId time field.
	when := time.Date(2026, 9, 12, 17, 32, 58, 0, time.UTC)
	if got := uniqueIDTime(when); got != 0x0ab726ca {
		t.Fatalf("got 0x%x want 0x0ab726ca", got)
	}
}

func TestInjectMissingUniqueIdsSeedsServices(t *testing.T) {
	if strings.Contains(minimalPlaceXML, `<UniqueId name="UniqueId">`) {
		t.Fatal("template should not hardcode UniqueIds")
	}
	var random [8]byte
	copy(random[:], []byte("randseed"))
	out, n := injectMissingUniqueIdsAt(minimalPlaceXML, random, 0x0ab726ca, 1)
	if n == 0 {
		t.Fatal("injected 0 UniqueIds")
	}
	if rbxlxMissingUniqueIds(out) {
		t.Fatal("still missing UniqueIds after inject")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "place.rbxlx")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, byPath, err := parsePlaceInstances(path)
	if err != nil {
		t.Fatal(err)
	}
	if ws == "" {
		t.Fatal("workspace UniqueId empty after inject")
	}
	want := []string{
		"Workspace",
		"ServerScriptService",
		"ReplicatedStorage",
		"StarterPlayer.StarterPlayerScripts",
	}
	seen := map[string]bool{}
	for _, p := range want {
		inst, ok := byPath[p]
		if !ok || inst.UniqueID == "" || len(inst.UniqueID) != 32 {
			t.Fatalf("%s %+v ok=%v", p, inst, ok)
		}
		if seen[inst.UniqueID] {
			t.Fatalf("duplicate UniqueId %s", inst.UniqueID)
		}
		seen[inst.UniqueID] = true
		if !strings.HasPrefix(inst.UniqueID, hex.EncodeToString(random[:])) {
			t.Fatalf("%s random prefix %s", p, inst.UniqueID)
		}
		if inst.UniqueID[16:24] != "0ab726ca" {
			t.Fatalf("%s time field %s", p, inst.UniqueID[16:24])
		}
	}

	again, n2 := injectMissingUniqueIdsAt(out, random, 0x0ab726ca, 99)
	if n2 != 0 || again != out {
		t.Fatalf("second inject changed file n=%d", n2)
	}
}

func TestInjectDoesNotRewriteStudioPlace(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "place.rbxlx"))
	if err != nil {
		t.Skip("no repo place.rbxlx")
	}
	src := string(raw)
	if rbxlxMissingUniqueIds(src) {
		t.Fatal("repo place.rbxlx unexpectedly missing UniqueIds")
	}
	got, n := injectMissingUniqueIdsAt(src, [8]byte{1}, 1, 1)
	if n != 0 || got != src {
		t.Fatalf("must not rewrite a Studio-saved place (n=%d)", n)
	}
}

func TestDesiredSyncBindingsAfterInject(t *testing.T) {
	root = t.TempDir()
	path := filepath.Join(root, "place.rbxlx")
	body, err := injectMissingUniqueIds(minimalPlaceXML)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	saveSyncManifest(greenfieldManifest())

	_, uid, bindings, err := desiredSyncBindings(place{LocalPlaceFile: path})
	if err != nil {
		t.Fatal(err)
	}
	if uid == "" {
		t.Fatal("empty workspace uid")
	}
	if len(bindings) != 3 {
		t.Fatalf("bindings %d: %+v", len(bindings), bindings)
	}
	got := map[string]bool{}
	for _, b := range bindings {
		got[b.ClassName] = true
		if b.ScriptID == "" || b.Status != "Syncing" {
			t.Fatalf("binding %+v", b)
		}
	}
	for _, class := range []string{"ServerScriptService", "ReplicatedStorage", "StarterPlayerScripts"} {
		if !got[class] {
			t.Fatalf("missing class %s in %+v", class, bindings)
		}
	}
	if persistNeedsWrite(place{LocalPlaceFile: path}) != true {
		// Linux readSyncPersistence errors → need write. Darwin with empty plist also true.
		t.Fatal("persistNeedsWrite should be true after seeding UniqueIds (no plist yet)")
	}
}

func TestEnsurePlaceUniqueIdsWritesFile(t *testing.T) {
	root = t.TempDir()
	path := filepath.Join(root, "place.rbxlx")
	if err := os.WriteFile(path, []byte(minimalPlaceXML), 0o644); err != nil {
		t.Fatal(err)
	}
	p := place{LocalPlaceFile: path}
	if !placeNeedsUniqueIds(p) {
		t.Fatal("template file should need UniqueIds")
	}
	if err := ensurePlaceUniqueIds(p); err != nil {
		t.Fatal(err)
	}
	if placeNeedsUniqueIds(p) {
		t.Fatal("still needs UniqueIds after ensure")
	}
	ws, byPath, err := parsePlaceInstances(path)
	if err != nil {
		t.Fatal(err)
	}
	if ws == "" || byPath["ServerScriptService"].UniqueID == "" {
		t.Fatalf("ws=%q sss=%+v", ws, byPath["ServerScriptService"])
	}
}

func TestNewPlaceXMLHasParseableUniqueIds(t *testing.T) {
	body, err := newPlaceXML()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "place.rbxlx")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, byPath, err := parsePlaceInstances(path)
	if err != nil {
		t.Fatal(err)
	}
	if ws == "" {
		t.Fatal("newPlaceXML workspace UniqueId empty")
	}
	if byPath["ReplicatedStorage"].UniqueID == "" {
		t.Fatal("newPlaceXML ReplicatedStorage UniqueId empty")
	}
}
