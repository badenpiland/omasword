package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	err := NewClient().getJSON(srv.URL, &v)
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
