//go:build !darwin

package main

import (
	"fmt"
	"time"
)

func saveStudioPlace(p place) error {
	return fmt.Errorf("save is macOS-only for now")
}

func saveAndQuitStudio(p place) error {
	return runSaveQuitPlan(saveQuitHooks{
		Save: func() error { return saveStudioPlace(p) },
		Quit: func(time.Duration) error {
			return fmt.Errorf("automatic Studio restart is macOS-only for now")
		},
		Enter: func() error { return nil },
		Kill: func(time.Duration) error {
			return fmt.Errorf("automatic Studio restart is macOS-only for now")
		},
		Running: func() bool { return true },
	})
}
