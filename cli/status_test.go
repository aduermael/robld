package main

import (
	"strings"
	"testing"
)

func TestFormatStatusConnected(t *testing.T) {
	dump := map[string]any{
		"sync": map[string]any{
			"instances": []any{
				map[string]any{"fullName": "ReplicatedStorage.ResetLock", "class": "ModuleScript"},
				map[string]any{"fullName": "ServerScriptService.Game", "class": "Script"},
			},
		},
	}
	out := formatStatus(statusSnapshot{
		Place:   place{Name: "Slash Waves", LocalPlaceFile: "place.rbxlx"},
		Running: true,
		PID:     51656,
		Probe: mcpProbeResult{
			OK:      true,
			Tools:   28,
			Studios: []mcpStudio{{ID: "abc-id", Name: "place.rbxlx"}},
		},
		Instances: syncedInstanceNames(dump),
	})
	for _, need := range []string{
		"Slash Waves",
		"place.rbxlx",
		"51656",
		"MCP: connected",
		"abc-id",
		"ReplicatedStorage.ResetLock",
		"ServerScriptService.Game",
	} {
		if !strings.Contains(out, need) {
			t.Fatalf("status missing %q\n%s", need, out)
		}
	}
}

func TestFormatStatusNotRunning(t *testing.T) {
	out := formatStatus(statusSnapshot{
		Place: place{LocalPlaceFile: "place.rbxlx"},
		Probe: mcpProbeResult{Tools: 28, Detail: "no Studio attached"},
	})
	for _, need := range []string{
		"place.rbxlx",
		"not running",
		"MCP: not connected",
		"Open a place",
		"Script Sync instances: (none)",
	} {
		if !strings.Contains(out, need) {
			t.Fatalf("status missing %q\n%s", need, out)
		}
	}
	if strings.Contains(out, "Manage MCP Servers") {
		t.Fatalf("empty studios must not use the MCP toggle:\n%s", out)
	}
}

func TestSyncedInstanceNames(t *testing.T) {
	got := syncedInstanceNames(map[string]any{
		"sync": map[string]any{
			"instances": []any{
				map[string]any{"fullName": "ReplicatedStorage.Hello"},
				map[string]any{"name": "Game"},
			},
		},
	})
	if strings.Join(got, ",") != "ReplicatedStorage.Hello,Game" {
		t.Fatalf("got %v", got)
	}
	if syncedInstanceNames(nil) != nil {
		t.Fatal("nil dump")
	}
}

func TestUsageListsStatus(t *testing.T) {
	u := usage()
	if !strings.Contains(u, "robld status") {
		t.Fatalf("usage missing status:\n%s", u)
	}
}
