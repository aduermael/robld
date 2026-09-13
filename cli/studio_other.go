//go:build !darwin

package main

import "fmt"

func quitStudio() error {
	return fmt.Errorf("automatic Studio restart is macOS-only for now")
}

func saveAndQuitStudio(p place) error {
	return quitStudio()
}

func saveStudioPlace(p place) error {
	return fmt.Errorf("save is macOS-only for now")
}
