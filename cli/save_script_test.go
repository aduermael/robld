package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStudioSaveToFileScript(t *testing.T) {
	src := studioSaveToFileScript(4242)
	if !strings.Contains(src, `menu item "Save to File"`) {
		t.Fatal("save script must click File → Save to File")
	}
	if !strings.Contains(src, `menu "File"`) {
		t.Fatal("save script must target the File menu")
	}
	if !strings.Contains(src, "delay") {
		t.Fatal("save script must delay before Enter")
	}
	if !strings.Contains(src, "key code 36") && !strings.Contains(strings.ToLower(src), "return") {
		t.Fatal("save script must send delayed Enter/return")
	}
	if strings.Contains(src, `keystroke "s" using command down`) {
		t.Fatal("Save to File script must not send Cmd+S; that is the fallback")
	}
	if strings.Contains(src, `menu item "Save"`) && !strings.Contains(src, `menu item "Save to File"`) {
		t.Fatal("must not click a menu item named Save")
	}
	if !strings.Contains(src, "4242") {
		t.Fatal("script should target the Studio pid")
	}
	if studioAppleEventTimeout < 2*time.Second {
		t.Fatalf("Apple Event timeout too short: %s", studioAppleEventTimeout)
	}
}

func TestStudioSaveCmdSScriptIsFallback(t *testing.T) {
	src := studioSaveCmdSScript(7)
	if !strings.Contains(src, `keystroke "s" using command down`) {
		t.Fatal("fallback script must send Cmd+S")
	}
	if strings.Contains(src, `menu item "Save to File"`) {
		t.Fatal("Cmd+S fallback is a separate script")
	}
}

func TestStudioReturnScript(t *testing.T) {
	src := studioReturnScript(9)
	if !strings.Contains(src, "key code 36") && !strings.Contains(strings.ToLower(src), "return") {
		t.Fatal("return script must send Enter/return")
	}
}

func TestSaveDialogNeedUserMessage(t *testing.T) {
	msg := saveDialogNeedUserMessage()
	for _, part := range []string{"NEED_USER:", "Save / Don't Save / Cancel", "Save"} {
		if !strings.Contains(msg, part) {
			t.Fatalf("missing %q in %q", part, msg)
		}
	}
}

func TestWaitPlaceChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "place.rbxlx")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if waitPlaceChanged(path, st.ModTime(), st.Size(), 0) {
		t.Fatal("unchanged file must not report a change")
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("after-save"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !waitPlaceChanged(path, st.ModTime(), st.Size(), time.Second) {
		t.Fatal("mtime/size change should be detected")
	}
}
