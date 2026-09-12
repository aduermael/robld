//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"time"
)

func quitStudio() error {
	_ = exec.Command("osascript", "-e", `tell application id "com.Roblox.RobloxStudio" to quit`).Run()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if !studioRunning(findStudio()) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = exec.Command("pkill", "-x", "RobloxStudio").Run()
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !studioRunning(findStudio()) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("Roblox Studio did not quit")
}

func bestEffortStudioSave() {
	if !studioRunning(findStudio()) {
		return
	}
	_ = sendStudioSave()
}
