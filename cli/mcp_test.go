package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("ROBLD_FAKE_MCP"); mode != "" {
		if err := runFakeMCPServer(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runFakeMCPServer(mode string) error {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for {
		var req map[string]any
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		method, _ := req["method"].(string)
		id := req["id"]
		switch method {
		case "initialize":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{},
					"serverInfo":      map[string]any{"name": "RobloxStudio"},
				},
			})
		case "notifications/initialized":
			continue
		case "tools/list":
			var tools []any
			switch mode {
			case "ok", "unreachable":
				tools = []any{
					map[string]any{"name": "list_roblox_studios"},
					map[string]any{"name": "start_stop_play"},
				}
			default:
				tools = []any{}
			}
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  map[string]any{"tools": tools},
			})
		case "tools/call":
			params, _ := req["params"].(map[string]any)
			name, _ := params["name"].(string)
			text := "ok"
			isErr := false
			if mode == "unreachable" || name == "list_roblox_studios" && mode == "unreachable" {
				text = "Unable to reach Roblox Studio right now. Ask the user to confirm that Studio is running with a place open and the MCP server enabled in Assistant settings."
				isErr = true
			}
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"content": []any{map[string]any{"type": "text", "text": text}},
					"isError": isErr,
				},
			})
		}
	}
}

func probeFake(t *testing.T, mode string) mcpProbeResult {
	t.Helper()
	return probeStudioMCPWithEnv(os.Args[0], []string{"-test.run=^$"}, mode, 8*time.Second)
}

func probeStudioMCPWithEnv(command string, args []string, mode string, timeout time.Duration) mcpProbeResult {
	oldPath := os.Getenv("ROBLD_FAKE_MCP")
	_ = os.Setenv("ROBLD_FAKE_MCP", mode)
	defer func() {
		if oldPath == "" {
			_ = os.Unsetenv("ROBLD_FAKE_MCP")
		} else {
			_ = os.Setenv("ROBLD_FAKE_MCP", oldPath)
		}
	}()
	return probeStudioMCP(command, args, timeout)
}

func TestMCPProbeEmptyTools(t *testing.T) {
	got := probeFake(t, "empty")
	if got.OK {
		t.Fatalf("empty tools/list must not be MCP-ready: %+v", got)
	}
	if got.Detail != "Studio MCP not connected" {
		t.Fatalf("probe must reach tools/list (not fail to start): %+v", got)
	}
	if mcpProbeOK(0, `{"tools":[]}`) {
		t.Fatal("classifier must reject empty tools")
	}
	msg := readySuffix(got.OK)
	if msg != "Script Sync" {
		t.Fatalf("ready suffix %q", msg)
	}
	need := mcpNeedUserMessage()
	if !strings.Contains(need, "NEED_USER:") || !strings.Contains(need, "Manage MCP Servers") {
		t.Fatalf("NEED_USER missing Assistant MCP toggle: %q", need)
	}
	if strings.Contains("READY: "+msg, "READY: Script Sync + MCP") {
		t.Fatal("must not claim MCP")
	}
}

func TestMCPProbeUnreachable(t *testing.T) {
	got := probeFake(t, "unreachable")
	if got.OK {
		t.Fatalf("Unable to reach Roblox Studio must not be MCP-ready: %+v", got)
	}
	blob := "Unable to reach Roblox Studio right now"
	if mcpProbeOK(2, blob) {
		t.Fatal("classifier must reject unreachable Studio")
	}
}

func TestMCPProbeNonEmptyTools(t *testing.T) {
	got := probeFake(t, "ok")
	if !got.OK {
		t.Fatalf("non-empty tools/list should claim MCP: %+v", got)
	}
	if got.Tools < 1 {
		t.Fatalf("expected tools from fake server, got %+v", got)
	}
	if readySuffix(true) != "Script Sync + MCP" {
		t.Fatal(readySuffix(true))
	}
}

func TestMCPProbeOKClassifier(t *testing.T) {
	if mcpProbeOK(0, "") {
		t.Fatal("0 tools")
	}
	if mcpProbeOK(3, "Unable to reach Roblox Studio") {
		t.Fatal("unreachable")
	}
	if !mcpProbeOK(1, `{"tools":[{"name":"list_roblox_studios"}]}`) {
		t.Fatal("non-empty tools should be ok")
	}
}

func TestPluginSettingsUserID(t *testing.T) {
	p := filepath.Join("Documents", "Roblox", "7924826801", "InstalledPlugins", "0", "settings.json")
	if got := pluginSettingsUserID(p); got != "7924826801" {
		t.Fatalf("got %q", got)
	}
	if !isLoggedOutPluginSettings(filepath.Join("Roblox", "0", "InstalledPlugins", "0", "settings.json")) {
		t.Fatal("user 0 is logged-out namespace")
	}
}

func TestEnsureJSONBoolMergesAndNoops(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte(`{"other":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := ensureJSONBool(p, assistantMCPSettingKey, true)
	if err != nil || !changed {
		t.Fatalf("first write changed=%v err=%v", changed, err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if !jsonBoolTrue(m[assistantMCPSettingKey]) {
		t.Fatalf("missing enable key: %s", raw)
	}
	if m["other"].(float64) != 1 {
		t.Fatalf("must keep other keys: %s", raw)
	}
	changed, err = ensureJSONBool(p, assistantMCPSettingKey, true)
	if err != nil || changed {
		t.Fatalf("second write should be a no-op: changed=%v err=%v", changed, err)
	}
}

func TestEnsureJSONBoolCreates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "settings.json")
	changed, err := ensureJSONBool(p, assistantMCPSettingKey, true)
	if err != nil || !changed {
		t.Fatalf("create changed=%v err=%v", changed, err)
	}
	if !jsonFileHasTrue(p, assistantMCPSettingKey) {
		t.Fatal("created file must have enable key")
	}
}

func TestJSONBoolTrue(t *testing.T) {
	if !jsonBoolTrue(true) || jsonBoolTrue(false) {
		t.Fatal("bool")
	}
	if !jsonBoolTrue("True") || !jsonBoolTrue("true") || jsonBoolTrue("false") {
		t.Fatal("string")
	}
}
