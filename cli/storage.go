package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func rbxStorageDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(home, "Library", "Roblox", "rbx-storage.db"),
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			candidates = append(candidates, filepath.Join(local, "Roblox", "rbx-storage.db"))
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

const rbxStorageInspectPy = `
import sqlite3, sys
db, out = sys.argv[1], sys.argv[2]
needles = sys.argv[3:]
con = sqlite3.connect(db)
lines = ["db="+db, ""]
cur = con.execute("SELECT name, sql FROM sqlite_master WHERE type IN ('table','index') ORDER BY type, name")
for name, sql in cur.fetchall():
    lines.append(name)
    if sql:
        lines.append(sql)
    if not name or name.startswith("sqlite_"):
        lines.append("")
        continue
    try:
        cols = [r[1] for r in con.execute('PRAGMA table_info("%s")' % name.replace('"','')).fetchall()]
        lines.append("cols: " + ", ".join(cols))
        n = con.execute('SELECT COUNT(*) FROM "%s"' % name.replace('"','')).fetchone()[0]
        lines.append("rows: %s" % n)
        rows = con.execute('SELECT * FROM "%s" LIMIT 30' % name.replace('"','')).fetchall()
        dumped = 0
        for row in rows:
            s = repr(row)
            hit = (not needles) or any(n and n in s for n in needles)
            if hit or dumped < 2:
                lines.append("  " + s[:2000])
                dumped += 1
    except Exception as e:
        lines.append("error: %s" % e)
    lines.append("")
open(out, "w", encoding="utf-8").write("\n".join(lines) + "\n")
`

func inspectRbxStorage(outPath string) string {
	db := rbxStorageDBPath()
	if db == "" {
		return ""
	}
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), "robld-rbx-storage.txt")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return ""
	}
	args := []string{"-c", rbxStorageInspectPy, db, outPath}
	args = append(args, extraScanNeedles()...)
	out, err := exec.Command("python3", args...).CombinedOutput()
	if err != nil {
		schema, err2 := exec.Command("sqlite3", db, ".schema").CombinedOutput()
		if err2 != nil {
			return strings.TrimSpace(fmt.Sprintf("rbx-storage inspect failed (%s): %s", err, strings.TrimSpace(string(out))))
		}
		_ = os.WriteFile(outPath, schema, 0o644)
		return string(schema)
	}
	b, readErr := os.ReadFile(outPath)
	if readErr != nil {
		return string(out)
	}
	return string(b)
}
