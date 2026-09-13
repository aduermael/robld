//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"time"
)

func saveAndQuitStudio(p place) error {
	return runSaveQuitPlan(saveQuitHooks{
		Save: func() error { return saveStudioPlace(p) },
		Quit: quitStudioGraceful,
		Enter: func() error {
			err := sendStudioScript(studioReturnScript)
			time.Sleep(time.Second)
			return err
		},
		Kill:    quitStudioForce,
		Running: func() bool { return studioRunning(findStudio()) },
	})
}

func quitStudioGraceful(wait time.Duration) error {
	_, _ = runOSAScript(`tell application id "com.Roblox.RobloxStudio" to quit`, studioAppleEventTimeout)
	if waitUntilStudioGone(wait) {
		return nil
	}
	return fmt.Errorf("Roblox Studio did not quit")
}

func quitStudioForce(wait time.Duration) error {
	_ = exec.Command("pkill", "-x", "RobloxStudio").Run()
	if waitUntilStudioGone(wait) {
		return nil
	}
	return fmt.Errorf("Roblox Studio did not quit")
}

func waitUntilStudioGone(wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if !studioRunning(findStudio()) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return !studioRunning(findStudio())
}
