package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type mcpStudio struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mcpProbeResult struct {
	OK      bool
	Tools   int
	Studios []mcpStudio
	Detail  string
}

func studioMCPCommand(studio string) (command string, args []string) {
	if runtime.GOOS == "darwin" && (studio == "" || strings.Contains(studio, "RobloxStudio.app")) {
		return "/Applications/RobloxStudio.app/Contents/MacOS/StudioMCP", nil
	}
	return "cmd.exe", []string{"/c", `%LOCALAPPDATA%\Roblox\mcp.bat`}
}

func readySuffix(mcpOK bool) string {
	if mcpOK {
		return "Script Sync + MCP"
	}
	return "Script Sync"
}

func mcpNeedUserMessage() string {
	return "NEED_USER: Enable Studio as MCP server: Assistant → … → Manage MCP Servers → enable Studio as MCP server."
}

func mcpNeedPlaceMessage() string {
	return "NEED_USER: Open a place in Roblox Studio so Studio MCP can attach."
}

func mcpNeedUserForProbe(p mcpProbeResult) string {
	if p.OK {
		return ""
	}
	if p.Tools > 0 && p.Detail == "no Studio attached" {
		return mcpNeedPlaceMessage()
	}
	return mcpNeedUserMessage()
}

// Assistant → Manage MCP Servers → "Enable Studio as MCP server".
// Confirmed on Studio 0.738 (dump 2026-09-13, MCP on): this JSON bool lives in
// Documents/Roblox/<userId>/InstalledPlugins/0/settings.json (local-plugin
// settings namespace). FFlagAssistantExternalMCPPluginSettingRedundancy is
// True, so also write Library/Roblox/AssistantSettings/<userId>.json.
const assistantMCPSettingKey = "Assistant-ExternalMCPEnabled"

// mcpProbeOK is the shipped classifier: empty tools/list,
// "Unable to reach Roblox Studio", or studios: [] is not MCP-ready.
func mcpProbeOK(nTools int, blob string) bool {
	if nTools <= 0 {
		return false
	}
	if strings.Contains(blob, "Unable to reach Roblox Studio") {
		return false
	}
	return len(studiosFromBlob(blob)) > 0
}

func probeDefaultMCP() mcpProbeResult {
	cmd, args := studioMCPCommand(findStudio())
	return probeStudioMCP(cmd, args, 8*time.Second)
}

func probeStudioMCP(command string, args []string, timeout time.Duration) mcpProbeResult {
	if strings.TrimSpace(command) == "" {
		return mcpProbeResult{Detail: "no StudioMCP command"}
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return mcpProbeResult{Detail: err.Error()}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return mcpProbeResult{Detail: err.Error()}
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return mcpProbeResult{Detail: err.Error()}
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	send := func(v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = stdin.Write(append(b, '\n'))
		return err
	}

	if err := send(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "robld", "version": version},
		},
	}); err != nil {
		return mcpProbeResult{Detail: err.Error()}
	}

	r := bufio.NewReader(stdout)
	initMsg, initRaw, err := readJSONRPCLine(ctx, r)
	if err != nil {
		return mcpProbeResult{Detail: "initialize: " + err.Error()}
	}
	blob := string(initRaw)
	if errText := jsonRPCErrorText(initMsg); errText != "" {
		blob += "\n" + errText
	}

	_ = send(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if err := send(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}); err != nil {
		return mcpProbeResult{Detail: err.Error()}
	}

	listMsg, listRaw, err := readJSONRPCLine(ctx, r)
	if err != nil {
		return mcpProbeResult{Detail: "tools/list: " + err.Error()}
	}
	blob += "\n" + string(listRaw)
	if errText := jsonRPCErrorText(listMsg); errText != "" {
		blob += "\n" + errText
	}

	names := toolNamesFromRPC(listMsg)
	nTools := len(names)
	if hasTool(names, "list_roblox_studios") {
		_ = send(map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "list_roblox_studios",
				"arguments": map[string]any{},
			},
		})
		if callMsg, callRaw, callErr := readJSONRPCLine(ctx, r); callErr == nil {
			blob += "\n" + string(callRaw)
			if errText := jsonRPCErrorText(callMsg); errText != "" {
				blob += "\n" + errText
			}
			blob += "\n" + rpcTextContent(callMsg)
		}
	}

	studios := studiosFromBlob(blob)
	ok := mcpProbeOK(nTools, blob)
	detail := "tools/list"
	if !ok {
		if nTools > 0 && !strings.Contains(blob, "Unable to reach Roblox Studio") {
			detail = "no Studio attached"
		} else {
			detail = "Studio MCP not connected"
		}
	}
	return mcpProbeResult{OK: ok, Tools: nTools, Studios: studios, Detail: detail}
}

func studiosFromBlob(blob string) []mcpStudio {
	if list := studiosFromText(blob); list != nil {
		if len(list) > 0 {
			return list
		}
	}
	var empty []mcpStudio
	for _, line := range strings.Split(blob, "\n") {
		if list := studiosFromText(line); list != nil {
			if len(list) > 0 {
				return list
			}
			empty = list
		}
	}
	return empty
}

func studiosFromText(s string) []mcpStudio {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " error")
	if s == "" {
		return nil
	}
	var wrap struct {
		Studios *[]mcpStudio `json:"studios"`
	}
	if json.Unmarshal([]byte(s), &wrap) == nil && wrap.Studios != nil {
		return *wrap.Studios
	}
	idx := strings.Index(s, `"studios"`)
	if idx < 0 {
		return nil
	}
	start := strings.LastIndex(s[:idx], "{")
	if start < 0 {
		start = idx
		s = "{" + s[idx:]
		start = 0
	}
	dec := json.NewDecoder(strings.NewReader(s[start:]))
	if dec.Decode(&wrap) == nil && wrap.Studios != nil {
		return *wrap.Studios
	}
	return nil
}

func readJSONRPCLine(ctx context.Context, r *bufio.Reader) (map[string]any, []byte, error) {
	for {
		line, err := readLineCtx(ctx, r)
		if err != nil {
			return nil, nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}
		return msg, []byte(line), nil
	}
}

func readLineCtx(ctx context.Context, r *bufio.Reader) (string, error) {
	type res struct {
		line string
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		s, err := r.ReadString('\n')
		ch <- res{s, err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case out := <-ch:
		if out.err != nil && out.err != io.EOF {
			return out.line, out.err
		}
		if out.err == io.EOF && strings.TrimSpace(out.line) == "" {
			return "", io.EOF
		}
		return out.line, nil
	}
}

func jsonRPCErrorText(msg map[string]any) string {
	errObj, _ := msg["error"].(map[string]any)
	if errObj == nil {
		return ""
	}
	if m, ok := errObj["message"].(string); ok {
		return m
	}
	return fmt.Sprint(errObj)
}

func toolNamesFromRPC(msg map[string]any) []string {
	result, _ := msg["result"].(map[string]any)
	if result == nil {
		return nil
	}
	raw, _ := result["tools"].([]any)
	var names []string
	for _, t := range raw {
		row, _ := t.(map[string]any)
		if row == nil {
			continue
		}
		if n, ok := row["name"].(string); ok && n != "" {
			names = append(names, n)
		}
	}
	return names
}

func hasTool(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func rpcTextContent(msg map[string]any) string {
	var b strings.Builder
	if s := jsonRPCErrorText(msg); s != "" {
		b.WriteString(s)
	}
	result, _ := msg["result"].(map[string]any)
	if result == nil {
		return b.String()
	}
	if content, ok := result["content"].([]any); ok {
		for _, c := range content {
			row, _ := c.(map[string]any)
			if row == nil {
				continue
			}
			if txt, ok := row["text"].(string); ok {
				b.WriteString(txt)
			}
		}
	}
	if isErr, _ := result["isError"].(bool); isErr {
		b.WriteString(" error")
	}
	return b.String()
}

func jsonBoolTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func jsonFileHasTrue(path, key string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	return jsonBoolTrue(m[key])
}

func ensureJSONBool(path, key string, want bool) (bool, error) {
	raw, err := os.ReadFile(path)
	m := map[string]any{}
	exists := err == nil
	if exists {
		if trim := bytes.TrimSpace(raw); len(trim) > 0 {
			if json.Unmarshal(raw, &m) != nil {
				return false, fmt.Errorf("%s: invalid JSON", path)
			}
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if exists {
		if v, ok := m[key]; ok && jsonBoolTrue(v) == want {
			return false, nil
		}
	}
	m[key] = want
	out, err := json.Marshal(m)
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func robloxUserDocumentsDirs() []string {
	var dirs []string
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		dirs = append(dirs,
			filepath.Join(home, "Documents", "Roblox"),
			filepath.Join(home, "Documents", "ROBLOX"),
		)
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			dirs = append(dirs, filepath.Join(local, "Roblox"))
		}
	}
	return dirs
}

func studioMCPPluginSettingFiles() []string {
	seen := map[string]bool{}
	var out []string
	for _, base := range robloxUserDocumentsDirs() {
		matches, _ := filepath.Glob(filepath.Join(base, "*", "InstalledPlugins", "0", "settings.json"))
		for _, m := range matches {
			abs, err := filepath.Abs(m)
			if err != nil {
				abs = m
			}
			if seen[abs] {
				continue
			}
			if st, err := os.Stat(abs); err != nil || st.IsDir() {
				continue
			}
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out
}

func pluginSettingsUserID(path string) string {
	// .../<userId>/InstalledPlugins/0/settings.json
	return filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
}

func isLoggedOutPluginSettings(path string) bool {
	return pluginSettingsUserID(path) == "0"
}

func studioMCPSettingNeedApply() bool {
	for _, p := range studioMCPPluginSettingFiles() {
		if isLoggedOutPluginSettings(p) {
			continue
		}
		if !jsonFileHasTrue(p, assistantMCPSettingKey) {
			return true
		}
	}
	return false
}

func studioUserIDs() []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, p := range studioMCPPluginSettingFiles() {
		add(pluginSettingsUserID(p))
	}
	for _, dir := range studioSettingsDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "AssistantSettings", "*.json"))
		for _, m := range matches {
			add(strings.TrimSuffix(filepath.Base(m), filepath.Ext(m)))
		}
	}
	return ids
}

func studioMCPSettingWriteTargets() []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if strings.TrimSpace(p) == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}
	for _, p := range studioMCPPluginSettingFiles() {
		add(p)
	}
	for _, id := range studioUserIDs() {
		for _, base := range robloxUserDocumentsDirs() {
			add(filepath.Join(base, id, "InstalledPlugins", "0", "settings.json"))
		}
		for _, dir := range studioSettingsDirs() {
			add(filepath.Join(dir, "AssistantSettings", id+".json"))
		}
	}
	return out
}

func applyStudioMCPSetting() bool {
	n := 0
	for _, path := range studioMCPSettingWriteTargets() {
		changed, err := ensureJSONBool(path, assistantMCPSettingKey, true)
		if err != nil {
			warn("Could not write Studio MCP setting %s: %v", path, err)
			continue
		}
		if changed {
			n++
			info("Wrote %s=true → %s", assistantMCPSettingKey, path)
		}
	}
	if n == 0 && studioMCPSettingNeedApply() {
		warn("Studio MCP setting %s is not on and no settings file could be written.", assistantMCPSettingKey)
	}
	return n > 0
}

func writeMCPProbeReport(w func(string, ...any), probe mcpProbeResult) {
	w("== Studio MCP probe ==")
	w("  ok=%v tools=%d studios=%d", probe.OK, probe.Tools, len(probe.Studios))
	if probe.Detail != "" {
		w("  detail=%s", probe.Detail)
	}
	if !probe.OK {
		if msg := mcpNeedUserForProbe(probe); msg != "" {
			w("  %s", msg)
		}
	}
	for _, s := range probe.Studios {
		w("  studio %s  %s", s.ID, s.Name)
	}
}

func writeMCPSettingReport(w func(string, ...any)) {
	w("== Studio MCP setting ==")
	w("key %s  want true", assistantMCPSettingKey)
	files := studioMCPPluginSettingFiles()
	for _, dir := range studioSettingsDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "AssistantSettings", "*.json"))
		files = append(files, matches...)
	}
	seen := map[string]bool{}
	n := 0
	for _, p := range files {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		n++
		status := "MISSING KEY"
		if jsonFileHasTrue(abs, assistantMCPSettingKey) {
			status = "ok"
		} else if raw, err := os.ReadFile(abs); err != nil {
			status = err.Error()
		} else if len(bytes.TrimSpace(raw)) == 0 {
			status = "empty"
		}
		w("  %s  %s", abs, status)
	}
	if n == 0 {
		w("  (no InstalledPlugins/0/settings.json or AssistantSettings/*.json yet)")
	}
}
