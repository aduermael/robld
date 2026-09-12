package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type syncBinding struct {
	ClassName string `json:"className"`
	FilePath  string `json:"filePath"`
	ScriptID  string `json:"scriptId"`
	Status    string `json:"status"`
}

type instInfo struct {
	Class    string
	Name     string
	UniqueID string
	Path     string
}

func uniqueIDToUUID(hex string) string {
	hex = strings.ToLower(strings.TrimSpace(hex))
	if len(hex) != 32 {
		return hex
	}
	return hex[0:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}

// Studio plist keys encode the place path as .Users.foo.bar.place·rbxlx
// (slashes → dots, extension dot → middle dot U+00B7).
func encodePlaceKeyPath(abs string) string {
	s := filepath.ToSlash(abs)
	s = strings.TrimPrefix(s, "/")
	s = strings.ReplaceAll(s, "/", ".")
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[:i] + "·" + s[i+1:]
	}
	return "." + s
}

func persistRecordPrefix(placeAbs, workspaceUUID string) string {
	return "File_Sync_Persistence_Record_V1:" + encodePlaceKeyPath(placeAbs) + ":" + workspaceUUID
}

func parsePlaceInstances(rbxlxPath string) (workspaceUID string, byPath map[string]instInfo, err error) {
	f, err := os.Open(rbxlxPath)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	dec.Strict = false
	byPath = map[string]instInfo{}
	type frame struct {
		class, name, uid string
		path             string
		inProps          bool
		propKind         string // element local name
		propName         string
		buf              strings.Builder
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "Item" {
				fr := &frame{}
				for _, a := range t.Attr {
					if a.Name.Local == "class" {
						fr.class = a.Value
					}
				}
				stack = append(stack, fr)
				continue
			}
			if len(stack) == 0 {
				continue
			}
			cur := stack[len(stack)-1]
			if t.Name.Local == "Properties" {
				cur.inProps = true
				continue
			}
			if !cur.inProps {
				continue
			}
			cur.propKind = t.Name.Local
			cur.propName = ""
			for _, a := range t.Attr {
				if a.Name.Local == "name" {
					cur.propName = a.Value
				}
			}
			cur.buf.Reset()
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			cur := stack[len(stack)-1]
			if cur.inProps && cur.propKind != "" {
				cur.buf.Write(t)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			cur := stack[len(stack)-1]
			if t.Name.Local == "Properties" {
				cur.inProps = false
				cur.propKind = ""
				parentPath := ""
				if len(stack) >= 2 {
					parentPath = stack[len(stack)-2].path
				}
				name := cur.name
				if name == "" {
					name = cur.class
				}
				if parentPath == "" {
					cur.path = name
				} else {
					cur.path = parentPath + "." + name
				}
				if cur.uid != "" {
					byPath[cur.path] = instInfo{Class: cur.class, Name: name, UniqueID: cur.uid, Path: cur.path}
					if cur.class == "Workspace" && workspaceUID == "" {
						workspaceUID = uniqueIDToUUID(cur.uid)
					}
				}
				continue
			}
			if t.Name.Local == "Item" {
				stack = stack[:len(stack)-1]
				continue
			}
			if !cur.inProps || t.Name.Local != cur.propKind {
				continue
			}
			val := strings.TrimSpace(cur.buf.String())
			if cur.propKind == "string" && cur.propName == "Name" {
				cur.name = val
			}
			if cur.propKind == "UniqueId" && cur.propName == "UniqueId" {
				cur.uid = val
			}
			cur.propKind = ""
			cur.propName = ""
			cur.buf.Reset()
		}
	}
	return workspaceUID, byPath, nil
}

func canonicalPlacePath(p place) string {
	abs := absFromRepo(p.LocalPlaceFile)
	if abs == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil && real != "" {
		return real
	}
	return abs
}

func desiredSyncBindings(p place) (placeAbs, workspaceUUID string, bindings []syncBinding, err error) {
	placeAbs = canonicalPlacePath(p)
	if placeAbs == "" {
		return "", "", nil, fmt.Errorf("no local place file")
	}
	uid, byPath, err := parsePlaceInstances(placeAbs)
	if err != nil {
		return "", "", nil, err
	}
	if uid == "" {
		return "", "", nil, fmt.Errorf("place file has no Workspace UniqueId (open and save once in Studio)")
	}
	m := loadSyncManifest()
	if len(m.Roots) == 0 {
		m = resolveSyncManifest()
	}
	for _, r := range m.Roots {
		inst, ok := byPath[r.Instance]
		if !ok || inst.UniqueID == "" {
			return "", "", nil, fmt.Errorf("instance %s not in place file yet (plugin creates it on first open)", r.Instance)
		}
		disk := absSyncPath(r.Disk)
		if real, err := filepath.EvalSymlinks(disk); err == nil && real != "" {
			disk = real
		}
		bindings = append(bindings, syncBinding{
			ClassName: inst.Class,
			FilePath:  filepath.ToSlash(disk),
			ScriptID:  uniqueIDToUUID(inst.UniqueID),
			Status:    "Syncing",
		})
	}
	if len(bindings) == 0 {
		return "", "", nil, fmt.Errorf("no sync roots")
	}
	return placeAbs, uid, bindings, nil
}

func persistNeedsWrite(p place) bool {
	placeAbs, uid, want, err := desiredSyncBindings(p)
	if err != nil {
		return false
	}
	cur, err := readSyncPersistence(placeAbs, uid)
	if err != nil {
		return true
	}
	if len(cur) != len(want) {
		return true
	}
	type key struct{ Class, Path, ID string }
	have := map[key]bool{}
	for _, b := range cur {
		have[key{b.ClassName, b.FilePath, b.ScriptID}] = true
	}
	for _, b := range want {
		if !have[key{b.ClassName, b.FilePath, b.ScriptID}] {
			return true
		}
	}
	return false
}

func applyFileSyncPersistence(p place) error {
	placeAbs, uid, bindings, err := desiredSyncBindings(p)
	if err != nil {
		return err
	}
	lastDir := filepath.Dir(bindings[0].FilePath)
	body, err := json.MarshalIndent(bindings, "", "    ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	prefix := persistRecordPrefix(placeAbs, uid)
	keys := map[string]any{
		prefix:                            string(body),
		prefix + "_lastUsedDir":           lastDir,
		prefix + "_timeLastUsed":          time.Now().UnixMilli(),
		"File_Sync_Persistence_SafetyBit": false,
	}
	info("Script Sync resume key: %s", prefix)
	for _, b := range bindings {
		info("  %s %s  →  %s", b.ClassName, b.ScriptID, b.FilePath)
	}
	if dump := dumpDir(); dump != "" {
		_ = os.MkdirAll(dump, 0o755)
		_ = os.WriteFile(filepath.Join(dump, "sync-persist.json"), body, 0o644)
	}
	return writeSyncPersistence(keys)
}

func dumpPersistReport(w func(string, ...any), p place) {
	w("")
	w("== Script Sync resume records (plist) ==")
	placeAbs, uid, want, err := desiredSyncBindings(p)
	if err != nil {
		w("  desired: %v", err)
		return
	}
	w("  key %s", persistRecordPrefix(placeAbs, uid))
	for _, b := range want {
		w("  want %s %s → %s", b.ClassName, b.ScriptID, b.FilePath)
	}
	cur, err := readSyncPersistence(placeAbs, uid)
	if err != nil {
		w("  read: %v", err)
		return
	}
	if len(cur) == 0 {
		w("  have (empty)")
		return
	}
	for _, b := range cur {
		w("  have %s %s → %s status=%s", b.ClassName, b.ScriptID, b.FilePath, b.Status)
	}
}
