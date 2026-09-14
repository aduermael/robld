//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runSave() {
	studio := findStudio()
	if studio == "" || !studioRunning(studio) {
		fail(exitError, "ERROR: Roblox Studio is not running.\nStart it first, then save the place.")
	}

	p := loadPlace()
	path := absFromRepo(p.LocalPlaceFile)
	if path == "" {
		warn("place.json has no localPlaceFile. Save to File may not write a git place file.")
	}

	if err := saveStudioPlace(p); err != nil {
		msg := err.Error()
		if isAccessibilityError(msg) {
			fail(exitError, "ERROR: macOS blocked keystrokes to Studio (Accessibility).\n"+
				"  System Settings → Privacy & Security → Accessibility\n"+
				"  Enable the app that launched this tool (Terminal, iTerm, Grok, …).\n"+
				"  %s", strings.TrimSpace(msg))
		}
		warn("Studio did not update %s.", repoRel(path))
		warn("Nothing to save, Save went to cloud, or a dialog is in front. Click Studio once and retry.")
		os.Exit(exitRetry)
	}

	if path == "" {
		info("Sent Save to File to Roblox Studio.")
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		warn("Save sent, but %s is missing.", repoRel(path))
		os.Exit(exitRetry)
	}
	info("Updated %s (%d bytes).", repoRel(path), st.Size())
	fmt.Println(color("\033[32;1m", "READY") + color("\033[32m", ": place file saved"))
}

func saveStudioPlace(p place) error {
	path := absFromRepo(p.LocalPlaceFile)
	var before time.Time
	var beforeSize int64
	if path != "" {
		if st, err := os.Stat(path); err == nil {
			before = st.ModTime()
			beforeSize = st.Size()
		}
	}

	if err := sendStudioScript(studioSaveToFileScript); err != nil {
		return err
	}
	wait := saveWaitDuration()
	if path == "" {
		return nil
	}
	if waitPlaceChanged(path, before, beforeSize, wait) {
		return nil
	}

	if err := sendStudioScript(studioSaveCmdSScript); err != nil {
		return err
	}
	if waitPlaceChanged(path, before, beforeSize, wait) {
		return nil
	}
	return fmt.Errorf("Studio did not update %s within %s", repoRel(path), wait)
}

func sendStudioScript(build func(int) string) error {
	pid, err := studioPID()
	if err != nil {
		return err
	}
	_, err = runOSAScript(build(pid), studioAppleEventTimeout)
	return err
}

func runOSAScript(script string, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = studioAppleEventTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("osascript timed out after %s: %s", timeout, strings.TrimSpace(string(out)))
	}
	if err != nil {
		return out, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func isAccessibilityError(msg string) bool {
	low := strings.ToLower(msg)
	return strings.Contains(low, "not allowed assistive") ||
		strings.Contains(low, "accessibility") ||
		strings.Contains(low, "not authorized") ||
		strings.Contains(low, "(-1719)") ||
		strings.Contains(low, "(-1743)")
}
