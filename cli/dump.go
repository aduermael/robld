package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const dumpFolderName = "robuild-dump"

var dumpLogRe = regexp.MustCompile(`(?i)(script\s*sync|scriptsync|instancefilesync|auto.?resume|did not resume|sync to|ROBUILD|KeepLocal|placeIDEState|File_Sync|synced hierarch|user_robld|StartSync|ExternalMCP|StudioMCP|AssistantSettings)`)

func dumpDir() string {
	if v := strings.TrimSpace(os.Getenv("ROBUILD_DUMP_DIR")); v != "" {
		return v
	}
	return filepath.Join(root, dumpFolderName)
}

func runDump() {
	dir := dumpDir()
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		fail(exitError, "ERROR: could not clear %s: %v", dir, err)
	}
	for _, sub := range []string{"copies", "logs", "project"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			fail(exitError, "ERROR: %v", err)
		}
	}

	p := loadPlace()
	studio := findStudio()
	running := studio != "" && studioRunning(studio)
	var report strings.Builder
	w := func(format string, args ...any) {
		fmt.Fprintf(&report, format+"\n", args...)
	}

	w("robld dump  %s", time.Now().Format(time.RFC3339))
	w("Paste this folder (especially REPORT.txt) back into the agent.")
	w("")
	w("== machine ==")
	w("GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	if home, err := os.UserHomeDir(); err == nil {
		w("home=%s", home)
	}
	w("studio=%s", emptyDash(studio))
	w("studioRunning=%v", running)
	w("project=%s", root)

	w("")
	w("== place ==")
	w("place.json=%s", placePath)
	w("  name=%s placeId=%d universeId=%d", p.Name, p.PlaceID, p.UniverseID)
	w("  localPlaceFile=%s", p.LocalPlaceFile)
	if local := absFromRepo(p.LocalPlaceFile); local != "" {
		if st, err := os.Stat(local); err == nil {
			w("  abs=%s mtime=%s size=%d", local, st.ModTime().Format(time.RFC3339), st.Size())
		} else {
			w("  abs=%s (missing)", local)
		}
	}
	lock := absFromRepo(p.LocalPlaceFile) + ".lock"
	if b, err := os.ReadFile(lock); err == nil {
		w("  lock=%s", strings.TrimSpace(string(b)))
	}

	w("")
	w("== Script Sync prefs (GlobalSettings) ==")
	gs, extras := findGlobalSettings()
	if gs == "" {
		w("GlobalSettings_13.xml not found. Looked in:")
		for _, d := range studioSettingsDirs() {
			w("  %s", d)
		}
	} else {
		w("file=%s", gs)
		for _, e := range extras {
			if e != gs {
				w("also=%s", e)
			}
		}
		if raw, err := os.ReadFile(gs); err == nil {
			if vals, err := readStudioProps(raw); err == nil {
				for _, pref := range scriptSyncPrefs {
					cur, ok := vals[pref.Name]
					if !ok {
						w("  %s  NOT IN FILE  want %s", pref.Name, formatPref(pref, pref.Want))
					} else {
						status := "ok"
						if normalizePref(cur) != normalizePref(pref.Want) {
							status = "WANT " + formatPref(pref, pref.Want)
						}
						w("  %s  %s  %s", pref.Name, formatPref(pref, cur), status)
					}
				}
			} else {
				w("parse error: %v", err)
			}
		}
	}

	w("")
	writeMCPSettingReport(w)

	w("")
	writeMCPProbeReport(w, probeDefaultMCP())

	w("")
	w("== intended sync map (robuild-sync.json) ==")
	m := loadSyncManifest()
	for _, r := range m.Roots {
		abs := absSyncPath(r.Disk)
		st, err := os.Stat(abs)
		switch {
		case err != nil:
			w("  %s -> %s  (missing)", r.Instance, r.Disk)
		case st.IsDir():
			w("  %s -> %s  dir", r.Instance, abs)
		default:
			w("  %s -> %s  file", r.Instance, abs)
		}
	}

	dumpPersistReport(w, p)

	w("")
	w("== placeIDEState hashes for this place ==")
	for _, key := range placePathKeys(p) {
		w("  key %q", key)
		w("    %s", placeIDEStateName(key))
		w("    %s", placeIDEStateDebuggerName(key))
	}

	w("")
	w("== candidate dirs ==")
	for _, d := range scanRoots() {
		st, err := os.Stat(d)
		switch {
		case err != nil:
			w("  missing  %s", d)
		case st.IsDir():
			w("  dir      %s", d)
		default:
			w("  file     %s", d)
		}
	}

	needles := extraScanNeedles()
	w("")
	w("== directory listings ==")
	writeDirListings(dir, &report)

	w("")
	w("== copied files ==")
	manifest := copyDumpFiles(dir, needles, &report)

	w("")
	w("== log excerpts ==")
	nLog := copyLogExcerpts(dir, &report)
	if nLog == 0 {
		w("(none)")
	}

	copyProjectSidecars(dir, p)

	w("")
	w("== rbx-storage.db ==")
	storageOut := filepath.Join(dir, "rbx-storage.txt")
	if text := inspectRbxStorage(storageOut); text == "" {
		w("not found (looked for ~/Library/Roblox/rbx-storage.db)")
	} else {
		w("wrote rbx-storage.txt (%d bytes)", len(text))
		lines := strings.Split(text, "\n")
		n := 40
		if len(lines) < n {
			n = len(lines)
		}
		for _, ln := range lines[:n] {
			w("  %s", ln)
		}
		if len(lines) > 40 {
			w("  …")
		}
	}

	w("")
	w("== project tree (depth 3) ==")
	w("%s", projectTree(root, 3))

	w("")
	w("== luau files ==")
	scripts := listScripts()
	w("count=%d", len(scripts))
	for i, s := range scripts {
		if i >= 40 {
			w("  … %d more", len(scripts)-40)
			break
		}
		w("  %s", repoRel(s))
	}

	pluginPath := filepath.Join(studioPluginsDir(), pluginFileName)
	w("")
	w("== robld plugin ==")
	if st, err := os.Stat(pluginPath); err == nil {
		w("installed %s (%d bytes, %s)", pluginPath, st.Size(), st.ModTime().Format(time.RFC3339))
	} else {
		w("not installed at %s", pluginPath)
	}

	mb, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), append(mb, '\n'), 0o644)
	if err := os.WriteFile(filepath.Join(dir, "REPORT.txt"), []byte(report.String()), 0o644); err != nil {
		fail(exitError, "ERROR: writing REPORT.txt: %v", err)
	}

	fmt.Println(color("\033[32;1m", "READY") + color("\033[32m", ": dump written"))
	fmt.Printf("  %s\n", dir)
	fmt.Println("  Read REPORT.txt plus copies/. Leave this folder in the repo so the agent can inspect it.")
	fmt.Println("This is a snapshot of this machine's Studio state, not a place-file patch.")
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func copyDumpFiles(dir string, needles []string, report *strings.Builder) []map[string]any {
	var out []map[string]any
	seen := map[string]bool{}
	n := 0
	const maxFiles = 80

	try := func(path string, force bool) {
		if n >= maxFiles {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			return
		}
		st, err := os.Stat(abs)
		if err != nil || st.IsDir() {
			return
		}
		if !force && !dumpWorthy(abs, st, needles) {
			return
		}
		limit := int64(scanMaxFileBytes)
		if dumpNameHint(abs) || force {
			limit = 32 << 20
		}
		if st.Size() > limit && !force {
			fmt.Fprintf(report, "  skip large %s (%d bytes)\n", abs, st.Size())
			return
		}
		seen[abs] = true
		name := fmt.Sprintf("%02d-%s", n+1, sanitizeDumpName(abs))
		dst := filepath.Join(dir, "copies", name)
		if err := copyFileLimited(abs, dst, limit); err != nil {
			fmt.Fprintf(report, "  copy fail %s: %v\n", abs, err)
			return
		}
		n++
		entry := map[string]any{
			"src":   abs,
			"dst":   "copies/" + name,
			"bytes": st.Size(),
			"mtime": st.ModTime().Format(time.RFC3339),
		}
		out = append(out, entry)
		fmt.Fprintf(report, "  %s\n    <- %s (%d bytes)\n", "copies/"+name, abs, st.Size())
	}

	for _, logPath := range recentStudioLogs(8) {
		try(logPath, true)
	}
	if gs, extras := findGlobalSettings(); gs != "" {
		try(gs, true)
		for _, e := range extras {
			try(e, true)
		}
	}
	for _, d := range studioSettingsDirs() {
		try(filepath.Join(d, "rbx-storage.db"), true)
		try(filepath.Join(d, "rbx-storage.db-wal"), true)
		try(filepath.Join(d, "rbx-storage.db-shm"), true)
		try(filepath.Join(d, "LocalStorage", "appStorage.json"), true)
		try(filepath.Join(d, "ClientSettings", "StudioGuacSettings.json"), true)
		try(filepath.Join(d, "frm.cfg"), true)
		matches, _ := filepath.Glob(filepath.Join(d, "LocalStorage", "*.json"))
		for _, m := range matches {
			try(m, true)
		}
		asst, _ := filepath.Glob(filepath.Join(d, "AssistantSettings", "*.json"))
		for _, m := range asst {
			try(m, true)
		}
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		for _, base := range []string{
			filepath.Join(home, "Documents", "Roblox"),
			filepath.Join(home, "Documents", "ROBLOX"),
		} {
			try(filepath.Join(base, "Plugins", pluginFileName), true)
			matches, _ := filepath.Glob(filepath.Join(base, "*", "InstalledPlugins", "0", "settings.json"))
			for _, m := range matches {
				try(m, true)
			}
		}
	}
	p := loadPlace()
	for _, key := range placePathKeys(p) {
		for _, dirName := range []string{
			placeIDEStateName(key),
			placeIDEStateDebuggerName(key),
			fmt.Sprintf("placeIDEState_%d_DebuggerData.xml", p.PlaceID),
		} {
			for _, rootDir := range scanRoots() {
				try(filepath.Join(rootDir, dirName), true)
				try(filepath.Join(rootDir, "placeIDEState", dirName), true)
			}
		}
	}

	for _, rootDir := range scanRoots() {
		st, err := os.Stat(rootDir)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			try(rootDir, dumpNameHint(rootDir))
			continue
		}
		_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || n >= maxFiles {
				if n >= maxFiles {
					return fs.SkipAll
				}
				return nil
			}
			if d.IsDir() {
				if skipScanDir(d.Name()) {
					return fs.SkipDir
				}
				return nil
			}
			try(path, dumpNameHint(path))
			return nil
		})
	}
	return out
}

func recentStudioLogs(n int) []string {
	files := studioLogFiles()
	sort.Slice(files, func(i, j int) bool {
		si, _ := os.Stat(files[i])
		sj, _ := os.Stat(files[j])
		if si == nil || sj == nil {
			return files[i] > files[j]
		}
		return si.ModTime().After(sj.ModTime())
	})
	if n > 0 && len(files) > n {
		files = files[:n]
	}
	return files
}

func dumpNameHint(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	lower := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))
	return scanNameHints.MatchString(base) ||
		strings.Contains(lower, "placeidestate") ||
		strings.HasPrefix(base, "com.roblox") ||
		strings.Contains(base, "globalsettings") ||
		strings.Contains(base, "globalbasicsettings") ||
		strings.Contains(base, "rbx-storage") ||
		strings.Contains(lower, "assistantsettings") ||
		strings.Contains(base, "assistant-externalmcp") ||
		ext == ".db" || ext == ".sqlite" || ext == ".sqlite3" ||
		strings.HasSuffix(lower, ".db-wal") || strings.HasSuffix(lower, ".db-shm")
}

func dumpWorthy(path string, st os.FileInfo, needles []string) bool {
	if dumpNameHint(path) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !isProbablyTextExt(ext) {
		return false
	}
	if st.Size() > scanMaxFileBytes {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if !utf8.Valid(raw) {
		return false
	}
	text := string(raw)
	if scanContentRe.MatchString(text) || dumpLogRe.MatchString(text) {
		return true
	}
	for _, n := range needles {
		if n != "" && strings.Contains(text, n) {
			return true
		}
	}
	return false
}

func sanitizeDumpName(path string) string {
	s := filepath.Base(path)
	s = strings.Map(func(r rune) rune {
		if r == '.' || r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, s)
	if s == "" {
		return "file"
	}
	return s
}

func copyFileLimited(src, dst string, max int64) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(in, max))
	return err
}

func copyLogExcerpts(dir string, report *strings.Builder) int {
	var logs []string
	for _, rootDir := range scanRoots() {
		lower := strings.ToLower(rootDir)
		if !strings.Contains(lower, "log") {
			continue
		}
		_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".log" || ext == ".txt" {
				logs = append(logs, path)
			}
			return nil
		})
	}
	sort.Slice(logs, func(i, j int) bool {
		si, _ := os.Stat(logs[i])
		sj, _ := os.Stat(logs[j])
		if si == nil || sj == nil {
			return logs[i] < logs[j]
		}
		return si.ModTime().After(sj.ModTime())
	})
	if len(logs) > 8 {
		logs = logs[:8]
	}
	var lines []string
	for _, path := range logs {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if int64(len(raw)) > scanMaxFileBytes {
			raw = raw[len(raw)-scanMaxFileBytes:]
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if dumpLogRe.MatchString(line) || strings.Contains(line, "ROBUILD_JSON") {
				lines = append(lines, filepath.Base(path)+": "+strings.TrimRight(line, "\r"))
			}
		}
	}
	if len(lines) > 400 {
		lines = lines[len(lines)-400:]
	}
	if len(lines) == 0 {
		return 0
	}
	body := strings.Join(lines, "\n") + "\n"
	_ = os.WriteFile(filepath.Join(dir, "logs", "excerpts.txt"), []byte(body), 0o644)
	fmt.Fprintf(report, "  logs/excerpts.txt  (%d lines)\n", len(lines))
	show := lines
	if len(show) > 30 {
		show = show[len(show)-30:]
		fmt.Fprintf(report, "  (last 30 of %d)\n", len(lines))
	}
	for _, ln := range show {
		if len(ln) > 300 {
			ln = ln[:300] + "…"
		}
		fmt.Fprintf(report, "  %s\n", ln)
	}
	return len(lines)
}

func writeDirListings(dir string, report *strings.Builder) {
	var b strings.Builder
	n := 0
	const max = 400
	for _, rootDir := range scanRoots() {
		st, err := os.Stat(rootDir)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "# %s\n", rootDir)
		if !st.IsDir() {
			fmt.Fprintf(&b, "  FILE %8d  %s\n", st.Size(), rootDir)
			fmt.Fprintf(report, "  file %s (%d bytes)\n", rootDir, st.Size())
			continue
		}
		_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || n >= max {
				if n >= max {
					return fs.SkipAll
				}
				return nil
			}
			if d.IsDir() {
				if skipScanDir(d.Name()) {
					return fs.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, err := filepath.Rel(rootDir, path)
			if err != nil {
				rel = path
			}
			fmt.Fprintf(&b, "  %10d  %s  %s\n", info.Size(), info.ModTime().Format(time.RFC3339), rel)
			n++
			return nil
		})
		fmt.Fprintln(&b)
	}
	_ = os.WriteFile(filepath.Join(dir, "listings.txt"), []byte(b.String()), 0o644)
	fmt.Fprintf(report, "  listings.txt  (%d files)\n", n)
}

func copyProjectSidecars(dir string, p place) {
	dst := filepath.Join(dir, "project")
	for _, name := range []string{"place.json", syncManifestName, "place.rbxlx.lock"} {
		src := filepath.Join(root, name)
		if _, err := os.Stat(src); err == nil {
			_ = copyFileLimited(src, filepath.Join(dst, name), scanMaxFileBytes)
		}
	}
	if local := absFromRepo(p.LocalPlaceFile); local != "" {
		lock := local + ".lock"
		if _, err := os.Stat(lock); err == nil {
			_ = copyFileLimited(lock, filepath.Join(dst, filepath.Base(lock)), scanMaxFileBytes)
		}
	}
}

func projectTree(dir string, depth int) string {
	var b strings.Builder
	var walk func(string, int, string)
	walk = func(path string, d int, prefix string) {
		ents, err := os.ReadDir(path)
		if err != nil {
			return
		}
		var names []string
		for _, e := range ents {
			name := e.Name()
			if skipDirs[name] || name == dumpFolderName || strings.HasPrefix(name, ".") {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for i, name := range names {
			last := i == len(names)-1
			branch := "├── "
			next := prefix + "│   "
			if last {
				branch = "└── "
				next = prefix + "    "
			}
			full := filepath.Join(path, name)
			st, err := os.Stat(full)
			mark := ""
			if err == nil && st.IsDir() {
				mark = "/"
			}
			fmt.Fprintf(&b, "%s%s%s%s\n", prefix, branch, name, mark)
			if err == nil && st.IsDir() && d > 1 {
				walk(full, d-1, next)
			}
		}
	}
	fmt.Fprintf(&b, "%s/\n", filepath.Base(dir))
	walk(dir, depth, "")
	return strings.TrimRight(b.String(), "\n")
}
