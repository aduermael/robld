package main

import (
	"fmt"
	"strings"
)

type statusSnapshot struct {
	Place     place
	Running   bool
	PID       int
	Probe     mcpProbeResult
	Instances []string
}

func runStatus() {
	p := loadPlace()
	running := studioRunning("")
	pid := 0
	if running {
		pid, _ = studioPID()
	}
	fmt.Print(formatStatus(statusSnapshot{
		Place:     p,
		Running:   running,
		PID:       pid,
		Probe:     probeDefaultMCP(),
		Instances: syncedInstanceNames(latestPluginSettingsJSON()),
	}))
}

func formatStatus(s statusSnapshot) string {
	var b strings.Builder
	name := strings.TrimSpace(s.Place.Name)
	if name == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(&b, "place: %s\n", name)
	if f := strings.TrimSpace(s.Place.LocalPlaceFile); f != "" {
		fmt.Fprintf(&b, "  file: %s\n", f)
	}
	if s.Place.PlaceID != 0 {
		fmt.Fprintf(&b, "  placeId %d  universeId %d\n", s.Place.PlaceID, s.Place.UniverseID)
	}
	if s.Running && s.PID > 0 {
		fmt.Fprintf(&b, "Studio: running pid=%d\n", s.PID)
	} else if s.Running {
		fmt.Fprintf(&b, "Studio: running\n")
	} else {
		fmt.Fprintf(&b, "Studio: not running\n")
	}
	if s.Probe.OK {
		fmt.Fprintf(&b, "MCP: connected (%d studio(s))\n", len(s.Probe.Studios))
		for _, st := range s.Probe.Studios {
			fmt.Fprintf(&b, "  %s  %s\n", st.ID, st.Name)
		}
	} else {
		fmt.Fprintf(&b, "MCP: not connected\n")
		if s.Probe.Detail != "" {
			fmt.Fprintf(&b, "  detail: %s  tools=%d studios=%d\n", s.Probe.Detail, s.Probe.Tools, len(s.Probe.Studios))
		}
		if msg := mcpNeedUserForProbe(s.Probe); msg != "" {
			fmt.Fprintf(&b, "  %s\n", msg)
		}
	}
	if len(s.Instances) == 0 {
		fmt.Fprintf(&b, "Script Sync instances: (none)\n")
	} else {
		fmt.Fprintf(&b, "Script Sync instances (%d):\n", len(s.Instances))
		for _, n := range s.Instances {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	return b.String()
}

func syncedInstanceNames(js map[string]any) []string {
	if js == nil {
		return nil
	}
	sync, _ := js["sync"].(map[string]any)
	if sync == nil {
		return nil
	}
	raw, _ := sync["instances"].([]any)
	var names []string
	seen := map[string]bool{}
	for _, item := range raw {
		row, _ := item.(map[string]any)
		if row == nil {
			continue
		}
		n := str(row["fullName"])
		if n == "" {
			n = str(row["name"])
		}
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}
	return names
}
