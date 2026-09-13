// Bootstrap Script Sync + Studio MCP. Re-run until it prints READY.
//
// One mode: non-interactive and stateful. Pass a place id / game URL once;
// it is saved to place.json. Exit 2 means do the Studio Sync to… step, then
// run again. See START.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	exitReady = 0
	exitError = 1
	exitRetry = 2
	userAgent = "robld/1.0"
)

var (
	root      string
	placePath string
	waitFor   = 90 * time.Second
	pollEvery = 3 * time.Second

	skipDirs = map[string]bool{
		".git": true, ".grok": true, ".claude": true, ".codex": true,
		".agents": true, ".cursor": true, "node_modules": true,
		".venv": true, "__pycache__": true,
		"robuild-dump": true,
	}

	reUniverseQuery = regexp.MustCompile(`(?i)universeId=(\d+)`)
	rePlaceQuery    = regexp.MustCompile(`(?i)placeId=(\d+)`)
	reCreateDash    = regexp.MustCompile(`(?i)create\.roblox\.com/dashboard/creations/experiences/(\d+)(?:/places/(\d+))?`)
	reGames         = regexp.MustCompile(`(?i)roblox\.com/games/(\d+)`)
	reExperiences   = regexp.MustCompile(`(?i)roblox\.com/experiences/(\d+)`)
	reLongID        = regexp.MustCompile(`(\d{8,})`)
	reURLIsh        = regexp.MustCompile(`(?i)://|roblox\.com|/games/|/experiences/`)
)

type place struct {
	UniverseID     int64  `json:"universeId"`
	PlaceID        int64  `json:"placeId"`
	LocalPlaceFile string `json:"localPlaceFile"`
	GameURL        string `json:"gameUrl"`
	Name           string `json:"name,omitempty"`
}

type options struct {
	target      string
	localFile   string
	newPlace    bool
	newName     string
	cmd         string // "", "save", "prefs", "scan", "dump", "install"
	prefsAction string // "" or "apply"
}

func main() {
	root = findRoot()
	placePath = filepath.Join(root, "place.json")
	if v := os.Getenv("START_WAIT_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			waitFor = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("START_POLL_SECS"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			pollEvery = time.Duration(n * float64(time.Second))
		}
	}

	opts := parseArgs(os.Args[1:])
	if err := os.Chdir(root); err != nil {
		fail(exitError, "ERROR: %v", err)
	}

	switch opts.cmd {
	case "save":
		runSave()
		return
	case "prefs":
		runPrefs(opts.prefsAction)
		return
	case "scan":
		runScan()
		return
	case "dump":
		runDump()
		return
	case "install":
		runInstall()
		return
	}

	var p place
	if opts.newPlace {
		p = createNewPlace(opts.newName, opts.localFile)
	} else {
		p = resolvePlace(opts.target, opts.localFile)
	}
	studio := findStudio()
	writeMCP(studio)
	if studio == "" {
		fail(exitError, "ERROR: Roblox Studio was not found.\n"+
			"  macOS: /Applications/RobloxStudio.app\n"+
			"  Windows: %%LOCALAPPDATA%%\\Roblox\\Versions\\*\\RobloxStudioBeta.exe\n"+
			"Run this on the machine that has Studio installed (or from WSL on that PC).")
	}

	bootStudio(studio, p)
	scripts := waitUntilSyncReady()
	printReady(p, scripts)
}

func bootStudio(studio string, p place) {
	m := ensureProjectSyncLayout()
	info("Sync map (%s) — edit this file for a different folder layout:", syncManifestName)
	for _, r := range m.Roots {
		info("  %s  ↔  %s", r.Instance, r.Disk)
	}
	if dump := dumpDir(); dump != "" {
		_ = os.MkdirAll(dump, 0o755)
		if s := inspectRbxStorage(filepath.Join(dump, "rbx-storage.txt")); s != "" {
			info("Wrote rbx-storage.db inspect → %s", filepath.Join(dump, "rbx-storage.txt"))
		}
	}
	_, pluginChanged := installRobuildPlugin()
	running := studioRunning(studio)
	needPrefs := studioPrefsNeedApply()
	needPersist := persistNeedsWrite(p)
	if running && (needPrefs || pluginChanged || needPersist) {
		info("Restarting Roblox Studio so Script Sync prefs, plugin, and resume records apply.")
		bestEffortStudioSave()
		if err := quitStudio(); err != nil {
			warn("%v", err)
			warn("Quit Studio, then re-run robld.")
		} else {
			running = false
			time.Sleep(time.Second)
		}
	}
	if running {
		info("Studio is already running — leaving that window. Confirm the right place is open.")
		reportScriptSyncPrefsBrief()
		return
	}
	applyScriptSyncPrefsIfNeeded()
	if err := applyFileSyncPersistence(p); err != nil {
		warn("Could not write Script Sync resume records yet: %v", err)
	} else {
		info("Wrote Script Sync resume records into Studio preferences (this machine).")
	}
	launchStudio(studio, p)
}

func studioPrefsNeedApply() bool {
	path, _ := findGlobalSettings()
	if path == "" {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	vals, err := readStudioProps(raw)
	if err != nil {
		return true
	}
	for _, pref := range allPatchPrefs() {
		cur, ok := vals[pref.Name]
		if !ok || normalizePref(cur) != normalizePref(pref.Want) {
			return true
		}
	}
	return false
}

func findRoot() string {
	wd, err := os.Getwd()
	if err == nil && hasProjectMarker(wd) {
		return wd
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if hasProjectMarker(dir) {
			return dir
		}
	}
	if wd != "" {
		return wd
	}
	return "."
}

func hasProjectMarker(dir string) bool {
	for _, name := range []string{"place.json", "AGENTS.md", "robuild-sync.json", "cli/go.mod"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func usage() string {
	return "Usage: robld [place-id-or-url]\n" +
		"       robld --new [\"Game Name\"]\n" +
		"       robld --file place.rbxlx\n" +
		"       robld --install\n" +
		"       robld save\n" +
		"       robld prefs\n" +
		"       robld prefs apply\n" +
		"       robld scan\n" +
		"       robld dump\n\n" +
		"Re-run until READY. Stateful: ids are stored in place.json.\n" +
		"--install: write the robld skill into this folder for Claude, Grok, Codex, Cursor.\n" +
		"save (macOS): focus Studio and Cmd+S so place.rbxlx updates. No restart.\n" +
		"prefs: show this machine's Script Sync Studio Settings. Main robld also writes them.\n" +
		"dump: copy Studio settings/logs/sync clues into robuild-dump/ for the agent to read.\n" +
		"Exit 0 = ready, 1 = error, 2 = waiting on Script Sync in Studio.\n"
}

func parseArgs(args []string) options {
	var opts options
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Print(usage())
			os.Exit(0)
		case arg == "--":
			continue
		case arg == "--install":
			opts.cmd = "install"
		case arg == "--new":
			opts.newPlace = true
		case strings.HasPrefix(arg, "--new="):
			opts.newPlace = true
			opts.newName = strings.TrimPrefix(arg, "--new=")
		case arg == "--file" || arg == "-f":
			i++
			if i >= len(args) {
				fail(exitError, "--file needs a path")
			}
			opts.localFile = args[i]
		case strings.HasPrefix(arg, "--file="):
			opts.localFile = strings.TrimPrefix(arg, "--file=")
		case strings.HasPrefix(arg, "-"):
			fail(exitError, "Unknown flag: %s\nTry robld --help", arg)
		default:
			rest = append(rest, arg)
		}
	}
	if len(rest) > 0 && !opts.newPlace {
		switch rest[0] {
		case "install":
			opts.cmd = "install"
			rest = rest[1:]
			if len(rest) > 0 {
				fail(exitError, "ERROR: robld --install takes no extra arguments")
			}
			return opts
		case "save":
			opts.cmd = "save"
			rest = rest[1:]
			if len(rest) > 0 {
				fail(exitError, "ERROR: robld save takes no extra arguments")
			}
			return opts
		case "prefs":
			opts.cmd = "prefs"
			rest = rest[1:]
			if len(rest) == 0 {
				return opts
			}
			if rest[0] != "apply" || len(rest) != 1 {
				fail(exitError, "ERROR: unknown prefs usage.\nUse: robld prefs\n     robld prefs apply")
			}
			opts.prefsAction = "apply"
			return opts
		case "scan":
			opts.cmd = "scan"
			rest = rest[1:]
			if len(rest) > 0 {
				fail(exitError, "ERROR: robld scan takes no extra arguments")
			}
			return opts
		case "dump":
			opts.cmd = "dump"
			rest = rest[1:]
			if len(rest) > 0 {
				fail(exitError, "ERROR: robld dump takes no extra arguments")
			}
			return opts
		}
	}
	if opts.cmd == "install" {
		if len(rest) > 0 {
			fail(exitError, "ERROR: robld --install takes no extra arguments")
		}
		return opts
	}
	joined := strings.Join(rest, " ")
	if opts.newPlace {
		if looksLikePlaceRef(joined) {
			fail(exitError, "ERROR: --new does not take a place id or URL.\nUse robld --new [\"Game Name\"]")
		}
		if opts.newName == "" {
			opts.newName = joined
		}
	} else {
		opts.target = joined
	}
	return opts
}

func looksLikePlaceRef(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	p := parseTarget(s)
	return p.PlaceID != 0 || p.UniverseID != 0 || p.GameURL != ""
}

func parseTarget(raw string) place {
	text := strings.Trim(strings.TrimSpace(raw), `"'`)
	var out place
	if text == "" {
		return out
	}
	lower := strings.ToLower(text)
	if strings.HasSuffix(lower, ".rbxl") || strings.HasSuffix(lower, ".rbxlx") {
		out.LocalPlaceFile = text
		return out
	}
	if st, err := os.Stat(text); err == nil && !st.IsDir() {
		out.LocalPlaceFile = text
		return out
	}
	if reURLIsh.MatchString(lower) {
		out.GameURL = strings.Fields(text)[0]
	}
	if m := reUniverseQuery.FindStringSubmatch(text); len(m) == 2 {
		out.UniverseID = atoi64(m[1])
	}
	if m := rePlaceQuery.FindStringSubmatch(text); len(m) == 2 {
		out.PlaceID = atoi64(m[1])
	}
	if m := reCreateDash.FindStringSubmatch(text); len(m) >= 2 {
		out.UniverseID = atoi64(m[1])
		if len(m) >= 3 && m[2] != "" {
			out.PlaceID = atoi64(m[2])
		}
	}
	if m := reGames.FindStringSubmatch(text); len(m) == 2 {
		out.PlaceID = atoi64(m[1])
	}
	if m := reExperiences.FindStringSubmatch(text); len(m) == 2 && out.PlaceID == 0 {
		out.UniverseID = atoi64(m[1])
	}
	if out.PlaceID == 0 && out.UniverseID == 0 {
		if m := reLongID.FindStringSubmatch(text); len(m) == 2 {
			out.PlaceID = atoi64(m[1])
		} else if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			out.PlaceID = n
		}
	}
	return out
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// repoRel stores paths as slash-separated relatives so place.json works on macOS and Windows.
func repoRel(abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

func absFromRepo(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func studioPathArg(abs string) string {
	if isWSL() && strings.HasPrefix(abs, "/") {
		return posixToWin(abs)
	}
	return abs
}

func loadPlace() place {
	b, err := os.ReadFile(placePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return place{}
		}
		fail(exitError, "ERROR: reading %s: %v", placePath, err)
	}
	var p place
	if err := json.Unmarshal(b, &p); err != nil {
		fail(exitError, "Invalid JSON in %s", placePath)
	}
	return p
}

func savePlace(p place) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fail(exitError, "ERROR: %v", err)
	}
	if err := os.WriteFile(placePath, append(b, '\n'), 0o644); err != nil {
		fail(exitError, "ERROR: writing %s: %v", placePath, err)
	}
}

func resolvePlace(cliTarget, localFile string) place {
	data := loadPlace()
	if localFile != "" {
		data.LocalPlaceFile = localFile
	}
	if cliTarget != "" {
		parsed := parseTarget(cliTarget)
		if parsed.LocalPlaceFile != "" {
			data.LocalPlaceFile = parsed.LocalPlaceFile
		}
		if parsed.GameURL != "" {
			data.GameURL = parsed.GameURL
		}
		if parsed.PlaceID != 0 {
			data.PlaceID = parsed.PlaceID
		}
		if parsed.UniverseID != 0 {
			data.UniverseID = parsed.UniverseID
		}
	}

	if local := strings.TrimSpace(data.LocalPlaceFile); local != "" {
		path := local
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			fail(exitError, "ERROR: %v", err)
		}
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			fail(exitError, "ERROR: local place file not found:\n  %s", abs)
		}
		data.LocalPlaceFile = repoRel(abs)
		savePlace(data)
		return data
	}

	if data.PlaceID != 0 && data.UniverseID == 0 {
		info("Looking up universe for place %d…", data.PlaceID)
		uid, err := universeForPlace(data.PlaceID)
		if err != nil {
			fail(exitError, "ERROR: could not resolve place/universe from Roblox.\n  %v\nPass a roblox.com/games/… URL, or both placeId and universeId.", err)
		}
		data.UniverseID = uid
	} else if data.UniverseID != 0 && data.PlaceID == 0 {
		info("Looking up root place for universe %d…", data.UniverseID)
		pid, err := rootPlaceForUniverse(data.UniverseID)
		if err != nil {
			fail(exitError, "ERROR: could not resolve place/universe from Roblox.\n  %v\nPass a roblox.com/games/… URL, or both placeId and universeId.", err)
		}
		data.PlaceID = pid
	}
	savePlace(data)
	if data.PlaceID == 0 || data.UniverseID == 0 {
		fmt.Print("NEED_PLACE: pass a place id or game URL, or create a local place.\n" +
			"  robld --new\n" +
			"  robld --new \"My Game\"\n" +
			"  robld 123456789\n" +
			"  robld https://www.roblox.com/games/123456789/My-Game\n" +
			"  robld --file place.rbxlx\n")
		os.Exit(exitError)
	}
	return data
}

func httpJSON(url string, dest any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return json.Unmarshal(body, dest)
}

func universeForPlace(placeID int64) (int64, error) {
	var data struct {
		UniverseID int64 `json:"universeId"`
	}
	if err := httpJSON(fmt.Sprintf("https://apis.roblox.com/universes/v1/places/%d/universe", placeID), &data); err != nil {
		return 0, err
	}
	if data.UniverseID == 0 {
		return 0, fmt.Errorf("no universeId for place %d", placeID)
	}
	return data.UniverseID, nil
}

func rootPlaceForUniverse(universeID int64) (int64, error) {
	var data struct {
		Data []struct {
			RootPlaceID int64 `json:"rootPlaceId"`
		} `json:"data"`
	}
	if err := httpJSON(fmt.Sprintf("https://games.roblox.com/v1/games?universeIds=%d", universeID), &data); err != nil {
		return 0, err
	}
	if len(data.Data) == 0 || data.Data[0].RootPlaceID == 0 {
		return 0, fmt.Errorf("no rootPlaceId for universe %d", universeID)
	}
	return data.Data[0].RootPlaceID, nil
}

func isScript(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".luau") || strings.HasSuffix(lower, ".lua")
}

func listScripts() []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if path != root && (skipDirs[base] || strings.HasPrefix(base, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if isScript(d.Name()) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func gitScriptReport() string {
	inside := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	if err := inside.Run(); err != nil {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "-u").Output()
	if err != nil {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		lower := strings.ToLower(path)
		if isScript(path) || strings.HasSuffix(lower, ".rbxlx") || strings.HasSuffix(lower, ".rbxl") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "Git: Luau and place files match HEAD (or are ignored)."
	}
	var newN, modN int
	for _, ln := range lines {
		if strings.HasPrefix(ln, "??") {
			newN++
		} else {
			modN++
		}
	}
	var parts []string
	if newN > 0 {
		parts = append(parts, fmt.Sprintf("%d new game file(s) — review and commit when you want.", newN))
	}
	if modN > 0 {
		parts = append(parts, fmt.Sprintf("%d game file(s) differ from git. Script Sync conflict: Keep Disk for this repo, Keep Studio for the place. World edits need Studio Save into place.rbxlx.", modN))
	}
	n := len(lines)
	if n > 12 {
		n = 12
	}
	sample := make([]string, n)
	for i := 0; i < n; i++ {
		sample[i] = "  " + lines[i]
	}
	extra := ""
	if len(lines) > 12 {
		extra = fmt.Sprintf("\n  … %d more", len(lines)-12)
	}
	return strings.Join(parts, "\n") + "\n" + strings.Join(sample, "\n") + extra
}

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}

func winToPosix(path string) string {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if len(path) >= 2 && path[1] == ':' {
		return "/mnt/" + strings.ToLower(string(path[0])) + strings.ReplaceAll(path[2:], `\`, "/")
	}
	return path
}

func posixToWin(path string) string {
	if strings.HasPrefix(path, "/mnt/") && len(path) > 6 && path[6] == '/' {
		return strings.ToUpper(string(path[5])) + ":" + strings.ReplaceAll(path[6:], "/", `\`)
	}
	return path
}

func findMacStudio() string {
	p := "/Applications/RobloxStudio.app/Contents/MacOS/RobloxStudio"
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

func windowsLocalAppData() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		if _, err := os.Stat(local); err == nil {
			return local
		}
		if posix := winToPosix(local); posix != local {
			if _, err := os.Stat(posix); err == nil {
				return posix
			}
		}
	}
	if runtime.GOOS == "windows" && !isWSL() {
		return os.Getenv("LOCALAPPDATA")
	}
	if !isWSL() {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "cmd.exe", "/c", "echo %LOCALAPPDATA%").Output()
	if err != nil {
		return ""
	}
	return winToPosix(strings.TrimSpace(string(out)))
}

func findWindowsStudio() string {
	local := windowsLocalAppData()
	if local == "" {
		return ""
	}
	versions := filepath.Join(local, "Roblox", "Versions")
	matches, _ := filepath.Glob(filepath.Join(versions, "*", "RobloxStudioBeta.exe"))
	var newest string
	var newestTime time.Time
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		if newest == "" || st.ModTime().After(newestTime) {
			newest, newestTime = m, st.ModTime()
		}
	}
	return newest
}

func findStudio() string {
	if runtime.GOOS == "darwin" {
		return findMacStudio()
	}
	if s := findWindowsStudio(); s != "" {
		return s
	}
	return findMacStudio()
}

func studioRunning(studio string) bool {
	name := filepath.Base(studio)
	if runtime.GOOS == "darwin" {
		return exec.Command("pgrep", "-f", "RobloxStudio").Run() == nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tasklist.exe", "/FI", "IMAGENAME eq "+name).CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, strings.ToLower(name)) && !strings.Contains(s, "no tasks")
}

func launchStudio(studio string, p place) {
	var args []string
	if local := canonicalPlacePath(p); local != "" {
		fileArg := studioPathArg(local)
		args = []string{"--task", "EditFile", "--localPlaceFile", fileArg}
		info("Opening local place: %s", local)
		info("Save in Studio (Cmd+S / Ctrl+S) so world changes write back into git.")
	} else {
		args = []string{
			"--task", "EditPlace",
			"--placeId", strconv.FormatInt(p.PlaceID, 10),
			"--universeId", strconv.FormatInt(p.UniverseID, 10),
		}
		info("Opening place %d (universe %d)", p.PlaceID, p.UniverseID)
	}
	cmd := exec.Command(studio, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = detachSysProcAttr()
	if err := cmd.Start(); err != nil {
		fail(exitError, "ERROR: Roblox Studio could not be started.\n  binary: %s\n  %v", studio, err)
	}
	_ = cmd.Process.Release()
	info("Launched %s", studio)
}

func writeMCP(studio string) {
	_ = os.MkdirAll(filepath.Join(root, ".grok"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, ".codex"), 0o755)
	var mcpJSON []byte
	var toml string
	if runtime.GOOS == "darwin" && (studio == "" || strings.Contains(studio, "RobloxStudio.app")) {
		cmd := "/Applications/RobloxStudio.app/Contents/MacOS/StudioMCP"
		mcpJSON, _ = json.MarshalIndent(map[string]any{
			"mcpServers": map[string]any{
				"Roblox_Studio": map[string]any{"command": cmd},
			},
		}, "", "  ")
		toml = fmt.Sprintf("[mcp_servers.Roblox_Studio]\ncommand = %q\n", cmd)
	} else {
		mcpJSON, _ = json.MarshalIndent(map[string]any{
			"mcpServers": map[string]any{
				"Roblox_Studio": map[string]any{
					"command": "cmd.exe",
					"args":    []string{"/c", `%LOCALAPPDATA%\Roblox\mcp.bat`},
				},
			},
		}, "", "  ")
		toml = "[mcp_servers.Roblox_Studio]\ncommand = \"cmd.exe\"\nargs = [\"/c\", \"%LOCALAPPDATA%\\\\Roblox\\\\mcp.bat\"]\n"
	}
	_ = os.WriteFile(filepath.Join(root, ".mcp.json"), append(mcpJSON, '\n'), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".grok", "config.toml"), []byte(toml), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte(toml), 0o644)
	info("Wrote MCP config: .mcp.json, .grok/config.toml, .codex/config.toml")
}

func printReady(p place, scripts []string) {
	fmt.Println()
	fmt.Println(color("\033[32;1m", "READY") + color("\033[32m", ": Script Sync + MCP"))
	if p.Name != "" {
		fmt.Printf("  name: %s\n", p.Name)
	}
	if strings.TrimSpace(p.LocalPlaceFile) != "" {
		fmt.Printf("  place file: %s\n", p.LocalPlaceFile)
	}
	if p.PlaceID != 0 {
		fmt.Printf("  place %d  universe %d\n", p.PlaceID, p.UniverseID)
	}
	fmt.Printf("  %d Luau file(s) in %s\n", len(scripts), root)
	fmt.Printf("  sync map: %s\n", syncManifestName)
	fmt.Println("  MCP config written. Keep Studio open. Open this folder in the agent.")
	if report := gitScriptReport(); report != "" {
		fmt.Println(report)
	}
}

func useColor() bool {
	v := os.Getenv("FORCE_COLOR")
	if v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func color(code, text string) string {
	if !useColor() {
		return text
	}
	return code + text + "\033[0m"
}

func info(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if useColor() {
		fmt.Fprintln(os.Stderr, "\033[33m"+msg+"\033[0m")
	} else {
		fmt.Fprintln(os.Stderr, msg)
	}
}

func fail(code int, format string, args ...any) {
	_ = os.Stdout.Sync()
	msg := fmt.Sprintf(format, args...)
	if useColor() {
		fmt.Fprintln(os.Stderr, "\033[31m"+msg+"\033[0m")
	} else {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(code)
}
