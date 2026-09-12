package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Studio names placeIDEState files from the local place path (or placeId://N)
// using an RS hash (tabs XML) and a Knuth hash (debugger XML).
// See https://devforum.roblox.com/t/roblox-studio-placeidestate-path-hash-calculation/3784645

func rsHash31(s string) uint32 {
	var a int32 = 63689
	var b int32 = 378551
	var hash int32
	for i := 0; i < len(s); i++ {
		hash = hash*a + int32(s[i])
		a *= b
	}
	return uint32(hash) & 0x7FFFFFFF
}

func knuthHash64(s string) uint64 {
	var hash uint64
	const golden uint64 = 2654435769
	for i := 0; i < len(s); i++ {
		hash ^= (hash >> 2) + golden + (hash << 6) + uint64(s[i])
	}
	return hash
}

func placeIDEStateName(pathKey string) string {
	return "placeIDEState" + strconv.FormatUint(uint64(rsHash31(pathKey)), 10) + ".xml"
}

func placeIDEStateDebuggerName(pathKey string) string {
	return "placeIDEState_" + strconv.FormatUint(knuthHash64(pathKey), 10) + "_DebuggerData.xml"
}

func placePathKeys(p place) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		keys = append(keys, s)
	}
	if local := absFromRepo(p.LocalPlaceFile); local != "" {
		add(local)
		add(filepath.ToSlash(local))
		if runtime.GOOS == "windows" {
			add(strings.ReplaceAll(local, `/`, `\`))
			if len(local) >= 2 && local[1] == ':' {
				add(strings.ToUpper(local[:1]) + ":/" + filepath.ToSlash(local[2:]))
				add(strings.ToLower(local[:1]) + ":/" + filepath.ToSlash(local[2:]))
			}
		}
	}
	if p.PlaceID != 0 {
		add(fmt.Sprintf("placeId://%d", p.PlaceID))
	}
	return keys
}
