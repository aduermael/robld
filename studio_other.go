//go:build !darwin

package main

import "fmt"

func quitStudio() error {
	return fmt.Errorf("automatic Studio restart is macOS-only for now; quit Studio and re-run robuild")
}

func bestEffortStudioSave() {}
