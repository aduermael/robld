package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type mcpProbeResult struct {
	OK     bool
	Tools  int
	Detail string
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

// mcpProbeOK is the shipped classifier: empty tools/list or
// "Unable to reach Roblox Studio" is not MCP-ready.
func mcpProbeOK(nTools int, blob string) bool {
	if nTools <= 0 {
		return false
	}
	return !strings.Contains(blob, "Unable to reach Roblox Studio")
}

func studioMCPConnected(studio string) bool {
	cmd, args := studioMCPCommand(studio)
	return probeStudioMCP(cmd, args, 8*time.Second).OK
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

	ok := mcpProbeOK(nTools, blob)
	detail := "tools/list"
	if !ok {
		detail = "Studio MCP not connected"
	}
	return mcpProbeResult{OK: ok, Tools: nTools, Detail: detail}
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
