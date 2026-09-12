//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const prefsDomain = "com.roblox.RobloxStudio"

func studioPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Preferences", "com.roblox.RobloxStudio.plist")
}

func readSyncPersistence(placeAbs, workspaceUUID string) ([]syncBinding, error) {
	key := persistRecordPrefix(placeAbs, workspaceUUID)
	out, err := exec.Command("defaults", "read", prefsDomain, key).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("defaults read: %v %s", err, strings.TrimSpace(string(out)))
	}
	raw := strings.TrimSpace(string(out))
	if strings.HasPrefix(raw, "\"") {
		if u, err := strconv.Unquote(raw); err == nil {
			raw = u
		}
	}
	var bindings []syncBinding
	if err := json.Unmarshal([]byte(raw), &bindings); err != nil {
		return nil, fmt.Errorf("parse persist JSON: %w", err)
	}
	return bindings, nil
}

func writeSyncPersistence(keys map[string]any) error {
	for k, v := range keys {
		args := []string{"write", prefsDomain, k}
		switch t := v.(type) {
		case string:
			args = append(args, "-string", t)
		case bool:
			if t {
				args = append(args, "-bool", "true")
			} else {
				args = append(args, "-bool", "false")
			}
		case int64:
			args = append(args, "-integer", strconv.FormatInt(t, 10))
		case int:
			args = append(args, "-integer", strconv.Itoa(t))
		case float64:
			args = append(args, "-integer", strconv.FormatInt(int64(t), 10))
		default:
			b, err := json.Marshal(t)
			if err != nil {
				return err
			}
			args = append(args, "-string", string(b))
		}
		out, err := exec.Command("defaults", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("defaults write %s: %v %s", k, err, strings.TrimSpace(string(out)))
		}
	}
	_ = exec.Command("killall", "cfprefsd").Run()
	time.Sleep(300 * time.Millisecond)
	return nil
}
