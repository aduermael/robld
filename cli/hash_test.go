package main

import (
	"strings"
	"testing"
)

func TestPlaceIDEStateForumExample(t *testing.T) {
	const path = "C:/test.rbxl"
	got := rsHash31(path)
	if got != 1853943878 {
		t.Fatalf("rsHash31(%q)=%d want 1853943878", path, got)
	}
	k := knuthHash64(path)
	if k != 18195390903440055872 {
		t.Fatalf("knuthHash64(%q)=%d want 18195390903440055872", path, k)
	}
	if placeIDEStateName(path) != "placeIDEState1853943878.xml" {
		t.Fatalf("name %s", placeIDEStateName(path))
	}
	if placeIDEStateDebuggerName(path) != "placeIDEState_18195390903440055872_DebuggerData.xml" {
		t.Fatalf("dbg %s", placeIDEStateDebuggerName(path))
	}
}

func TestParseArgsDump(t *testing.T) {
	opts := parseArgs([]string{"dump"})
	if opts.cmd != "dump" {
		t.Fatalf("got %+v", opts)
	}
}

func TestRenderPluginSource(t *testing.T) {
	root = t.TempDir()
	src, err := renderPluginSource(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"ROBUILD_JSON:", "InstanceFileSyncService", "AutoResumeSyncOnPlaceOpen"} {
		if !strings.Contains(src, part) {
			t.Fatalf("plugin source missing %q", part)
		}
	}
}
