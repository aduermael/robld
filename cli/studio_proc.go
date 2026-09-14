package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func studioProcessName() string {
	if runtime.GOOS == "windows" || isWSL() {
		return "RobloxStudioBeta.exe"
	}
	return "RobloxStudio"
}

// isRobloxStudioProcess is true only for the Studio app process, never
// leftover StudioMCP (whose path contains RobloxStudio.app).
func isRobloxStudioProcess(nameOrCmdline string) bool {
	s := strings.TrimSpace(nameOrCmdline)
	if s == "" || strings.Contains(s, "StudioMCP") {
		return false
	}
	path := s
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		path = s[:i]
	}
	path = strings.ReplaceAll(path, `\`, "/")
	base := filepath.Base(path)
	return base == "RobloxStudio" || strings.EqualFold(base, "RobloxStudioBeta.exe")
}

func studioRunning(_ string) bool {
	_, err := studioPID()
	return err == nil
}

func studioPID() (int, error) {
	name := studioProcessName()
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("pgrep", "-x", name).Output()
		if err != nil {
			return 0, errStudioNotRunning
		}
		return firstPID(string(out))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tasklist.exe", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH").CombinedOutput()
	if err != nil {
		return 0, errStudioNotRunning
	}
	s := string(out)
	low := strings.ToLower(s)
	if !strings.Contains(low, strings.ToLower(name)) || strings.Contains(low, "no tasks") {
		return 0, errStudioNotRunning
	}
	return firstPID(s)
}

var errStudioNotRunning = fmt.Errorf("Roblox Studio process not found")

func firstPID(text string) (int, error) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, ",") {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				if pid, err := strconv.Atoi(strings.Trim(parts[1], `" `)); err == nil && pid > 0 {
					return pid, nil
				}
			}
		}
		if pid, err := strconv.Atoi(line); err == nil && pid > 0 {
			return pid, nil
		}
		for _, f := range strings.Fields(line) {
			if pid, err := strconv.Atoi(strings.Trim(f, `"`)); err == nil && pid > 0 {
				return pid, nil
			}
		}
	}
	return 0, errStudioNotRunning
}
