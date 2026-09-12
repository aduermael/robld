//go:build !darwin

package main

import "fmt"

func readSyncPersistence(placeAbs, workspaceUUID string) ([]syncBinding, error) {
	return nil, fmt.Errorf("Script Sync plist is macOS-only")
}

func writeSyncPersistence(keys map[string]any) error {
	return fmt.Errorf("Script Sync plist is macOS-only")
}
