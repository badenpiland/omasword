package update_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"omasword/internal/update"
)

// countingServer answers the latest-release call with tag and counts hits.
func countingServer(t *testing.T, tag string, status int) (*update.Updater, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"` + tag + `","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	return &update.Updater{APIURL: srv.URL, Client: srv.Client(), Current: "v0.3.0", GOOS: "linux", GOARCH: "amd64"}, &hits
}

func TestNoticeChecksAtMostOnceADay(t *testing.T) {
	dir := isolate(t)
	cache := filepath.Join(dir, "cache", "omasword")
	u, hits := countingServer(t, "v0.4.0", http.StatusOK)
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()

	if got := u.Notice(ctx, cache, now); got != "v0.4.0" {
		t.Fatalf("first check = %q", got)
	}
	if got := u.Notice(ctx, cache, now.Add(23*time.Hour)); got != "v0.4.0" || hits.Load() != 1 {
		t.Fatalf("within a day: %q after %d requests, want the cached tag and 1 request", got, hits.Load())
	}
	u.Current = "v0.4.0" // updated in the meantime: the cached tag is no longer news
	if got := u.Notice(ctx, cache, now.Add(23*time.Hour)); got != "" {
		t.Fatalf("after updating: %q", got)
	}
	u.Current = "v0.3.0"
	u.Notice(ctx, cache, now.Add(25*time.Hour))
	if hits.Load() != 2 {
		t.Fatalf("after a day: %d requests, want 2", hits.Load())
	}
	// A clock set backwards doesn't freeze the check forever.
	u.Notice(ctx, cache, now.Add(-48*time.Hour))
	if hits.Load() != 3 {
		t.Fatalf("clock moved back: %d requests, want 3", hits.Load())
	}
	if fi, err := os.Stat(filepath.Join(cache, "update-check.json")); err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache file: %v %v", fi, err)
	}
}

func TestNoticeFailedCheckStillWaitsADay(t *testing.T) {
	dir := isolate(t)
	cache := filepath.Join(dir, "cache", "omasword")
	u, hits := countingServer(t, "", http.StatusServiceUnavailable)
	now := time.Now()
	if got := u.Notice(context.Background(), cache, now); got != "" {
		t.Fatalf("got %q", got)
	}
	u.Notice(context.Background(), cache, now.Add(time.Hour))
	if hits.Load() != 1 {
		t.Fatalf("%d requests after a failure, want 1", hits.Load())
	}
}

func TestNoticeIgnoresOlderAndJunk(t *testing.T) {
	dir := isolate(t)
	cache := filepath.Join(dir, "cache", "omasword")
	u, _ := countingServer(t, "v0.2.0", http.StatusOK)
	if got := u.Notice(context.Background(), cache, time.Now()); got != "" {
		t.Fatalf("older release noticed: %q", got)
	}
	// A hand-edited cache can't put arbitrary text on screen.
	if err := os.WriteFile(filepath.Join(cache, "update-check.json"),
		[]byte(`{"checked_at":"`+time.Now().Format(time.RFC3339)+`","latest":"v9.9.9\u001b[2J"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := u.Notice(context.Background(), cache, time.Now()); got != "" {
		t.Fatalf("junk tag shown: %q", got)
	}
	if got := u.Notice(context.Background(), "", time.Now()); got != "" {
		t.Fatalf("no cache dir: %q", got)
	}
}

func TestOptedOut(t *testing.T) {
	t.Setenv(update.OptOutEnv, "")
	if update.OptedOut(false) {
		t.Fatal("on by default")
	}
	if !update.OptedOut(true) {
		t.Fatal("config opt-out ignored")
	}
	for v, want := range map[string]bool{"1": true, "yes": true, "0": false} {
		t.Setenv(update.OptOutEnv, v)
		if update.OptedOut(false) != want {
			t.Errorf("%s=%q: want %v", update.OptOutEnv, v, want)
		}
	}
}

func TestDefaultCacheDirFollowsXDG(t *testing.T) {
	dir := isolate(t)
	if got := update.DefaultCacheDir(); got != filepath.Join(dir, "cache", "omasword") {
		t.Fatalf("got %q", got)
	}
}
