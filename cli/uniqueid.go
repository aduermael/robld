package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UniqueId XML layout (rbx-dom / Studio rbxlx):
//
//	[random 8 bytes][time u32 BE][index u32 BE]  as 32 lowercase hex chars.
//
// Time is seconds since 2021-01-01 UTC. Index increments per instance in the
// file. Random is shared across instances in one place (Studio does this too).
const uniqueIDEpochUnix = 1609459200 // 2021-01-01 00:00:00 UTC

func uniqueIDTime(now time.Time) uint32 {
	sec := now.UTC().Unix() - uniqueIDEpochUnix
	if sec < 0 {
		return 0
	}
	if sec > int64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(sec)
}

func formatUniqueID(random [8]byte, when, index uint32) string {
	var buf [16]byte
	copy(buf[0:8], random[:])
	binary.BigEndian.PutUint32(buf[8:12], when)
	binary.BigEndian.PutUint32(buf[12:16], index)
	return hex.EncodeToString(buf[:])
}

func newPlaceXML() (string, error) {
	return injectMissingUniqueIds(minimalPlaceXML)
}

func injectMissingUniqueIds(src string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate UniqueId random: %w", err)
	}
	out, _ := injectMissingUniqueIdsAt(src, random, uniqueIDTime(time.Now()), 1)
	return out, nil
}

func injectMissingUniqueIdsAt(src string, random [8]byte, when, start uint32) (string, int) {
	parts := strings.Split(src, "</Properties>")
	if len(parts) < 2 {
		return src, 0
	}
	index := start
	var b strings.Builder
	n := 0
	for i, part := range parts {
		if i == len(parts)-1 {
			b.WriteString(part)
			break
		}
		if !strings.Contains(part, `<UniqueId name="UniqueId">`) {
			id := formatUniqueID(random, when, index)
			index++
			n++
			part += "\n" + propertiesIndent(part) + `<UniqueId name="UniqueId">` + id + `</UniqueId>`
		}
		b.WriteString(part)
		b.WriteString("</Properties>")
	}
	return b.String(), n
}

func propertiesIndent(block string) string {
	lines := strings.Split(block, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		return line[:len(line)-len(trimmed)]
	}
	return "\t\t\t"
}

func rbxlxMissingUniqueIds(src string) bool {
	parts := strings.Split(src, "</Properties>")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if !strings.Contains(part, `<UniqueId name="UniqueId">`) {
			return true
		}
	}
	return false
}

func placeNeedsUniqueIds(p place) bool {
	path := canonicalPlacePath(p)
	if path == "" || strings.ToLower(filepath.Ext(path)) != ".rbxlx" {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return rbxlxMissingUniqueIds(string(raw))
}

func ensurePlaceUniqueIds(p place) error {
	path := canonicalPlacePath(p)
	if path == "" || strings.ToLower(filepath.Ext(path)) != ".rbxlx" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(raw)
	if !rbxlxMissingUniqueIds(src) {
		return nil
	}
	out, err := injectMissingUniqueIds(src)
	if err != nil {
		return err
	}
	if out == src {
		return nil
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	info("Seeded UniqueIds into %s so Script Sync can auto-resume.", repoRel(path))
	return nil
}
