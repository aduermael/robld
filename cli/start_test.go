package main

import "testing"

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
