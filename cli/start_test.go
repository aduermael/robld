package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	t.Setenv("FORCE_COLOR", "0")
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestPrintReadySplitsMCP(t *testing.T) {
	root = t.TempDir()
	out := captureStdout(t, func() {
		printReady(place{Name: "Slash Waves", LocalPlaceFile: "place.rbxlx"}, nil, false)
	})
	if !strings.Contains(out, "READY: Script Sync") {
		t.Fatalf("expected Script Sync ready:\n%s", out)
	}
	if strings.Contains(out, "READY: Script Sync + MCP") {
		t.Fatalf("empty MCP must not claim MCP:\n%s", out)
	}
	if !strings.Contains(out, "NEED_USER:") || !strings.Contains(out, "Manage MCP Servers") {
		t.Fatalf("expected Assistant MCP toggle NEED_USER:\n%s", out)
	}

	ok := captureStdout(t, func() {
		printReady(place{Name: "Slash Waves"}, nil, true)
	})
	if !strings.Contains(ok, "READY: Script Sync + MCP") {
		t.Fatalf("non-empty tools may claim MCP:\n%s", ok)
	}
}

func TestParseArgsNew(t *testing.T) {
	opts := parseArgs([]string{"--new", "My Game"})
	if !opts.newPlace || opts.newName != "My Game" {
		t.Fatalf("got %+v", opts)
	}
}

func TestParseArgsSave(t *testing.T) {
	opts := parseArgs([]string{"save"})
	if opts.cmd != "save" {
		t.Fatalf("got %+v", opts)
	}
}

func TestParseArgsVersionUpdateHelp(t *testing.T) {
	v := parseArgs([]string{"--version"})
	if v.cmd != "version" {
		t.Fatalf("version flag: %+v", v)
	}
	v2 := parseArgs([]string{"version"})
	if v2.cmd != "version" {
		t.Fatalf("version cmd: %+v", v2)
	}
	u := parseArgs([]string{"--update"})
	if u.cmd != "update" {
		t.Fatalf("update flag: %+v", u)
	}
	u2 := parseArgs([]string{"update"})
	if u2.cmd != "update" {
		t.Fatalf("update cmd: %+v", u2)
	}
	h := parseArgs([]string{"--help"})
	if h.cmd != "help" {
		t.Fatalf("help: %+v", h)
	}
}

func TestParseArgsPrefsAndScan(t *testing.T) {
	prefs := parseArgs([]string{"prefs"})
	if prefs.cmd != "prefs" || prefs.prefsAction != "" {
		t.Fatalf("prefs: %+v", prefs)
	}
	apply := parseArgs([]string{"prefs", "apply"})
	if apply.cmd != "prefs" || apply.prefsAction != "apply" {
		t.Fatalf("apply: %+v", apply)
	}
	scan := parseArgs([]string{"scan"})
	if scan.cmd != "scan" {
		t.Fatalf("scan: %+v", scan)
	}
}

func TestParseTarget(t *testing.T) {
	cases := []struct {
		in   string
		want place
	}{
		{"123456789", place{PlaceID: 123456789}},
		{"https://www.roblox.com/games/920587237/Adopt-Me", place{
			PlaceID: 920587237,
			GameURL: "https://www.roblox.com/games/920587237/Adopt-Me",
		}},
		{"https://www.roblox.com/games/920587237/Adopt-Me?extra=1", place{
			PlaceID: 920587237,
			GameURL: "https://www.roblox.com/games/920587237/Adopt-Me?extra=1",
		}},
		{"placeId=920587237&universeId=1", place{PlaceID: 920587237, UniverseID: 1}},
		{"  920587237  ", place{PlaceID: 920587237}},
		{"https://create.roblox.com/dashboard/creations/experiences/383310974/places/920587237", place{
			PlaceID:    920587237,
			UniverseID: 383310974,
			GameURL:    "https://create.roblox.com/dashboard/creations/experiences/383310974/places/920587237",
		}},
		{"game.rbxl", place{LocalPlaceFile: "game.rbxl"}},
	}
	for _, tc := range cases {
		got := parseTarget(tc.in)
		if got != tc.want {
			t.Errorf("parseTarget(%q)\n got %+v\nwant %+v", tc.in, got, tc.want)
		}
	}
}
