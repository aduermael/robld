package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// All Studio Apple Events (save, quit, focus) share this timeout so osascript
// cannot hang for minutes when a dialog is up.
const studioAppleEventTimeout = 12 * time.Second

func studioSaveToFileScript(pid int) string {
	return fmt.Sprintf(`
tell application "System Events"
  set frontmost of (first process whose unix id is %d) to true
end tell
delay 0.2
tell application "System Events"
  tell (first process whose unix id is %d)
    try
      click menu item "Save to File" of menu "File" of menu bar 1
    end try
  end tell
end tell
delay 0.8
tell application "System Events"
  key code 36
end tell
`, pid, pid)
}

func studioSaveCmdSScript(pid int) string {
	return fmt.Sprintf(`
tell application "System Events"
  set frontmost of (first process whose unix id is %d) to true
end tell
delay 0.2
tell application "System Events"
  tell (first process whose unix id is %d)
    keystroke "s" using command down
  end tell
end tell
`, pid, pid)
}

func studioReturnScript(pid int) string {
	return fmt.Sprintf(`
tell application "System Events"
  set frontmost of (first process whose unix id is %d) to true
end tell
delay 0.2
tell application "System Events"
  key code 36
end tell
`, pid)
}

func saveWaitDuration() time.Duration {
	wait := 8 * time.Second
	if v := os.Getenv("SAVE_WAIT_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			wait = time.Duration(n) * time.Second
		}
	}
	return wait
}

func placeFileChanged(path string, before time.Time, beforeSize int64) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.ModTime().After(before) || st.Size() != beforeSize
}

func waitPlaceChanged(path string, before time.Time, beforeSize int64, wait time.Duration) bool {
	if path == "" {
		return false
	}
	deadline := time.Now().Add(wait)
	for {
		if placeFileChanged(path, before, beforeSize) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func saveDialogNeedUserMessage() string {
	return "NEED_USER: Roblox Studio did not quit. A Save / Don't Save / Cancel dialog is likely in front of Studio. Click Save (for a local place), then I will continue."
}
