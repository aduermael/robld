//go:build !darwin

package main

func runSave() {
	fail(exitError, "ERROR: robld save is macOS-only for now.\nOn Windows we can add SendInput later; do not restart Studio to persist the place file.")
}
