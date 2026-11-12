package update_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"omasword/internal/update"
	"omasword/internal/update/updatetest"
)

// isolate keeps every test away from the real home, config, and cache.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	return dir
}

func oldBinary(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(bin, "omasword")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func assertOnlyBinary(t *testing.T, exe, want string) {
	t.Helper()
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("binary = %q, want %q", got, want)
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	if len(entries) != 1 {
		t.Fatalf("left files beside the binary: %v", entries)
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.3.1", "v0.3.0", true},
		{"v0.10.0", "v0.9.9", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.3.0", "v0.3.0", false},
		{"v0.2.9", "v0.3.0", false},
		{"v1.0.0", "v1.0.0-rc1", true},
		{"v1.0.0-rc1", "v1.0.0", false},
	}
	for _, c := range cases {
		got, err := update.Newer(c.latest, c.current)
		if err != nil || got != c.want {
			t.Errorf("Newer(%s, %s) = %v, %v; want %v", c.latest, c.current, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "0.3.0", "v0.3", "v0.3.0\x1b[2J", "latest"} {
		if _, err := update.Newer(bad, "v0.3.0"); err == nil {
			t.Errorf("Newer(%q) accepted a bad tag", bad)
		}
	}
}

func TestInstallReplacesBinaryAtomically(t *testing.T) {
	dir := isolate(t)
	exe := oldBinary(t, dir)
	u := updatetest.Serve(t, updatetest.Release{Binary: []byte("new")})
	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v9.9.9" {
		t.Fatalf("tag = %q", rel.Tag)
	}
	if err := u.Install(context.Background(), rel, exe); err != nil {
		t.Fatal(err)
	}
	assertOnlyBinary(t, exe, "new")
	if fi, _ := os.Stat(exe); fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v", fi.Mode())
	}
}

func TestInstallArm64(t *testing.T) {
	dir := isolate(t)
	exe := oldBinary(t, dir)
	u := updatetest.Serve(t, updatetest.Release{Binary: []byte("arm"), Arch: "arm64"})
	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Install(context.Background(), rel, exe); err != nil {
		t.Fatal(err)
	}
	assertOnlyBinary(t, exe, "arm")
}

func TestInstallRefusesBadDownloads(t *testing.T) {
	wrong := "0000000000000000000000000000000000000000000000000000000000000000  omasword-linux-amd64.tar.gz\n"
	none := ""
	other := "0000000000000000000000000000000000000000000000000000000000000000  something-else.tar.gz\n"
	cases := map[string]updatetest.Release{
		"checksum mismatch":   {Checksums: &wrong},
		"no checksums file":   {Checksums: &none},
		"no line for archive": {Checksums: &other},
		"asset elsewhere":     {AssetBase: "https://example.invalid/dl/"},
		"not an archive":      {ArchiveRaw: []byte("not gzip")},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			dir := isolate(t)
			exe := oldBinary(t, dir)
			r.Binary = []byte("new")
			u := updatetest.Serve(t, r)
			rel, err := u.Latest(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			err = u.Install(context.Background(), rel, exe)
			if !errors.Is(err, update.ErrVerify) {
				t.Fatalf("err = %v, want ErrVerify", err)
			}
			assertOnlyBinary(t, exe, "old")
		})
	}
}

func TestInstallArchiveWithoutBinary(t *testing.T) {
	dir := isolate(t)
	exe := oldBinary(t, dir)
	archive := updatetest.TarGz(t, "omasword-linux-arm64", []byte("wrong arch"))
	u := updatetest.Serve(t, updatetest.Release{ArchiveRaw: archive})
	rel, _ := u.Latest(context.Background())
	if err := u.Install(context.Background(), rel, exe); !errors.Is(err, update.ErrVerify) {
		t.Fatalf("err = %v, want ErrVerify", err)
	}
	assertOnlyBinary(t, exe, "old")
}

func TestInstallNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := isolate(t)
	exe := oldBinary(t, dir)
	bin := filepath.Dir(exe)
	if err := os.Chmod(bin, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })
	u := updatetest.Serve(t, updatetest.Release{Binary: []byte("new")})
	rel, _ := u.Latest(context.Background())
	if err := u.Install(context.Background(), rel, exe); !errors.Is(err, update.ErrNotWritable) {
		t.Fatalf("err = %v, want ErrNotWritable", err)
	}
	assertOnlyBinary(t, exe, "old")
}

func TestInstallRefusesSystemPackage(t *testing.T) {
	isolate(t)
	u := updatetest.Serve(t, updatetest.Release{Binary: []byte("new")})
	rel, _ := u.Latest(context.Background())
	if err := u.Install(context.Background(), rel, "/usr/bin/omasword"); !errors.Is(err, update.ErrNotWritable) {
		t.Fatalf("err = %v, want ErrNotWritable", err)
	}
}

func TestUnsupportedSystem(t *testing.T) {
	u := &update.Updater{GOOS: "darwin", GOARCH: "arm64"}
	if _, err := u.AssetName(); !errors.Is(err, update.ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
	u = &update.Updater{GOOS: "linux", GOARCH: "386"}
	if _, err := u.AssetName(); !errors.Is(err, update.ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
}

func TestLatestErrors(t *testing.T) {
	isolate(t)
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		u := updatetest.Serve(t, updatetest.Release{APIStatus: status})
		if _, err := u.Latest(context.Background()); !errors.Is(err, update.ErrFetch) {
			t.Errorf("HTTP %d: err = %v, want ErrFetch", status, err)
		}
	}
	u := updatetest.Serve(t, updatetest.Release{Tag: "v1.0.0\x1b]0;evil\x07"})
	if _, err := u.Latest(context.Background()); !errors.Is(err, update.ErrFetch) {
		t.Errorf("control characters in tag: err = %v, want ErrFetch", err)
	}
}
