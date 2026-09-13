package main

import "testing"

func TestStudioNeedsRestart(t *testing.T) {
	if studioNeedsRestart(true, restartNeeds{}) {
		t.Fatal("already-applied prefs/plugin/resume must not restart Studio")
	}
	if studioNeedsRestart(false, restartNeeds{Prefs: true, Plugin: true, Resume: true}) {
		t.Fatal("must not restart when Studio is not running")
	}
	cases := []restartNeeds{
		{Prefs: true},
		{Plugin: true},
		{Resume: true},
	}
	for _, n := range cases {
		if !studioNeedsRestart(true, n) {
			t.Fatalf("expected restart for %+v", n)
		}
	}
}
