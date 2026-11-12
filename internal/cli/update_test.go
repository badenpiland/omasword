package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omasword/internal/update"
	"omasword/internal/update/updatetest"
)

// fakeUpdate points `omasword update` at a fake release and a temporary
// binary, with HOME and XDG dirs isolated.
func fakeUpdate(t *testing.T, r updatetest.Release, current string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(bin, "omasword")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := updatetest.Serve(t, r)
	u.Current = current
	oldNew, oldExe := newUpdater, executable
	newUpdater = func() *update.Updater { return u }
	executable = func() (string, error) { return exe, nil }
	t.Cleanup(func() { newUpdater, executable = oldNew, oldExe })
	return exe
}

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := RunWith(&out, &errb, args)
	return code, out.String(), errb.String()
}

func TestUpdateIsCLI(t *testing.T) {
	if !WantsCLI([]string{"update"}) || !WantsCLI([]string{"update", "--check"}) {
		t.Fatal("update should not start the reader")
	}
}

func TestUpdateCheck(t *testing.T) {
	exe := fakeUpdate(t, updatetest.Release{Tag: "v0.4.0", Binary: []byte("new")}, "v0.3.0")
	code, out, _ := runCLI("update", "--check")
	if code != ExitUpdateAvailable || !strings.Contains(out, "v0.4.0") {
		t.Fatalf("code %d, out %q", code, out)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatal("--check changed the binary")
	}

	fakeUpdate(t, updatetest.Release{Tag: "v0.3.0"}, "v0.3.0")
	code, out, _ = runCLI("update", "--check")
	if code != ExitOK || !strings.Contains(out, "up to date") {
		t.Fatalf("code %d, out %q", code, out)
	}
}

func TestUpdateInstalls(t *testing.T) {
	exe := fakeUpdate(t, updatetest.Release{Tag: "v0.4.0", Binary: []byte("new")}, "v0.3.0")
	code, out, errOut := runCLI("update")
	if code != ExitOK || !strings.Contains(out, "v0.4.0") {
		t.Fatalf("code %d, out %q, err %q", code, out, errOut)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Fatalf("binary = %q", b)
	}
	// Already current: nothing to do.
	fakeUpdate(t, updatetest.Release{Tag: "v0.4.0"}, "v0.4.0")
	if code, out, _ := runCLI("update"); code != ExitOK || !strings.Contains(out, "up to date") {
		t.Fatalf("code %d, out %q", code, out)
	}
}

func TestUpdateFailures(t *testing.T) {
	bad := "0000000000000000000000000000000000000000000000000000000000000000  omasword-linux-amd64.tar.gz\n"
	exe := fakeUpdate(t, updatetest.Release{Tag: "v0.4.0", Binary: []byte("new"), Checksums: &bad}, "v0.3.0")
	if code, _, errOut := runCLI("update"); code != ExitUpdateFailed || errOut == "" {
		t.Fatalf("checksum mismatch: code %d", code)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatal("a failed update changed the binary")
	}

	fakeUpdate(t, updatetest.Release{APIStatus: 503}, "v0.3.0")
	if code, _, _ := runCLI("update", "--check"); code != ExitFetch {
		t.Fatalf("GitHub down: code %d, want %d", code, ExitFetch)
	}

	if os.Geteuid() != 0 {
		exe = fakeUpdate(t, updatetest.Release{Tag: "v0.4.0", Binary: []byte("new")}, "v0.3.0")
		bin := filepath.Dir(exe)
		if err := os.Chmod(bin, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })
		code, _, errOut := runCLI("update")
		if code != ExitUpdateFailed || !strings.Contains(errOut, "not writable") || strings.Contains(errOut, "sudo") {
			t.Fatalf("not writable: code %d, err %q", code, errOut)
		}
	}

	for _, args := range [][]string{{"update", "now"}, {"update", "--bogus"}} {
		if code, _, _ := runCLI(args...); code != ExitUsage {
			t.Errorf("%v: code %d, want %d", args, code, ExitUsage)
		}
	}
}
