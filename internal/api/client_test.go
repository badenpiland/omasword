package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testClient retries without real waits and records the waits it would
// have made.
func testClient(waits *[]time.Duration) *Client {
	c := NewClient()
	c.sleep = func(d time.Duration) {
		if waits != nil {
			*waits = append(*waits, d)
		}
	}
	return c
}

func TestGetJSONCapsResponseSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`"`))
		w.Write([]byte(strings.Repeat("a", maxResponseBytes+10)))
		w.Write([]byte(`"`))
	}))
	defer srv.Close()

	var s string
	if err := NewClient().getJSON(srv.URL, &s); err == nil {
		t.Fatal("expected an oversized response to fail")
	}
}

func TestGetJSONErrorBodyIsSanitizedAndCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("upstream\x1b]52;c;evil\x07 down" + strings.Repeat("x", 4096)))
	}))
	defer srv.Close()

	var v any
	err := testClient(nil).getJSON(srv.URL, &v)
	if err == nil {
		t.Fatal("expected an error for a 502")
	}
	msg := err.Error()
	if strings.ContainsAny(msg, "\x1b\x07") {
		t.Fatalf("control characters in error: %q", msg)
	}
	if !strings.Contains(msg, "502") || len(msg) > maxErrorBody+64 {
		t.Fatalf("unexpected error message (%d bytes): %.80q", len(msg), msg)
	}
}

func TestRateLimitIsRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`"ok"`))
	}))
	defer srv.Close()

	var waits []time.Duration
	var s string
	if err := testClient(&waits).getJSON(srv.URL, &s); err != nil || s != "ok" {
		t.Fatalf("got %q, %v; want ok after retries", s, err)
	}
	// The first retry waits the server's 2s (longer than our 1s); the
	// second keeps our own 3s.
	if want := []time.Duration{2 * time.Second, 3 * time.Second}; len(waits) != 2 || waits[0] != want[0] || waits[1] != want[1] {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
}

func TestRetriesGiveUpWithReadableError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("<html>429 Too Many Requests</html>"))
	}))
	defer srv.Close()

	var v any
	err := testClient(nil).getJSON(srv.URL, &v)
	if err == nil || !strings.Contains(err.Error(), "too many requests") || strings.Contains(err.Error(), "<html>") {
		t.Fatalf("err = %v, want a short rate-limit message", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("server saw %d requests, want 3 (1 + 2 retries)", got)
	}
}

func TestLongRetryAfterIsNotWaited(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	var waits []time.Duration
	var v any
	if err := testClient(&waits).getJSON(srv.URL, &v); err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() != 1 || len(waits) != 0 {
		t.Fatalf("calls %d waits %v: an hour-long Retry-After should fail at once", calls.Load(), waits)
	}
}

func TestNotFoundIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	var v any
	if err := testClient(nil).getJSON(srv.URL, &v); err == nil {
		t.Fatal("expected an error")
	}
	if calls.Load() != 1 {
		t.Fatalf("a 404 was requested %d times, want 1", calls.Load())
	}
}

func TestRetriedPostResendsBody(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`"` + string(body) + `"`))
	}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("payload"))
	var s string
	if err := testClient(nil).doJSON(req, &s); err != nil || s != "payload" {
		t.Fatalf("got %q, %v; want the body resent on retry", s, err)
	}
}
