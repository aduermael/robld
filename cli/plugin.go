package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const pluginFileName = "robuild_agent.lua"

// Local Studio plugin. InstanceFileSyncService is documented query-only;
// we still create the Folder tree, try FileSync/InternalSync start methods,
// and print ROBUILD_JSON so robld can see whether resume actually bound.
const pluginTemplate = `-- robld: Script Sync prefs, folder tree, start-sync probe.
local HttpService = game:GetService("HttpService")
local PROJECT = @@PROJECT@@
local ROOTS = HttpService:JSONDecode([==[@@ROOTS@@]==])
local FILES = HttpService:JSONDecode([==[@@FILES@@]==])

local function trySet(studio, name, value)
	local ok, err = pcall(function()
		studio[name] = value
	end)
	if ok then
		return { ok = true }
	end
	return { ok = false, err = tostring(err) }
end

local function applyPrefs()
	local okS, studio = pcall(function()
		return settings().Studio
	end)
	if not okS then
		return { error = tostring(studio) }
	end
	local out = {}
	out.AutoResumeSyncOnPlaceOpen = trySet(studio, "AutoResumeSyncOnPlaceOpen", true)
	pcall(function()
		out.ActionOnAutoResumeSync = trySet(studio, "ActionOnAutoResumeSync", Enum.ActionOnAutoResumeSync.KeepLocal)
	end)
	pcall(function()
		out.ActionOnStopSync = trySet(studio, "ActionOnStopSync", Enum.ActionOnStopSync.KeepLocalFiles)
	end)
	pcall(function()
		out.DefaultScriptSyncFileType = trySet(studio, "DefaultScriptSyncFileType", Enum.DefaultScriptSyncFileType.Luau)
	end)
	return out
end

local function findInstance(path)
	local parts = {}
	for name in string.gmatch(path, "[^.]+") do
		table.insert(parts, name)
	end
	if #parts == 0 then
		return nil
	end
	local cur
	local ok, svc = pcall(function()
		return game:GetService(parts[1])
	end)
	if ok then
		cur = svc
	else
		cur = game:FindFirstChild(parts[1])
	end
	if not cur then
		return nil
	end
	for i = 2, #parts do
		cur = cur:FindFirstChild(parts[i])
		if not cur then
			return nil
		end
	end
	return cur
end

local function ensureFolder(path)
	local parts = {}
	for name in string.gmatch(path, "[^.]+") do
		table.insert(parts, name)
	end
	if #parts == 0 then
		return nil, "empty path"
	end
	local ok, cur = pcall(function()
		return game:GetService(parts[1])
	end)
	if not ok or not cur then
		return nil, "missing service " .. tostring(parts[1])
	end
	for i = 2, #parts do
		local child = cur:FindFirstChild(parts[i])
		if not child then
			child = Instance.new("Folder")
			child.Name = parts[i]
			child.Parent = cur
		end
		cur = child
	end
	return cur, nil
end

local function applyFiles()
	local created = {}
	-- FILES is only the first --new seed. Later runs pass [] so deleting a
	-- synced script on disk cannot resurrect it here.
	if FILES == nil or #FILES == 0 then
		return created
	end
	for _, f in ipairs(FILES) do
		local parent, err = ensureFolder(f.parent)
		if not parent then
			table.insert(created, { name = f.name, error = err })
		else
			local inst = parent:FindFirstChild(f.name)
			if not inst then
				local className = f.class or "ModuleScript"
				inst = Instance.new(className)
				inst.Name = f.name
				if f.runContext and f.runContext ~= "" then
					pcall(function()
						inst.RunContext = Enum.RunContext[f.runContext]
					end)
				end
				pcall(function()
					inst.Source = f.source or ""
				end)
				inst.Parent = parent
				table.insert(created, { name = f.name, class = className, parent = f.parent, new = true })
			else
				table.insert(created, { name = f.name, class = inst.ClassName, parent = f.parent, new = false })
			end
		end
	end
	return created
end

local startNames = {
	"StartSync", "RequestSync", "BeginSync", "SyncInstance", "StartFileSync",
	"SyncToPath", "StartSyncing", "EnableSync", "SyncToDirectory", "StartFolderSync",
	"Bind", "BindPath", "SetSyncPath", "Sync", "Start",
}
local serviceNames = {
	"InstanceFileSyncService", "FileSyncService", "InternalSyncService",
	"LiveSyncService", "LSPFileSyncService",
}

local function probeStart(svc, inst, path, probes)
	for _, name in ipairs(startNames) do
		local fn = svc[name]
		if typeof(fn) == "function" then
			local attempts = {
				function() return fn(svc, inst, path) end,
				function() return fn(svc, path, inst) end,
				function() return fn(svc, inst, path, true) end,
			}
			for i, attempt in ipairs(attempts) do
				local okp, err = pcall(attempt)
				probes[name .. "#" .. tostring(i)] = { exists = true, ok = okp, err = okp and nil or tostring(err) }
				if okp then
					return name
				end
			end
		elseif probes[name] == nil then
			probes[name] = { exists = false }
		end
	end
	return nil
end

local function primaryService()
	for _, sname in ipairs(serviceNames) do
		local ok, svc = pcall(function()
			return game:GetService(sname)
		end)
		if ok and svc then
			return svc, sname
		end
	end
	return nil, nil
end

local function dumpSync(createIfMissing)
	local created = {}
	if createIfMissing then
		created = applyFiles()
	end
	local probes = {}
	local listed = {}
	local roots = {}
	local primary = primaryService()
	if primary then
		pcall(function()
			for _, inst in ipairs(primary:GetAllInstances()) do
				local st
				pcall(function()
					st = tostring(primary:GetStatus(inst))
				end)
				table.insert(listed, {
					class = inst.ClassName,
					name = inst.Name,
					fullName = inst:GetFullName(),
					status = st,
				})
			end
		end)
	end
	for _, root in ipairs(ROOTS) do
		local row = { instance = root.instance, path = root.path }
		local inst = findInstance(root.instance)
		if not inst and createIfMissing then
			local err
			inst, err = ensureFolder(root.instance)
			row.ensureError = err
		end
		row.found = inst ~= nil
		if inst and primary then
			pcall(function()
				row.status = tostring(primary:GetStatus(inst))
			end)
			pcall(function()
				local mapped = primary:GetSyncedInstance(root.path)
				if mapped then
					row.mappedClass = mapped.ClassName
					row.mappedName = mapped:GetFullName()
				end
			end)
			local st = string.lower(tostring(row.status or ""))
			if row.mappedName == nil and not string.find(st, "syncedasroot", 1, true) then
				row.started = probeStart(primary, inst, root.path, probes)
			end
		end
		table.insert(roots, row)
	end
	return {
		created = created,
		instances = listed,
		roots = roots,
		probes = probes,
	}
end

local function emit(createIfMissing)
	local report = {
		project = PROJECT,
		prefs = applyPrefs(),
		sync = dumpSync(createIfMissing),
	}
	local ok, encoded = pcall(function()
		return HttpService:JSONEncode(report)
	end)
	if not ok then
		warn("ROBUILD_JSON_ERROR:" .. tostring(encoded))
		return
	end
	warn("ROBUILD_JSON:" .. encoded)
	print("ROBUILD_JSON:" .. encoded)
	pcall(function()
		plugin:SetSetting("robuildDump", encoded)
	end)
end

warn("ROBUILD_PLUGIN_LOADED")
pcall(function()
	plugin:SetSetting("robuildDump", '{"plugin":"loaded"}')
end)

-- Never Wait() on game.Loaded from a plugin: if Loaded already fired, that
-- hangs forever and ROBUILD_JSON / SetSetting never run. Studio Edit can
-- query services immediately. Delay create so File_Sync resume can bind first.
task.defer(function()
	pcall(emit, false)
end)
task.delay(3, function()
	pcall(emit, true)
end)
task.delay(8, function()
	pcall(emit, false)
end)
`

type luaRoot struct {
	Instance string `json:"instance"`
	Path     string `json:"path"`
}

type luaFile struct {
	Parent     string `json:"parent"`
	Name       string `json:"name"`
	Class      string `json:"class"`
	RunContext string `json:"runContext,omitempty"`
	Source     string `json:"source"`
}

func shouldSeedPlugin(newPlace bool, pluginPath string) bool {
	if !newPlace {
		return false
	}
	_, err := os.Stat(pluginPath)
	return err != nil
}

func installRobuildPlugin(newPlace bool) (string, bool) {
	dir := studioPluginsDir()
	if dir == "" {
		warn("Could not find Studio Plugins folder — skip live Script Sync plugin.")
		return "", false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		warn("Could not create Plugins folder %s: %v", dir, err)
		return "", false
	}
	path := filepath.Join(dir, pluginFileName)
	src, err := renderPluginSource(shouldSeedPlugin(newPlace, path))
	if err != nil {
		warn("Could not render robld plugin: %v", err)
		return "", false
	}
	old, _ := os.ReadFile(path)
	if bytes.Equal(old, []byte(src)) {
		return path, false
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		warn("Could not install Studio plugin %s: %v", path, err)
		return "", false
	}
	info("Installed Studio plugin %s", path)
	return path, true
}

func renderPluginSource(seed bool) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	proj, err := json.Marshal(filepath.ToSlash(abs))
	if err != nil {
		return "", err
	}
	m := loadSyncManifest()
	if len(m.Roots) == 0 {
		m = resolveSyncManifest()
	}
	var roots []luaRoot
	for _, r := range m.Roots {
		roots = append(roots, luaRoot{
			Instance: r.Instance,
			Path:     filepath.ToSlash(absSyncPath(r.Disk)),
		})
	}
	rootsJSON, err := json.Marshal(roots)
	if err != nil {
		return "", err
	}
	files := []luaFile{}
	if seed {
		files = collectSeedFiles(m)
	}
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	src := pluginTemplate
	src = strings.Replace(src, "@@PROJECT@@", string(proj), 1)
	src = strings.Replace(src, "@@ROOTS@@", string(rootsJSON), 1)
	src = strings.Replace(src, "@@FILES@@", string(filesJSON), 1)
	return src, nil
}

func collectSeedFiles(m syncManifest) []luaFile {
	var out []luaFile
	for _, sf := range greenfieldSeedFiles() {
		rel := filepath.ToSlash(sf.Rel)
		path := filepath.Join(root, filepath.FromSlash(rel))
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name, class, run := scriptInstanceName(filepath.Base(rel))
		if name == "" {
			continue
		}
		out = append(out, luaFile{
			Parent:     seedParent(m, rel),
			Name:       name,
			Class:      class,
			RunContext: run,
			Source:     string(body),
		})
	}
	return out
}

func seedParent(m syncManifest, rel string) string {
	disk := diskRootForScript(rel)
	rest := strings.TrimPrefix(filepath.ToSlash(rel), disk+"/")
	dir := filepath.ToSlash(filepath.Dir(rest))
	parent := inferInstancePath(disk)
	for _, r := range m.Roots {
		if filepath.ToSlash(r.Disk) == disk {
			parent = r.Instance
			break
		}
	}
	if dir != "." && dir != "" {
		return parent + "." + strings.ReplaceAll(dir, "/", ".")
	}
	return parent
}

func scriptInstanceName(filename string) (name, class, runContext string) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".server.luau") || strings.HasSuffix(lower, ".server.lua"):
		return trimScriptExt(filename, ".server.luau", ".server.lua"), "Script", "Server"
	case strings.HasSuffix(lower, ".client.luau") || strings.HasSuffix(lower, ".client.lua"):
		return trimScriptExt(filename, ".client.luau", ".client.lua"), "Script", "Client"
	case strings.HasSuffix(lower, ".local.luau") || strings.HasSuffix(lower, ".local.lua"):
		return trimScriptExt(filename, ".local.luau", ".local.lua"), "LocalScript", ""
	case strings.HasSuffix(lower, ".legacy.luau") || strings.HasSuffix(lower, ".legacy.lua"):
		return trimScriptExt(filename, ".legacy.luau", ".legacy.lua"), "Script", "Legacy"
	default:
		return trimScriptExt(filename, ".luau", ".lua"), "ModuleScript", ""
	}
}

func trimScriptExt(filename string, exts ...string) string {
	lower := strings.ToLower(filename)
	for _, ext := range exts {
		if strings.HasSuffix(lower, ext) {
			return filename[:len(filename)-len(ext)]
		}
	}
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func studioPluginsDir() string {
	if p, _ := findGlobalSettings(); p != "" {
		if raw, err := os.ReadFile(p); err == nil {
			if vals, err := readStudioProps(raw); err == nil {
				if d := strings.TrimSpace(vals["PluginsDir"]); d != "" {
					if exp := expandStudioPath(d); exp != "" {
						return exp
					}
				}
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(home, "Documents", "Roblox", "Plugins"),
		filepath.Join(home, "Documents", "ROBLOX", "Plugins"),
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			candidates = append(candidates, filepath.Join(local, "Roblox", "Plugins"))
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			return filepath.Join(local, "Roblox", "Plugins")
		}
	}
	return filepath.Join(home, "Documents", "Roblox", "Plugins")
}

func expandStudioPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		s = strings.ReplaceAll(s, "%UserProfile%", home)
		if strings.HasPrefix(s, "~/") {
			s = filepath.Join(home, s[2:])
		}
	}
	if runtime.GOOS == "windows" || isWSL() {
		if local := windowsLocalAppData(); local != "" {
			s = strings.ReplaceAll(s, "%LOCALAPPDATA%", local)
			s = strings.ReplaceAll(s, "%LocalAppData%", local)
		}
	}
	return filepath.FromSlash(s)
}
