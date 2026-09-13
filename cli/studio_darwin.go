//go:build darwin

package main

import (
	"fmt"
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
		Running: func() bool { return studioRunning(findStudio()) },
	})
}

func quitStudioGraceful(wait time.Duration) error {
	_, _ = runOSAScript(`tell application id "com.Roblox.RobloxStudio" to quit`, studioAppleEventTimeout)
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if !studioRunning(findStudio()) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !studioRunning(findStudio()) {
		return nil
	}
	return fmt.Errorf("Roblox Studio did not quit")
}
