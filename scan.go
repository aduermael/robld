package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	scanMaxFileBytes = 2 << 20
	scanMaxHits      = 80
	scanMaxFiles     = 400
)

var scanNameHints = regexp.MustCompile(`(?i)(globalsettings|globalbasicsettings|placeidestate|scriptsync|filesync|clientsettings|rbx-storage|\.db$|sqlite)`)

var scanContentRe = regexp.MustCompile(`(?i)(AutoResumeSyncOnPlaceOpen|ActionOnAutoResumeSync|ActionOnStopSync|DefaultScriptSyncFileType|InstanceFileSync|ScriptSync|KeepLocalFiles|KeepLocal|InternalSync|LiveSync|placeIDEState|ROBUILD_JSON)`)

func runScan() {
	fmt.Println("SCAN: looking for Studio Script Sync state on this machine")
	fmt.Println("Prefs (auto-resume / keep local) live in GlobalSettings_13.xml.")
	fmt.Println("Per-place Sync-to-folder bindings are not in place.rbxlx — this hunt is for that store.")
	fmt.Println()

	dirs := scanRoots()
	fmt.Println("Candidate directories")
	for _, d := range dirs {
		st, err := os.Stat(d)
		switch {
		case err != nil:
			fmt.Printf("  missing  %s\n", d)
		case st.IsDir():
			fmt.Printf("  dir      %s\n", d)
		default:
			fmt.Printf("  file     %s\n", d)
		}
	}

	needles := extraScanNeedles()
	fmt.Println()
	fmt.Println("Path needles (this repo)")
	if len(needles) == 0 {
		fmt.Println("  (none)")
	}
	for _, n := range needles {
		fmt.Printf("  %s\n", n)
	}

	fmt.Println()
	fmt.Println("Hits")
	hits := 0
	files := 0
	seen := map[string]bool{}
	for _, dir := range dirs {
		st, err := os.Stat(dir)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			if reportScanFile(dir, needles, &hits) {
				files++
			}
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || hits >= scanMaxHits || files >= scanMaxFiles {
				if hits >= scanMaxHits || files >= scanMaxFiles {
					return fs.SkipAll
				}
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if skipScanDir(name) {
					return fs.SkipDir
				}
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			if seen[abs] {
				return nil
			}
			seen[abs] = true
			if reportScanFile(abs, needles, &hits) {
				files++
			}
			return nil
		})
	}
	if hits == 0 {
		fmt.Println("  (no keyword hits in readable files)")
	}
	fmt.Println()
	fmt.Printf("Scanned %d files, %d hits (cap %d files / %d hits).\n", files, hits, scanMaxFiles, scanMaxHits)
	fmt.Println("Paste this output back if you want help locating the per-place map.")
}

func skipScanDir(name string) bool {
	switch strings.ToLower(name) {
	case "versions", "downloads", "http", "logs", "crashes", "gpuinfo", "shadercache",
		"node_modules", ".git", "cache", "caches", "webkit", "networkcache", "blobs":
		return true
	}
	return false
}

func scanRoots() []string {
	var dirs []string
	add := func(p string) {
		if p == "" {
			return
		}
		dirs = append(dirs, p)
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		add(filepath.Join(home, "Library", "Roblox"))
		add(filepath.Join(home, "Library", "Application Support", "Roblox"))
		add(filepath.Join(home, "Library", "Application Support", "RobloxStudio"))
		add(filepath.Join(home, "Library", "Logs", "Roblox"))
		add(filepath.Join(home, "Library", "Caches", "com.Roblox.RobloxStudio"))
		add(filepath.Join(home, "Library", "Caches", "com.roblox.RobloxStudio"))
		for _, pat := range []string{
			filepath.Join(home, "Library", "Preferences", "com.roblox*"),
			filepath.Join(home, "Library", "Preferences", "com.Roblox*"),
		} {
			matches, _ := filepath.Glob(pat)
			for _, m := range matches {
				add(m)
			}
		}
		add(filepath.Join(home, "Documents", "Roblox"))
		add(filepath.Join(home, "Documents", "ROBLOX"))
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			add(filepath.Join(local, "Roblox"))
			add(filepath.Join(local, "Roblox", "placeIDEState"))
			add(filepath.Join(local, "Roblox", "ClientSettings"))
			add(filepath.Join(local, "Roblox", "logs"))
		}
	} else if home != "" {
		add(filepath.Join(home, "Library", "Roblox", "placeIDEState"))
		add(filepath.Join(home, "Library", "Roblox", "ClientSettings"))
	}
	return dirs
}

func extraScanNeedles() []string {
	var out []string
	if root != "" {
		out = append(out, root)
		out = append(out, filepath.ToSlash(root))
	}
	p := loadPlace()
	if local := absFromRepo(p.LocalPlaceFile); local != "" {
		out = append(out, local)
		out = append(out, filepath.ToSlash(local))
	}
	seen := map[string]bool{}
	var uniq []string
	for _, s := range out {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		uniq = append(uniq, s)
	}
	return uniq
}

func reportScanFile(path string, needles []string, hits *int) bool {
	lower := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))
	base := filepath.Base(path)
	interestingName := scanNameHints.MatchString(base) ||
		strings.Contains(lower, "placeidestate") ||
		strings.HasPrefix(strings.ToLower(base), "com.roblox")
	switch ext {
	case ".xml", ".json", ".txt", ".log", ".plist", ".config", ".ini", ".sqlite", ".db":
		interestingName = true
	case ".exe", ".dll", ".so", ".dylib", ".pak", ".bin", ".png", ".jpg", ".rbxl", ".rbxlx", ".zip", ".dmg":
		if !scanNameHints.MatchString(base) {
			return false
		}
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	if st.Size() > scanMaxFileBytes && !scanNameHints.MatchString(base) {
		return false
	}

	if interestingName && (ext == ".sqlite" || ext == ".db" || !isProbablyTextExt(ext)) {
		fmt.Printf("  file  %s  (%d bytes)\n", path, st.Size())
		*hits++
		return true
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if st.Size() > scanMaxFileBytes {
		raw = raw[:scanMaxFileBytes]
	}
	if !utf8.Valid(raw) && !isProbablyTextExt(ext) {
		return false
	}
	text := string(raw)
	var reasons []string
	if scanContentRe.MatchString(text) {
		reasons = append(reasons, uniqueMatchNames(scanContentRe, text)...)
	}
	for _, n := range needles {
		if n != "" && strings.Contains(text, n) {
			reasons = append(reasons, "path:"+n)
		}
	}
	if len(reasons) == 0 {
		if interestingName && (ext == ".xml" || ext == ".json" || ext == ".plist") {
			fmt.Printf("  file  %s  (%d bytes, no keyword hits)\n", path, st.Size())
			return true
		}
		return false
	}
	fmt.Printf("  HIT   %s  (%d bytes)\n", path, st.Size())
	for i, r := range reasons {
		if i >= 8 {
			fmt.Printf("         … %d more\n", len(reasons)-8)
			break
		}
		fmt.Printf("         %s\n", r)
	}
	*hits++
	return true
}

func isProbablyTextExt(ext string) bool {
	switch ext {
	case ".xml", ".json", ".txt", ".log", ".plist", ".config", ".ini", ".csv", ".md", ".lua", ".luau":
		return true
	}
	return ext == ""
}

func uniqueMatchNames(re *regexp.Regexp, text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllString(text, 40) {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}
