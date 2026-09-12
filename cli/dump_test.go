package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpWritesReport(t *testing.T) {
	root = t.TempDir()
	placePath = filepath.Join(root, "place.json")
	dump := filepath.Join(root, "out-dump")
	t.Setenv("ROBUILD_DUMP_DIR", dump)
	t.Setenv("ROBUILD_GLOBAL_SETTINGS", filepath.Join(root, "missing.xml"))
	if err := os.WriteFile(placePath, []byte(`{"localPlaceFile":"place.rbxlx","name":"DumpTest"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "place.rbxlx"), []byte("<roblox/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	runDump()

	report, err := os.ReadFile(filepath.Join(dump, "REPORT.txt"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(report)
	for _, part := range []string{"robld dump", "DumpTest", "placeIDEState", "intended sync map", "Script Sync resume records"} {
		if !strings.Contains(s, part) {
			t.Fatalf("REPORT missing %q\n%s", part, s)
		}
	}
}
