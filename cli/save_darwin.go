//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func runSave() {
	studio := findStudio()
	if studio == "" || !studioRunning(studio) {
		fail(exitError, "ERROR: Roblox Studio is not running.\nStart it with robld first, then robld save.")
	}

	p := loadPlace()
	path := absFromRepo(p.LocalPlaceFile)
	var before time.Time
	var beforeSize int64
	if path != "" {
		if st, err := os.Stat(path); err == nil {
			before = st.ModTime()
			beforeSize = st.Size()
		} else {
			warn("No local place file yet at %s — still sending Cmd+S.", path)
		}
	} else {
		warn("place.json has no localPlaceFile. Cmd+S may save to Roblox cloud instead of disk.")
	}

	if err := sendStudioSave(); err != nil {
		msg := err.Error()
		if isAccessibilityError(msg) {
			fail(exitError, "ERROR: macOS blocked keystrokes to Studio (Accessibility).\n"+
				"  System Settings → Privacy & Security → Accessibility\n"+
				"  Enable the app that launched robld (Terminal, iTerm, Grok, …).\n"+
				"  %s", strings.TrimSpace(msg))
		}
		fail(exitError, "ERROR: could not send Save to Studio.\n  %s", strings.TrimSpace(msg))
	}
	info("Sent Cmd+S to Roblox Studio.")

	if path == "" {
		info("No place.rbxlx to watch. If this was a cloud place, Save went to Roblox.")
		return
	}

	wait := 8 * time.Second
	if v := os.Getenv("SAVE_WAIT_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			wait = time.Duration(n) * time.Second
		}
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		if st.ModTime().After(before) || st.Size() != beforeSize {
			info("Updated %s (%d bytes).", repoRel(path), st.Size())
			fmt.Println(color("\033[32;1m", "READY") + color("\033[32m", ": place file saved"))
			return
		}
	}
	warn("Studio did not update %s within %s.", repoRel(path), wait)
	warn("Nothing to save, Save went to cloud, or a dialog is in front. Click Studio once and retry.")
	os.Exit(exitRetry)
}

func sendStudioSave() error {
	pid, err := studioPID()
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`
tell application "System Events"
  set frontmost of (first process whose unix id is %d) to true
end tell
delay 0.2
tell application "System Events"
  tell (first process whose unix id is %d)
    try
      click menu item "Save" of menu "File" of menu bar 1
    on error
      keystroke "s" using command down
    end try
  end tell
end tell
`, pid, pid)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func studioPID() (int, error) {
	out, err := exec.Command("pgrep", "-x", "RobloxStudio").Output()
	if err != nil {
		out, err = exec.Command("pgrep", "-f", "RobloxStudio.app").Output()
	}
	if err != nil {
		return 0, fmt.Errorf("Roblox Studio process not found")
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pid, convErr := strconv.Atoi(strings.TrimSpace(line))
		if convErr == nil && pid > 0 {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("Roblox Studio process not found")
}

func isAccessibilityError(msg string) bool {
	low := strings.ToLower(msg)
	return strings.Contains(low, "not allowed assistive") ||
		strings.Contains(low, "accessibility") ||
		strings.Contains(low, "not authorized") ||
		strings.Contains(low, "(-1719)") ||
		strings.Contains(low, "(-1743)")
}
