package main

import (
	"strings"
	"testing"
)

const sampleGlobalSettings = `<?xml version="1.0" encoding="utf-8"?>
<roblox xmlns:xmime="http://www.w3.org/2005/05/xmlmime" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="http://www.roblox.com/roblox.xsd" version="4">
	<External>null</External>
	<Item class="GameSettings" referent="RBXGAME">
		<Properties>
			<string name="Name">Game Options</string>
		</Properties>
	</Item>
	<Item class="Studio" referent="RBXSTUDIO">
		<Properties>
			<bool name="Always Save Script Changes">false</bool>
			<token name="ActionOnStopSync">0</token>
			<Color3 name="Warning Color">
				<R>0</R>
				<G>0</G>
				<B>1</B>
			</Color3>
		</Properties>
	</Item>
</roblox>
`

func TestPatchStudioSettingsInsertAndReplace(t *testing.T) {
	out, changed, err := patchStudioSettings([]byte(sampleGlobalSettings), scriptSyncPrefs)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changes")
	}
	s := string(out)
	if !strings.Contains(s, `<bool name="Always Save Script Changes">false</bool>`) {
		t.Fatal("must leave unrelated properties alone")
	}
	if !strings.Contains(s, `<token name="ActionOnStopSync">1</token>`) {
		t.Fatalf("should replace AlwaysAsk with KeepLocalFiles:\n%s", s)
	}
	if strings.Contains(s, `<token name="ActionOnStopSync">0</token>`) {
		t.Fatal("old ActionOnStopSync value still present")
	}
	if !strings.Contains(s, `<bool name="AutoResumeSyncOnPlaceOpen">true</bool>`) {
		t.Fatal("missing AutoResumeSyncOnPlaceOpen")
	}
	if !strings.Contains(s, `<token name="ActionOnAutoResumeSync">2</token>`) {
		t.Fatal("missing ActionOnAutoResumeSync KeepLocal")
	}
	if !strings.Contains(s, `<token name="DefaultScriptSyncFileType">1</token>`) {
		t.Fatal("missing DefaultScriptSyncFileType Luau")
	}
	if !strings.Contains(s, "<R>0</R>") || !strings.Contains(s, `class="GameSettings"`) {
		t.Fatal("must not disturb nested Color3 or other items")
	}
	// Closing tag indent preserved (two tabs in the fixture).
	if !strings.Contains(s, "\t\t</Properties>\n\t</Item>") {
		t.Fatalf("</Properties> indent changed:\n%s", s)
	}

	again, changed2, err := patchStudioSettings(out, scriptSyncPrefs)
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Fatalf("second patch should be a no-op:\n%s", again)
	}
}

func TestReadStudioProps(t *testing.T) {
	vals, err := readStudioProps([]byte(sampleGlobalSettings))
	if err != nil {
		t.Fatal(err)
	}
	if vals["ActionOnStopSync"] != "0" {
		t.Fatalf("got %#v", vals)
	}
	if _, ok := vals["AutoResumeSyncOnPlaceOpen"]; ok {
		t.Fatal("should not invent missing props")
	}
}

func TestPatchStudioSettingsCRLF(t *testing.T) {
	in := strings.ReplaceAll(sampleGlobalSettings, "\n", "\r\n")
	out, changed, err := patchStudioSettings([]byte(in), scriptSyncPrefs)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changes")
	}
	s := string(out)
	if !strings.Contains(s, "\r\n") {
		t.Fatal("should keep CRLF")
	}
	if !strings.Contains(s, `<bool name="AutoResumeSyncOnPlaceOpen">true</bool>`) {
		t.Fatal(s)
	}
}

func TestAllPatchPrefsIncludesReload(t *testing.T) {
	found := false
	for _, p := range allPatchPrefs() {
		if p.Name == "ReloadLocalPluginsOnChange" && p.Want == "true" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ReloadLocalPluginsOnChange in patch set")
	}
}

func TestParseArgsPrefs(t *testing.T) {
	show := parseArgs([]string{"prefs"})
	if show.cmd != "prefs" || show.prefsAction != "" {
		t.Fatalf("got %+v", show)
	}
	apply := parseArgs([]string{"prefs", "apply"})
	if apply.cmd != "prefs" || apply.prefsAction != "apply" {
		t.Fatalf("got %+v", apply)
	}
	scan := parseArgs([]string{"scan"})
	if scan.cmd != "scan" {
		t.Fatalf("got %+v", scan)
	}
}

func TestPrefsNeedApply(t *testing.T) {
	if !prefsNeedApply(map[string]string{}) {
		t.Fatal("empty should need apply")
	}
	good := map[string]string{}
	for _, p := range scriptSyncPrefs {
		good[p.Name] = p.Want
	}
	if prefsNeedApply(good) {
		t.Fatal("matching map should not need apply")
	}
}

func TestFindGlobalSettingsEnv(t *testing.T) {
	t.Setenv("ROBUILD_GLOBAL_SETTINGS", "/tmp/studio-settings.xml")
	p, all := findGlobalSettings()
	if p != "/tmp/studio-settings.xml" || len(all) != 1 || all[0] != p {
		t.Fatalf("got %q %#v", p, all)
	}
}
