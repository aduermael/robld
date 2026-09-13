package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	githubLatestReleaseURL = "https://api.github.com/repos/aduermael/robld/releases/latest"
	updateCheckInterval    = time.Hour
	updateNoticeExact      = "robld --update"
)

// updater is the shipped GitHub-check / self-replace path. Tests inject
// HTTP, clock, cache file, dest dir, executable path, and version.
type updater struct {
	Version     string
	Now         func() time.Time
	Client      *http.Client
	ReleasesURL string
	CachePath   string
	GOOS        string
	GOARCH      string
	Executable  string
	DestRoot    string
	Stdout      io.Writer
	Stderr      io.Writer
}

type updateCache struct {
	CheckedAt time.Time `json:"checkedAt"`
	Latest    string    `json:"latest"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func defaultUpdater() *updater {
	u := &updater{
		Version:     version,
		Now:         time.Now,
		Client:      &http.Client{Timeout: 60 * time.Second},
		ReleasesURL: githubLatestReleaseURL,
		CachePath:   defaultUpdateCachePath(),
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}
	if v := strings.TrimSpace(os.Getenv("ROBLD_RELEASES_URL")); v != "" {
		u.ReleasesURL = v
	}
	if v := strings.TrimSpace(os.Getenv("ROBLD_CACHE_DIR")); v != "" {
		u.CachePath = filepath.Join(v, "update-check.json")
	}
	return u
}

func defaultUpdateCachePath() string {
	if v := strings.TrimSpace(os.Getenv("ROBLD_CACHE_DIR")); v != "" {
		return filepath.Join(v, "update-check.json")
	}
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "robld", "update-check.json")
}

func skipUpdateNotice(args []string) bool {
	for _, a := range args {
		if a == "--update" || a == "update" {
			return true
		}
	}
	return false
}

// maybePrintUpdateNotice writes the nag to w when GitHub reports a newer tag.
// A failed check never returns an error to the caller; it just skips the note.
func maybePrintUpdateNotice(args []string, u *updater, w io.Writer) {
	if u == nil || skipUpdateNotice(args) {
		return
	}
	n := u.Notice()
	if n == "" {
		return
	}
	fmt.Fprintln(w, n)
}

func formatUpdateNotice(latest, current string) string {
	return fmt.Sprintf(
		"NOTE: a new robld version is available (%s; this binary is %s). Run %s to install it.",
		latest, current, updateNoticeExact,
	)
}

func (u *updater) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

func (u *updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return http.DefaultClient
}

func (u *updater) stdout() io.Writer {
	if u.Stdout != nil {
		return u.Stdout
	}
	return os.Stdout
}

func (u *updater) stderr() io.Writer {
	if u.Stderr != nil {
		return u.Stderr
	}
	return os.Stderr
}

func (u *updater) goos() string {
	if u.GOOS != "" {
		return u.GOOS
	}
	return runtime.GOOS
}

func (u *updater) goarch() string {
	if u.GOARCH != "" {
		return u.GOARCH
	}
	return runtime.GOARCH
}

func (u *updater) version() string {
	if u.Version != "" {
		return u.Version
	}
	return version
}

func (u *updater) releasesURL() string {
	if u.ReleasesURL != "" {
		return u.ReleasesURL
	}
	return githubLatestReleaseURL
}

// Notice returns the nag string when latest is newer than this binary, else "".
func (u *updater) Notice() string {
	latest, err := u.cachedOrFetchLatest()
	if err != nil || latest == "" {
		return ""
	}
	cur := u.version()
	if !versionNewer(latest, cur) {
		return ""
	}
	return formatUpdateNotice(latest, cur)
}

func (u *updater) cachedOrFetchLatest() (string, error) {
	now := u.now()
	c := u.readCache()
	if c.Latest != "" && !c.CheckedAt.IsZero() && now.Sub(c.CheckedAt) < updateCheckInterval {
		return c.Latest, nil
	}
	rel, err := u.fetchRelease()
	if err != nil {
		if c.Latest != "" {
			c.CheckedAt = now
			_ = u.writeCache(c)
			return c.Latest, nil
		}
		c.CheckedAt = now
		_ = u.writeCache(c)
		return "", err
	}
	c.Latest = rel.TagName
	c.CheckedAt = now
	_ = u.writeCache(c)
	return c.Latest, nil
}

func (u *updater) readCache() updateCache {
	var c updateCache
	if u.CachePath == "" {
		return c
	}
	b, err := os.ReadFile(u.CachePath)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	return c
}

func (u *updater) writeCache(c updateCache) error {
	if u.CachePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(u.CachePath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(u.CachePath, append(b, '\n'), 0o644)
}

func (u *updater) fetchRelease() (*githubRelease, error) {
	url := u.releasesURL()
	if url == "" {
		return nil, fmt.Errorf("no releases URL")
	}
	body, err := u.httpGet(url, 12*time.Second)
	if err != nil {
		return nil, err
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, fmt.Errorf("latest release missing tag_name")
	}
	return &rel, nil
}

func (u *updater) httpGet(url string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "robld/"+u.version())
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return body, nil
}

func releaseAssetName(goos, goarch string) string {
	name := "robld-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func versionNewer(latest, current string) bool {
	lMaj, lMin, lPat, lOK := parseVersion(latest)
	if !lOK {
		return false
	}
	cMaj, cMin, cPat, cOK := parseVersion(current)
	if !cOK {
		return true
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPat > cPat
}

func parseVersion(s string) (maj, min, pat int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if s == "" || strings.EqualFold(s, "dev") {
		return 0, 0, 0, false
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	var parts []string
	for _, p := range strings.Split(s, ".") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return 0, 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[0], "%d", &maj); err != nil {
		return 0, 0, 0, false
	}
	if len(parts) > 1 {
		if _, err := fmt.Sscanf(parts[1], "%d", &min); err != nil {
			return 0, 0, 0, false
		}
	}
	if len(parts) > 2 {
		if _, err := fmt.Sscanf(parts[2], "%d", &pat); err != nil {
			return 0, 0, 0, false
		}
	}
	return maj, min, pat, true
}

// Update downloads the latest release asset for this OS/arch, replaces the
// target binary, then runs that new binary's --install so the skill matches.
func (u *updater) Update() error {
	rel, err := u.fetchRelease()
	if err != nil {
		return err
	}
	_ = u.writeCache(updateCache{CheckedAt: u.now(), Latest: rel.TagName})

	want := releaseAssetName(u.goos(), u.goarch())
	var assetURL string
	for _, a := range rel.Assets {
		if a.Name == want {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("latest release %s has no asset %s", rel.TagName, want)
	}
	data, err := u.httpGet(assetURL, 60*time.Second)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("empty download for %s", want)
	}
	exe := u.Executable
	if exe == "" {
		exe, err = os.Executable()
		if err != nil {
			return err
		}
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
	}
	if err := replaceExecutable(exe, data); err != nil {
		return err
	}
	dest := u.DestRoot
	if dest == "" {
		dest, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(u.stdout(), "Updated to %s\n", rel.TagName)
	cmd := exec.Command(exe, "--install")
	cmd.Dir = dest
	cmd.Stdout = u.stdout()
	cmd.Stderr = u.stderr()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updated binary but robld --install failed: %w", err)
	}
	return nil
}

func replaceExecutable(path string, data []byte) error {
	if path == "" {
		return fmt.Errorf("empty executable path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".robld-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		bak := path + ".old"
		_ = os.Remove(bak)
		if _, err := os.Stat(path); err == nil {
			if err := os.Rename(path, bak); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func runUpdate() {
	u := defaultUpdater()
	u.DestRoot = root
	exe, err := os.Executable()
	if err != nil {
		fail(exitError, "ERROR: could not locate this binary: %v", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	u.Executable = exe
	if err := u.Update(); err != nil {
		fail(exitError, "ERROR: update failed: %v", err)
	}
}
