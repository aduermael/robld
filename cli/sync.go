package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const syncManifestName = "robuild-sync.json"

type syncRoot struct {
	Instance string `json:"instance"`
	Disk     string `json:"disk"`
}

type syncManifest struct {
	Version   int        `json:"version"`
	Generated bool       `json:"generated,omitempty"`
	Roots     []syncRoot `json:"roots"`
}

func syncManifestPath() string {
	return filepath.Join(root, syncManifestName)
}

func loadSyncManifest() syncManifest {
	b, err := os.ReadFile(syncManifestPath())
	if err != nil {
		return syncManifest{}
	}
	var m syncManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return syncManifest{}
	}
	return m
}

func saveSyncManifest(m syncManifest) {
	if m.Version == 0 {
		m.Version = 1
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(syncManifestPath(), append(b, '\n'), 0o644); err != nil {
		warn("Could not write %s: %v", syncManifestName, err)
	}
}

func resolveSyncManifest() syncManifest {
	existing := loadSyncManifest()
	if len(existing.Roots) > 0 {
		return migrateLegacyGreenfieldMap(existing)
	}
	if discovered := discoverSyncRootsFromDisk(); len(discovered) > 0 {
		m := syncManifest{Version: 1, Generated: true, Roots: discovered}
		saveSyncManifest(m)
		info("Wrote %s from existing Luau folders (%d root(s)).", syncManifestName, len(discovered))
		return m
	}
	m := greenfieldManifest()
	seedGreenfield(m)
	saveSyncManifest(m)
	info("Wrote %s mapping src/{server,shared,client} to Studio services (auto-resume binds services).", syncManifestName)
	return m
}

func greenfieldManifest() syncManifest {
	return syncManifest{
		Version:   1,
		Generated: true,
		Roots: []syncRoot{
			{Instance: "ServerScriptService", Disk: "src/server"},
			{Instance: "ReplicatedStorage", Disk: "src/shared"},
			{Instance: "StarterPlayer.StarterPlayerScripts", Disk: "src/client"},
		},
	}
}

// Studio auto-resume (File_Sync_Persistence_Record_V1) actually starts for
// service roots. Nested Folder records we wrote did not. Rewrite the original
// generated map so persist + plugin look at the same instances.
func migrateLegacyGreenfieldMap(m syncManifest) syncManifest {
	if len(m.Roots) != 3 {
		return m
	}
	old := map[string]string{
		"ServerScriptService.src":                "src/server",
		"ReplicatedStorage.shared":               "src/shared",
		"StarterPlayer.StarterPlayerScripts.src": "src/client",
	}
	for _, r := range m.Roots {
		want, ok := old[r.Instance]
		if !ok || filepath.ToSlash(r.Disk) != want {
			return m
		}
	}
	next := greenfieldManifest()
	saveSyncManifest(next)
	info("Updated %s to sync service roots (Studio auto-resume ignores nested Folder records).", syncManifestName)
	return next
}

func seedGreenfield(m syncManifest) {
	files := map[string]string{
		"src/server/main.server.luau": "print(\"robld server\")\n",
		"src/shared/Hello.luau":       "return {}\n",
		"src/client/main.client.luau": "print(\"robld client\")\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			warn("Could not create %s: %v", rel, err)
			continue
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			warn("Could not seed %s: %v", rel, err)
		}
	}
	for _, r := range m.Roots {
		_ = os.MkdirAll(absSyncPath(r.Disk), 0o755)
	}
}

func discoverSyncRootsFromDisk() []syncRoot {
	files := listScripts()
	seen := map[string]bool{}
	var roots []syncRoot
	for _, f := range files {
		rel := repoRel(f)
		disk := diskRootForScript(rel)
		if disk == "" || seen[disk] {
			continue
		}
		seen[disk] = true
		roots = append(roots, syncRoot{
			Instance: inferInstancePath(disk),
			Disk:     disk,
		})
	}
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Disk == roots[j].Disk {
			return roots[i].Instance < roots[j].Instance
		}
		return roots[i].Disk < roots[j].Disk
	})
	return roots
}

func diskRootForScript(rel string) string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	if parts[0] == "src" && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

var wellKnownDiskToInstance = map[string]string{
	"ServerScriptService":     "ServerScriptService",
	"ReplicatedStorage":       "ReplicatedStorage",
	"ReplicatedFirst":         "ReplicatedFirst",
	"ServerStorage":           "ServerStorage",
	"StarterGui":              "StarterGui",
	"StarterPack":             "StarterPack",
	"Workspace":               "Workspace",
	"StarterPlayerScripts":    "StarterPlayer.StarterPlayerScripts",
	"StarterCharacterScripts": "StarterPlayer.StarterCharacterScripts",
	"src/server":              "ServerScriptService",
	"src/shared":              "ReplicatedStorage",
	"src/client":              "StarterPlayer.StarterPlayerScripts",
	"server":                  "ServerScriptService",
	"shared":                  "ReplicatedStorage",
	"client":                  "StarterPlayer.StarterPlayerScripts",
}

func inferInstancePath(disk string) string {
	disk = filepath.ToSlash(strings.TrimSpace(disk))
	if inst, ok := wellKnownDiskToInstance[disk]; ok {
		return inst
	}
	first, rest, _ := strings.Cut(disk, "/")
	if inst, ok := wellKnownDiskToInstance[first]; ok {
		if rest == "" || rest == first {
			return inst
		}
		return inst + "." + strings.ReplaceAll(rest, "/", ".")
	}
	return "ServerScriptService." + strings.ReplaceAll(disk, "/", ".")
}

func absSyncPath(disk string) string {
	disk = strings.TrimSpace(disk)
	if disk == "" {
		return ""
	}
	p := filepath.FromSlash(disk)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func ensureProjectSyncLayout() syncManifest {
	return resolveSyncManifest()
}

func syncInstructions() string {
	m := loadSyncManifest()
	if len(m.Roots) == 0 {
		m = resolveSyncManifest()
	}
	var b strings.Builder
	b.WriteString("In Roblox Studio, Sync to… each folder (Keep Disk if asked):\n")
	for _, r := range m.Roots {
		fmt.Fprintf(&b, "  %s  →  %s\n", r.Instance, absSyncPath(r.Disk))
	}
	b.WriteString("Then run robld again. Edit ")
	b.WriteString(syncManifestName)
	b.WriteString(" if this project uses different folders.\n")
	return b.String()
}
