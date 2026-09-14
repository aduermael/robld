package main

import (
	"os"
	"strings"
	"testing"
)

func TestIsRobloxStudioProcess(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"RobloxStudio", true},
		{"/Applications/RobloxStudio.app/Contents/MacOS/RobloxStudio", true},
		{"RobloxStudioBeta.exe", true},
		{`C:\Users\a\AppData\Local\Roblox\Versions\v\RobloxStudioBeta.exe`, true},
		{"StudioMCP", false},
		{"/Applications/RobloxStudio.app/Contents/MacOS/StudioMCP", false},
		{"/Applications/RobloxStudio.app/Contents/MacOS/StudioMCP --stdio", false},
		{"", false},
		{"RobloxCrashHandler", false},
	}
	for _, tc := range cases {
		if got := isRobloxStudioProcess(tc.in); got != tc.want {
			t.Errorf("isRobloxStudioProcess(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestNoSubstringStudioPgrep(t *testing.T) {
	files := []string{"studio_proc.go", "save_darwin.go", "start.go", "dump.go", "status.go"}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s := string(b)
		if strings.Contains(s, `pgrep", "-f"`) || strings.Contains(s, "pgrep -f") {
			t.Fatalf("%s still uses substring pgrep -f (matches StudioMCP)", f)
		}
	}
}
