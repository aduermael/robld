package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriteVersionContainsInjectedVersion(t *testing.T) {
	old := version
	version = "v3.1.4-teststamp"
	defer func() { version = old }()
	var buf bytes.Buffer
	writeVersion(&buf)
	out := buf.String()
	if !strings.Contains(out, version) {
		t.Fatalf("version output %q does not contain %q", out, version)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("version output empty")
	}
}

func TestUsageMentionsInstallVersionUpdate(t *testing.T) {
	u := usage()
	for _, s := range []string{"--install", "--version", "--update"} {
		if !strings.Contains(u, s) {
			t.Fatalf("usage missing %s", s)
		}
	}
}

type fakeGitHub struct {
	tag        string
	asset      []byte
	fetches    atomic.Int32
	downloads  atomic.Int32
	failLatest bool
}

func (f *fakeGitHub) start(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	assetName := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		f.fetches.Add(1)
		if f.failLatest {
			http.Error(w, "nope", http.StatusBadGateway)
			return
		}
		payload := map[string]any{
			"tag_name": f.tag,
			"assets": []map[string]string{{
				"name":                 assetName,
				"browser_download_url": "http://" + r.Host + "/asset",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		f.downloads.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(f.asset)
	})
	return httptest.NewServer(mux)
}

func testUpdater(t *testing.T, srv *httptest.Server, now *time.Time) *updater {
	t.Helper()
	dir := t.TempDir()
	if now == nil {
		n := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		now = &n
	}
	return &updater{
		Version:     "v1.0.0",
		Now:         func() time.Time { return *now },
		Client:      srv.Client(),
		ReleasesURL: srv.URL + "/releases/latest",
		CachePath:   filepath.Join(dir, "cache", "update-check.json"),
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		Stdout:      io.Discard,
		Stderr:      io.Discard,
		DestRoot:    dir,
	}
}

func TestNoticeWhenLatestIsNewer(t *testing.T) {
	gh := &fakeGitHub{tag: "v1.2.0", asset: []byte("x")}
	srv := gh.start(t)
	defer srv.Close()
	u := testUpdater(t, srv, nil)
	u.Version = "v1.0.0"

	var buf bytes.Buffer
	maybePrintUpdateNotice([]string{"--help"}, u, &buf)
	out := buf.String()
	if !strings.Contains(out, "v1.2.0") {
		t.Fatalf("missing new version in %q", out)
	}
	if !strings.Contains(out, "v1.0.0") {
		t.Fatalf("missing current version in %q", out)
	}
	if !strings.Contains(out, "robld --update") {
		t.Fatalf("missing %q in %q", "robld --update", out)
	}
	if gh.fetches.Load() != 1 {
		t.Fatalf("fetches = %d", gh.fetches.Load())
	}
}

func TestNoticeAbsentWhenLatestEqualOrOlder(t *testing.T) {
	for _, tag := range []string{"v1.0.0", "v0.9.0"} {
		gh := &fakeGitHub{tag: tag, asset: []byte("x")}
		srv := gh.start(t)
		u := testUpdater(t, srv, nil)
		u.Version = "v1.0.0"
		var buf bytes.Buffer
		maybePrintUpdateNotice([]string{"--version"}, u, &buf)
		srv.Close()
		out := buf.String()
		if out != "" {
			t.Errorf("tag %s: unexpected notice %q", tag, out)
		}
		if strings.Contains(out, "robld --update") {
			t.Errorf("tag %s: nag should be absent", tag)
		}
	}
}

func TestNoticeSkippedForUpdateCommand(t *testing.T) {
	gh := &fakeGitHub{tag: "v9.9.9", asset: []byte("x")}
	srv := gh.start(t)
	defer srv.Close()
	u := testUpdater(t, srv, nil)
	var buf bytes.Buffer
	maybePrintUpdateNotice([]string{"--update"}, u, &buf)
	if buf.Len() != 0 {
		t.Fatalf("update cmd should not nag: %q", buf.String())
	}
	if gh.fetches.Load() != 0 {
		t.Fatalf("update cmd should not fetch for nag, fetches=%d", gh.fetches.Load())
	}
}

func TestGitHubCheckAtMostOncePerHour(t *testing.T) {
	gh := &fakeGitHub{tag: "v2.0.0", asset: []byte("x")}
	srv := gh.start(t)
	defer srv.Close()
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	u := testUpdater(t, srv, &now)
	u.Version = "v1.0.0"

	n1 := u.Notice()
	if !strings.Contains(n1, "robld --update") {
		t.Fatalf("first notice: %q", n1)
	}
	if gh.fetches.Load() != 1 {
		t.Fatalf("after first: fetches=%d", gh.fetches.Load())
	}

	now = now.Add(30 * time.Minute)
	n2 := u.Notice()
	if n2 != n1 {
		t.Fatalf("cached notice changed: %q vs %q", n2, n1)
	}
	if gh.fetches.Load() != 1 {
		t.Fatalf("second check inside hour fetched again: fetches=%d", gh.fetches.Load())
	}

	now = now.Add(31 * time.Minute) // 61 minutes from first
	_ = u.Notice()
	if gh.fetches.Load() != 2 {
		t.Fatalf("after an hour should fetch again: fetches=%d", gh.fetches.Load())
	}
}

func TestNoticeSurvivesFailedGitHub(t *testing.T) {
	gh := &fakeGitHub{tag: "v2.0.0", asset: []byte("x"), failLatest: true}
	srv := gh.start(t)
	defer srv.Close()
	u := testUpdater(t, srv, nil)
	var buf bytes.Buffer
	maybePrintUpdateNotice([]string{"--help"}, u, &buf)
	if buf.Len() != 0 {
		t.Fatalf("failed check must not nag: %q", buf.String())
	}
}

func TestUpdateReplacesBinaryAndInstallsSkill(t *testing.T) {
	script := "#!/bin/sh\n" +
		"set -e\n" +
		"if [ \"$1\" = \"--install\" ]; then\n" +
		"  mkdir -p .claude/skills/robld .grok/skills/robld .agents/skills/robld .cursor/skills/robld .codex/skills/robld\n" +
		"  for d in .claude .grok .agents .cursor .codex; do\n" +
		"    printf 'skill-from-new-binary\\n' > \"$d/skills/robld/SKILL.md\"\n" +
		"  done\n" +
		"  echo READY: installed\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo fake-robld\n"
	gh := &fakeGitHub{tag: "v9.9.9", asset: []byte(script)}
	srv := gh.start(t)
	defer srv.Close()

	u := testUpdater(t, srv, nil)
	dest := u.DestRoot
	exe := filepath.Join(dest, "robld-bin")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	u.Executable = exe
	var out bytes.Buffer
	u.Stdout = &out
	u.Stderr = &out

	if err := u.Update(); err != nil {
		t.Fatalf("Update: %v\n%s", err, out.String())
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != script {
		t.Fatalf("binary not replaced: got %d bytes want %d", len(got), len(script))
	}
	if gh.downloads.Load() != 1 {
		t.Fatalf("downloads=%d", gh.downloads.Load())
	}
	skillPath := filepath.Join(dest, ".claude/skills/robld/SKILL.md")
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("skill not written: %v", err)
	}
	if !strings.Contains(string(raw), "skill-from-new-binary") {
		t.Fatalf("new binary --install did not write skill: %q", raw)
	}
	for _, rel := range skillInstallPaths {
		p := filepath.Join(dest, rel)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", rel, err)
			continue
		}
		if string(b) != "skill-from-new-binary\n" {
			t.Errorf("%s: %q", rel, b)
		}
	}
}
