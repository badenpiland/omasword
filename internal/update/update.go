// Package update replaces the running omasword binary with the newest
// GitHub release. It never asks for sudo: when the binary's folder is not
// writable by the user, it says so and stops.
package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"omasword/internal/version"
)

const (
	// LatestURL is GitHub's "latest release" endpoint for omasword. It
	// skips drafts and pre-releases.
	LatestURL = "https://api.github.com/repos/badenpiland/omasword/releases/latest"
	// DownloadPrefix is where release assets must live. Asset URLs from the
	// API response are refused unless they start with it.
	DownloadPrefix = "https://github.com/badenpiland/omasword/releases/download/"
	// ChecksumsName is the release asset listing each archive's SHA-256.
	ChecksumsName = "SHA256SUMS"

	maxAPIBody      = 1 << 20  // release JSON
	maxChecksums    = 64 << 10 // SHA256SUMS
	maxArchive      = 64 << 20 // the .tar.gz
	maxBinary       = 128 << 20
	packageManaged  = "/usr/"
	defaultTimeout  = 2 * time.Minute
	tempFilePattern = ".omasword-update-*"
)

// Sentinel errors let the CLI pick an exit code. Wrapped errors carry the
// detail shown to the user.
var (
	// ErrFetch: GitHub could not be reached or answered with an error.
	ErrFetch = errors.New("could not fetch the latest release")
	// ErrUnsupported: no release build exists for this OS or CPU.
	ErrUnsupported = errors.New("no release build for this system")
	// ErrNotWritable: the binary's folder can't be written by this user,
	// or the binary belongs to a system package.
	ErrNotWritable = errors.New("cannot replace the omasword binary")
	// ErrVerify: the download is missing its checksum, doesn't match it,
	// or isn't the expected archive.
	ErrVerify = errors.New("the download failed verification")
)

// Release is the part of GitHub's release object that update uses.
type Release struct {
	Tag    string            // e.g. v0.3.1, already validated
	Assets map[string]string // asset name -> download URL
}

// Updater talks to GitHub. Tests point APIURL and DownloadPrefix at an
// httptest server.
type Updater struct {
	APIURL         string
	DownloadPrefix string
	Client         *http.Client
	Current        string // the running version, e.g. v0.3.0
	GOOS, GOARCH   string
}

// New returns an Updater for the real GitHub repository and this binary.
func New() *Updater {
	return &Updater{
		APIURL:         LatestURL,
		DownloadPrefix: DownloadPrefix,
		Client:         &http.Client{Timeout: defaultTimeout},
		Current:        version.Version,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	}
}

// AssetName is the release archive for this system, or ErrUnsupported.
func (u *Updater) AssetName() (string, error) {
	if u.GOOS != "linux" || (u.GOARCH != "amd64" && u.GOARCH != "arm64") {
		return "", fmt.Errorf("%w: %s/%s (releases are built for linux/amd64 and linux/arm64)", ErrUnsupported, u.GOOS, u.GOARCH)
	}
	return "omasword-linux-" + u.GOARCH + ".tar.gz", nil
}

// Latest asks GitHub for the newest release.
func (u *Updater) Latest(ctx context.Context) (Release, error) {
	body, err := u.get(ctx, u.APIURL, "application/vnd.github+json", maxAPIBody)
	if err != nil {
		return Release{}, err
	}
	var raw struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Release{}, fmt.Errorf("%w: unreadable reply from GitHub", ErrFetch)
	}
	// The tag is printed to the terminal; accept only a plain version.
	if _, err := parseVersion(raw.TagName); err != nil {
		return Release{}, fmt.Errorf("%w: GitHub returned an unexpected release tag", ErrFetch)
	}
	rel := Release{Tag: raw.TagName, Assets: map[string]string{}}
	for _, a := range raw.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

// Newer reports whether release tag latest is newer than current.
func Newer(latest, current string) (bool, error) {
	l, err := parseVersion(latest)
	if err != nil {
		return false, err
	}
	c, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for i := 0; i < 3; i++ {
		if l.num[i] != c.num[i] {
			return l.num[i] > c.num[i], nil
		}
	}
	// Same numbers: a release beats its own pre-release (v1.0.0 > v1.0.0-rc1).
	if l.pre == "" || c.pre == "" {
		return l.pre == "" && c.pre != "", nil
	}
	return l.pre > c.pre, nil
}

type semver struct {
	num [3]int
	pre string
}

var versionRE = regexp.MustCompile(`^v(\d{1,6})\.(\d{1,6})\.(\d{1,6})(?:-([0-9A-Za-z.]{1,32}))?$`)

func parseVersion(s string) (semver, error) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return semver{}, fmt.Errorf("not a version: %q", s)
	}
	var v semver
	for i := 0; i < 3; i++ {
		v.num[i], _ = strconv.Atoi(m[i+1])
	}
	v.pre = m[4]
	return v, nil
}

// Executable is the running binary's real path, with symlinks resolved so
// the file is replaced and not the link.
func Executable() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

// Install downloads rel's archive for this system, checks it against the
// release's SHA256SUMS, and atomically replaces the binary at exe.
func (u *Updater) Install(ctx context.Context, rel Release, exe string) error {
	name, err := u.AssetName()
	if err != nil {
		return err
	}
	exe = filepath.Clean(exe)
	if strings.HasPrefix(exe, packageManaged) {
		return fmt.Errorf("%w: %s belongs to a system package; update it with your package manager", ErrNotWritable, exe)
	}
	dir := filepath.Dir(exe)
	// Find out before downloading whether the folder takes a new file.
	tmp, err := os.CreateTemp(dir, tempFilePattern)
	if err != nil {
		reason := err
		var pe *os.PathError
		if errors.As(err, &pe) {
			reason = pe.Err // leave out the random temp name
		}
		return fmt.Errorf("%w: %s is not writable by you (%v). Reinstall omasword somewhere you own, such as ~/.local/bin", ErrNotWritable, dir, reason)
	}
	tmpName := tmp.Name()
	done := false
	defer func() {
		if !done {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	want, err := u.checksum(ctx, rel, name)
	if err != nil {
		return err
	}
	archiveURL, ok := rel.Assets[name]
	if !ok {
		return fmt.Errorf("%w: release %s has no %s", ErrUnsupported, rel.Tag, name)
	}
	archive, err := u.download(ctx, archiveURL, maxArchive)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != want {
		return fmt.Errorf("%w: %s does not match its SHA-256 in %s", ErrVerify, name, ChecksumsName)
	}
	if err := extractBinary(archive, strings.TrimSuffix(name, ".tar.gz"), tmp); err != nil {
		return err
	}
	if err := tmp.Chmod(0o755); err != nil { // #nosec G302 -- an executable must be executable
		return fmt.Errorf("%w: %v", ErrNotWritable, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("%w: %v", ErrNotWritable, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: %v", ErrNotWritable, err)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("%w: %v", ErrNotWritable, err)
	}
	done = true
	return nil
}

// checksum returns the hex SHA-256 SHA256SUMS lists for name.
func (u *Updater) checksum(ctx context.Context, rel Release, name string) (string, error) {
	url, ok := rel.Assets[ChecksumsName]
	if !ok {
		return "", fmt.Errorf("%w: release %s has no %s file", ErrVerify, rel.Tag, ChecksumsName)
	}
	body, err := u.download(ctx, url, maxChecksums)
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			h := strings.ToLower(f[0])
			if b, err := hex.DecodeString(h); err != nil || len(b) != sha256.Size {
				break
			}
			return h, nil
		}
	}
	return "", fmt.Errorf("%w: %s has no SHA-256 for %s", ErrVerify, ChecksumsName, name)
}

// extractBinary copies the regular file called want out of a .tar.gz.
func extractBinary(archive []byte, want string, dst io.Writer) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("%w: not a gzip archive", ErrVerify)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%w: archive has no %s", ErrVerify, want)
		}
		if err != nil {
			return fmt.Errorf("%w: unreadable archive", ErrVerify)
		}
		if h.Typeflag != tar.TypeReg || filepath.Clean(h.Name) != want {
			continue
		}
		if h.Size <= 0 || h.Size > maxBinary {
			return fmt.Errorf("%w: %s has an unexpected size", ErrVerify, want)
		}
		if _, err := io.CopyN(dst, tr, h.Size); err != nil {
			return fmt.Errorf("%w: %v", ErrNotWritable, err)
		}
		return nil
	}
}

// download fetches a release asset. Only URLs under DownloadPrefix are
// allowed, so the API reply can't send the updater somewhere else.
func (u *Updater) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	if u.DownloadPrefix == "" || !strings.HasPrefix(url, u.DownloadPrefix) {
		return nil, fmt.Errorf("%w: release asset is not on the omasword releases page", ErrVerify)
	}
	return u.get(ctx, url, "application/octet-stream", limit)
}

func (u *Updater) get(ctx context.Context, url, accept string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "omasword/"+u.Current)
	client := u.Client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w: GitHub rate limit reached, try again later (HTTP %d)", ErrFetch, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: not found on GitHub (HTTP 404)", ErrFetch)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: GitHub answered HTTP %d", ErrFetch, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: download is larger than expected", ErrVerify)
	}
	return body, nil
}
