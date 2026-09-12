package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func waitUntilSyncReady() []string {
	info("")
	info("%s", color("\033[33m", "Waiting for Script Sync (plugin ROBUILD_JSON or mapped folders)…"))
	t0 := time.Now()
	deadline := t0.Add(waitFor)
	lastPrint := time.Time{}
	var last map[string]any
	for time.Now().Before(deadline) {
		time.Sleep(pollEvery)
		scripts := listScripts()
		if js := latestRobuildJSON(t0); js != nil {
			last = js
			if syncJSONReady(js) {
				info("Studio plugin reports Script Sync roots are bound.")
				return scripts
			}
		}
		if js := latestPluginSettingsJSON(); js != nil {
			last = js
			if syncJSONReady(js) {
				info("Studio plugin reports Script Sync roots are bound (plugin settings).")
				return scripts
			}
		}
		if logShowsSyncResume(t0) {
			info("Studio logs show Script Sync resume.")
			return scripts
		}
		if time.Since(lastPrint) >= 15*time.Second {
			left := int(time.Until(deadline).Seconds())
			info("  still waiting (%ds left, %d Luau file(s) on disk).", left, len(scripts))
			lastPrint = time.Now()
		}
	}
	fmt.Println()
	fmt.Println("NOT_READY: Script Sync roots are not bound yet.")
	if last != nil {
		fmt.Println("Plugin last report:")
		printSyncJSONSummary(last)
	} else {
		fmt.Println("No ROBUILD_JSON from the Studio plugin yet (plugin may not have loaded).")
	}
	fmt.Println(syncInstructions())
	os.Exit(exitRetry)
	return nil
}

func syncJSONReady(js map[string]any) bool {
	sync, _ := js["sync"].(map[string]any)
	if sync == nil {
		return false
	}
	roots, _ := sync["roots"].([]any)
	if len(roots) == 0 {
		return false
	}
	for _, raw := range roots {
		row, _ := raw.(map[string]any)
		if row == nil {
			return false
		}
		if !syncRootBound(row) {
			return false
		}
	}
	return true
}

func syncRootBound(row map[string]any) bool {
	if str(row["mappedName"]) != "" || str(row["started"]) != "" {
		return true
	}
	st := strings.ToLower(str(row["status"]))
	if strings.Contains(st, "notsynced") || strings.Contains(st, "not synced") || strings.Contains(st, "errored") {
		return false
	}
	return strings.Contains(st, "syncedasroot") || strings.Contains(st, "syncedasdescendant")
}

func printSyncJSONSummary(js map[string]any) {
	sync, _ := js["sync"].(map[string]any)
	if sync == nil {
		return
	}
	if sv, ok := sync["services"].(map[string]any); ok {
		fmt.Printf("  services: %v\n", sv)
	}
	roots, _ := sync["roots"].([]any)
	for _, raw := range roots {
		row, _ := raw.(map[string]any)
		if row == nil {
			continue
		}
		fmt.Printf("  %s  status=%s mapped=%s started=%s\n",
			str(row["instance"]), str(row["status"]), str(row["mappedName"]), str(row["started"]))
	}
}

func str(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func latestRobuildJSON(after time.Time) map[string]any {
	var newest string
	var newestTime time.Time
	for _, f := range studioLogFiles() {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		if st.ModTime().Before(after.Add(-8 * time.Second)) {
			continue
		}
		if newest == "" || st.ModTime().After(newestTime) {
			newest, newestTime = f, st.ModTime()
		}
	}
	if newest == "" {
		return nil
	}
	b, err := os.ReadFile(newest)
	if err != nil {
		return nil
	}
	var payload string
	for _, line := range strings.Split(string(b), "\n") {
		i := strings.Index(line, "ROBUILD_JSON:")
		if i < 0 {
			continue
		}
		payload = strings.TrimSpace(line[i+len("ROBUILD_JSON:"):])
	}
	if payload == "" {
		return nil
	}
	var js map[string]any
	if err := json.Unmarshal([]byte(payload), &js); err != nil {
		return nil
	}
	return js
}

func logShowsSyncResume(after time.Time) bool {
	for _, f := range studioLogFiles() {
		st, err := os.Stat(f)
		if err != nil || st.ModTime().Before(after.Add(-8*time.Second)) {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if logTextShowsSyncResume(string(b)) {
			return true
		}
	}
	return false
}

func logTextShowsSyncResume(s string) bool {
	if strings.Contains(s, "ROBUILD_JSON:") && strings.Contains(s, "SyncedAsRoot") {
		return true
	}
	if strings.Contains(s, "synced hierarchies") && !strings.Contains(s, "did not resume syncing") {
		return true
	}
	if strings.Contains(s, "File_Sync") && strings.Contains(strings.ToLower(s), "syncing") && !strings.Contains(s, "did not resume syncing") {
		return true
	}
	return false
}

func latestPluginSettingsJSON() map[string]any {
	var best map[string]any
	var bestTime time.Time
	for _, f := range pluginSettingsFiles() {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		js := jsonFromPluginSettings(f)
		if js == nil {
			continue
		}
		if best == nil || st.ModTime().After(bestTime) {
			best, bestTime = js, st.ModTime()
		}
	}
	return best
}

func pluginSettingsFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	var out []string
	for _, base := range []string{
		filepath.Join(home, "Documents", "Roblox"),
		filepath.Join(home, "Documents", "ROBLOX"),
	} {
		for _, pat := range []string{
			filepath.Join(base, "*", "InstalledPlugins", "0", "settings.json"),
			filepath.Join(base, "InstalledPlugins", "0", "settings.json"),
		} {
			matches, _ := filepath.Glob(pat)
			out = append(out, matches...)
		}
	}
	return out
}

func jsonFromPluginSettings(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	v, ok := raw["robuildDump"]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case string:
		var js map[string]any
		if json.Unmarshal([]byte(t), &js) == nil {
			return js
		}
	case map[string]any:
		return t
	}
	return nil
}

func studioLogFiles() []string {
	var globs []string
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" && home != "" {
		globs = append(globs, filepath.Join(home, "Library", "Logs", "Roblox", "*.log"))
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			globs = append(globs, filepath.Join(local, "Roblox", "logs", "*.log"))
		}
	}
	var out []string
	for _, g := range globs {
		matches, _ := filepath.Glob(g)
		out = append(out, matches...)
	}
	return out
}
