package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Script Sync UI settings live on this machine in Studio's GlobalSettings XML
// (rbxmx), not in place.rbxlx. Per-place Sync-to-folder bindings are a
// different store — see robld scan.

type studioPref struct {
	UI     string
	Name   string
	Tag    string // bool or token
	Want   string
	Labels map[string]string
}

var scriptSyncPrefs = []studioPref{
	{
		UI:   "Auto resume sync on place open",
		Name: "AutoResumeSyncOnPlaceOpen",
		Tag:  "bool",
		Want: "true",
		Labels: map[string]string{
			"true":  "on",
			"false": "off",
		},
	},
	{
		UI:   "Resume conflicted sync on place open",
		Name: "ActionOnAutoResumeSync",
		Tag:  "token",
		Want: "2",
		Labels: map[string]string{
			"0": "DontResume (do not resume)",
			"1": "KeepStudio (always keep Studio)",
			"2": "KeepLocal (always keep local)",
		},
	},
	{
		UI:   "Keep local files/directories after Stop Sync",
		Name: "ActionOnStopSync",
		Tag:  "token",
		Want: "1",
		Labels: map[string]string{
			"0": "AlwaysAsk",
			"1": "KeepLocalFiles",
			"2": "DeleteLocalFiles",
		},
	},
	{
		UI:   "File extension",
		Name: "DefaultScriptSyncFileType",
		Tag:  "token",
		Want: "1",
		Labels: map[string]string{
			"0": "Lua",
			"1": "Luau",
		},
	},
}

// MCP: Studio "Enable Studio as MCP server" (Assistant → Manage MCP Servers) also lives in
// GlobalSettings. robld should turn that on before launch so agents never rely on a manual
// toggle. Property name TBD — capture with `robld dump` / GlobalSettings on a machine where
// the setting is on, then add it to extraStudioPrefs like the rows below.
// Extra Studio properties that make the robld plugin usable without a restart.
var extraStudioPrefs = []studioPref{
	{
		UI:   "Reload local plugins on change",
		Name: "ReloadLocalPluginsOnChange",
		Tag:  "bool",
		Want: "true",
		Labels: map[string]string{
			"true":  "on",
			"false": "off",
		},
	},
}

func allPatchPrefs() []studioPref {
	out := make([]studioPref, 0, len(scriptSyncPrefs)+len(extraStudioPrefs))
	out = append(out, scriptSyncPrefs...)
	out = append(out, extraStudioPrefs...)
	return out
}

func runPrefs(action string) {
	path, extras := findGlobalSettings()
	if path == "" {
		fmt.Println("ERROR: Studio GlobalSettings_13.xml was not found.")
		fmt.Println("Open Roblox Studio once so it creates the file, then re-run.")
		fmt.Println("Looked in:")
		for _, d := range studioSettingsDirs() {
			fmt.Printf("  %s\n", d)
		}
		os.Exit(exitError)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		fail(exitError, "ERROR: reading %s: %v", path, err)
	}
	values, err := readStudioProps(raw)
	if err != nil {
		fail(exitError, "ERROR: %s\n  %v", path, err)
	}

	running := studioRunning(findStudio())
	printPrefsReport(path, extras, values, running)

	switch action {
	case "", "show":
		if prefsNeedApply(values) {
			fmt.Println()
			if running {
				fmt.Println("NOT_READY: close Roblox Studio, then:  robld prefs apply")
				fmt.Println("Studio writes this file on quit, so a patch while it is open will not stick.")
				os.Exit(exitRetry)
			}
			fmt.Println("To write the agent defaults:  robld prefs apply")
		}
		return
	case "apply":
		if running {
			fmt.Println()
			fmt.Println("NOT_READY: close Roblox Studio, then re-run:  robld prefs apply")
			fmt.Println("Studio overwrites GlobalSettings_13.xml on quit.")
			os.Exit(exitRetry)
		}
		out, changed, err := patchStudioSettings(raw, allPatchPrefs())
		if err != nil {
			fail(exitError, "ERROR: %v", err)
		}
		if !changed {
			fmt.Println()
			fmt.Println(color("\033[32;1m", "READY") + color("\033[32m", ": Script Sync prefs already match"))
			return
		}
		if err := replaceFile(path, out); err != nil {
			fail(exitError, "ERROR: writing %s: %v", path, err)
		}
		fmt.Println()
		fmt.Println(color("\033[32;1m", "READY") + color("\033[32m", ": wrote Script Sync prefs"))
		fmt.Printf("  %s\n", path)
		fmt.Println("  backup: " + path + ".robld-bak")
		fmt.Println("These do not restore per-place Sync to… folder bindings.")
	default:
		fail(exitError, "Unknown prefs action %q. Use: robld prefs | robld prefs apply", action)
	}
}

func printPrefsReport(path string, extras []string, values map[string]string, running bool) {
	fmt.Println("Script Sync preferences (this machine, not the place file)")
	fmt.Printf("  file: %s\n", path)
	if st, err := os.Stat(path); err == nil {
		fmt.Printf("  mtime: %s  (%d bytes)\n", st.ModTime().Format(time.RFC3339), st.Size())
	}
	if running {
		fmt.Println("  Studio: running — in-memory Settings may be newer than this file until quit.")
	} else {
		fmt.Println("  Studio: not running")
	}
	for _, extra := range extras {
		if extra != path {
			fmt.Printf("  also found: %s\n", extra)
		}
	}
	fmt.Println()
	for _, p := range scriptSyncPrefs {
		cur, ok := values[p.Name]
		want := formatPref(p, p.Want)
		status := "ok"
		var shown string
		if !ok {
			shown = "not in file (Studio default)"
			status = "WANT " + want
		} else {
			shown = formatPref(p, cur)
			if normalizePref(cur) != normalizePref(p.Want) {
				status = "WANT " + want
			}
		}
		fmt.Printf("  %-48s  %-28s  %s\n", p.UI, shown, status)
		fmt.Printf("    %s.%s\n", "Studio", p.Name)
	}
	for _, p := range extraStudioPrefs {
		cur, ok := values[p.Name]
		if !ok {
			fmt.Printf("  %-48s  not in file  WANT %s\n", p.UI, formatPref(p, p.Want))
		} else {
			status := "ok"
			if normalizePref(cur) != normalizePref(p.Want) {
				status = "WANT " + formatPref(p, p.Want)
			}
			fmt.Printf("  %-48s  %-28s  %s\n", p.UI, formatPref(p, cur), status)
		}
	}
	fmt.Println()
	fmt.Println("These are per-machine Studio Settings → Script Sync.")
	fmt.Println("They do not store which Explorer folders are synced, or to which disk path.")
}

func formatPref(p studioPref, raw string) string {
	raw = normalizePref(raw)
	if lab, ok := p.Labels[raw]; ok {
		if p.Tag == "token" {
			return lab + " (" + raw + ")"
		}
		return lab
	}
	if raw == "" {
		return "(empty)"
	}
	return raw
}

func normalizePref(s string) string {
	return strings.TrimSpace(s)
}

func prefsNeedApply(values map[string]string) bool {
	for _, p := range scriptSyncPrefs {
		cur, ok := values[p.Name]
		if !ok || normalizePref(cur) != normalizePref(p.Want) {
			return true
		}
	}
	return false
}

func applyScriptSyncPrefsIfNeeded() {
	path, _ := findGlobalSettings()
	if path == "" {
		warn("Studio GlobalSettings_13.xml not found yet — Script Sync prefs will be writable after Studio has launched once.")
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		warn("Could not read %s: %v", path, err)
		return
	}
	out, changed, err := patchStudioSettings(raw, allPatchPrefs())
	if err != nil {
		warn("Could not patch Script Sync prefs: %v", err)
		return
	}
	if !changed {
		info("Script Sync prefs: auto-resume on, conflicted=KeepLocal, stop=KeepLocalFiles, files=.luau")
		return
	}
	if err := replaceFile(path, out); err != nil {
		warn("Could not write %s: %v", path, err)
		return
	}
	info("Wrote Script Sync prefs (Keep local / auto-resume) to %s", path)
}

func reportScriptSyncPrefsBrief() {
	path, _ := findGlobalSettings()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	values, err := readStudioProps(raw)
	if err != nil {
		return
	}
	if !prefsNeedApply(values) {
		info("Script Sync prefs file matches agent defaults (Studio may still have unsaved Settings until quit).")
		return
	}
	warn("Script Sync prefs in %s are not agent defaults.", path)
	warn("If you just changed them in Studio, they land in this file when Studio quits.")
	warn("Otherwise close Studio and run:  robld prefs apply")
}

func studioSettingsDirs() []string {
	var dirs []string
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		dirs = append(dirs,
			filepath.Join(home, "Library", "Roblox"),
			filepath.Join(home, "Library", "Application Support", "Roblox"),
		)
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			dirs = append(dirs, filepath.Join(local, "Roblox"))
		}
	}
	return dirs
}

func findGlobalSettings() (primary string, all []string) {
	if p := strings.TrimSpace(os.Getenv("ROBUILD_GLOBAL_SETTINGS")); p != "" {
		return p, []string{p}
	}
	seen := map[string]bool{}
	var ranked []string
	for _, dir := range studioSettingsDirs() {
		for _, pat := range []string{
			filepath.Join(dir, "GlobalSettings_13.xml"),
			filepath.Join(dir, "*", "GlobalSettings_13.xml"),
		} {
			matches, _ := filepath.Glob(pat)
			for _, m := range matches {
				abs, err := filepath.Abs(m)
				if err != nil {
					abs = m
				}
				if seen[abs] {
					continue
				}
				st, err := os.Stat(abs)
				if err != nil || st.IsDir() {
					continue
				}
				seen[abs] = true
				all = append(all, abs)
				b, err := os.ReadFile(abs)
				if err == nil && strings.Contains(string(b), `class="Studio"`) {
					ranked = append(ranked, abs)
				}
			}
		}
	}
	if len(ranked) == 0 {
		return "", all
	}
	// Prefer the well-known top-level file over versioned subfolders.
	for _, dir := range studioSettingsDirs() {
		want := filepath.Join(dir, "GlobalSettings_13.xml")
		for _, p := range ranked {
			if sameFilePath(p, want) {
				return p, all
			}
		}
	}
	newest := ranked[0]
	var newestTime time.Time
	if st, err := os.Stat(newest); err == nil {
		newestTime = st.ModTime()
	}
	for _, p := range ranked[1:] {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.ModTime().After(newestTime) {
			newest, newestTime = p, st.ModTime()
		}
	}
	return newest, all
}

func sameFilePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func readStudioProps(raw []byte) (map[string]string, error) {
	block, err := studioPropertiesBlock(string(raw))
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`<(bool|token|string|int|float|double|QDir)\s+name="([^"]+)"\s*>([^<]*)</(?:bool|token|string|int|float|double|QDir)>`)
	out := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(block, -1) {
		out[m[2]] = strings.TrimSpace(m[3])
	}
	return out, nil
}

func studioPropertiesBlock(xml string) (string, error) {
	item := strings.Index(xml, `<Item class="Studio"`)
	if item < 0 {
		return "", fmt.Errorf("no <Item class=\"Studio\"> in GlobalSettings")
	}
	props := strings.Index(xml[item:], "<Properties>")
	if props < 0 {
		return "", fmt.Errorf("Studio item has no <Properties>")
	}
	props += item
	end := strings.Index(xml[props:], "</Properties>")
	if end < 0 {
		return "", fmt.Errorf("Studio item has no </Properties>")
	}
	return xml[props : props+end+len("</Properties>")], nil
}

func patchStudioSettings(raw []byte, prefs []studioPref) ([]byte, bool, error) {
	xml := string(raw)
	item := strings.Index(xml, `<Item class="Studio"`)
	if item < 0 {
		return nil, false, fmt.Errorf("no <Item class=\"Studio\"> in GlobalSettings")
	}
	props := strings.Index(xml[item:], "<Properties>")
	if props < 0 {
		return nil, false, fmt.Errorf("Studio item has no <Properties>")
	}
	props += item
	endRel := strings.Index(xml[props:], "</Properties>")
	if endRel < 0 {
		return nil, false, fmt.Errorf("Studio item has no </Properties>")
	}
	end := props + endRel
	block := xml[props:end]
	changed := false
	indent := detectPropIndent(block)
	nl := "\n"
	if strings.Contains(block, "\r\n") {
		nl = "\r\n"
	}
	for _, p := range prefs {
		next, did, err := setLeafProp(block, p.Tag, p.Name, p.Want, indent, nl)
		if err != nil {
			return nil, false, err
		}
		block = next
		if did {
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	out := xml[:props] + block + xml[end:]
	return []byte(out), true, nil
}

func detectPropIndent(block string) string {
	lines := strings.Split(block, "\n")
	for _, raw := range lines[1:] {
		line := strings.TrimRight(raw, "\r")
		trim := strings.TrimLeft(line, " \t")
		if trim == "" {
			continue
		}
		return line[:len(line)-len(trim)]
	}
	return "\t\t"
}

func setLeafProp(block, tag, name, value, indent, nl string) (string, bool, error) {
	re := regexp.MustCompile(`(?s)<(bool|token|string|int)\s+name="` + regexp.QuoteMeta(name) + `"\s*>\s*([^<]*?)\s*</(?:bool|token|string|int)>`)
	if loc := re.FindStringSubmatchIndex(block); loc != nil {
		curTag := block[loc[2]:loc[3]]
		cur := strings.TrimSpace(block[loc[4]:loc[5]])
		if cur == value && curTag == tag {
			return block, false, nil
		}
		repl := fmt.Sprintf("<%s name=\"%s\">%s</%s>", tag, name, value, tag)
		return block[:loc[0]] + repl + block[loc[1]:], true, nil
	}
	line := indent + fmt.Sprintf("<%s name=\"%s\">%s</%s>", tag, name, value, tag)
	return insertBeforeTrailingIndent(block, line, nl), true, nil
}

// insertBeforeTrailingIndent keeps the whitespace that indented </Properties>.
func insertBeforeTrailingIndent(block, line, nl string) string {
	i := len(block)
	for i > 0 && (block[i-1] == ' ' || block[i-1] == '\t') {
		i--
	}
	prefix, suffix := block[:i], block[i:]
	if !strings.HasSuffix(prefix, "\n") && !strings.HasSuffix(prefix, "\r\n") {
		prefix += nl
	}
	return prefix + line + nl + suffix
}

func replaceFile(path string, data []byte) error {
	st, err := os.Stat(path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = st.Mode()
		orig, readErr := os.ReadFile(path)
		if readErr == nil {
			_ = os.WriteFile(path+".robld-bak", orig, mode)
		}
	}
	tmp := path + ".robld-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if err2 := os.WriteFile(path, data, mode); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
		_ = os.Remove(tmp)
	}
	return nil
}
