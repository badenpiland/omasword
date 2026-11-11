package esv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleHTML = `<p><b class="verse-num" id="v31001001-1">1&nbsp;</b>The vision of Obadiah.</p>` +
	`<p><b class="verse-num">2&nbsp;</b>Behold, I will make you small.</p>` +
	`<p><b class="verse-num">3&nbsp;</b>The pride of your heart has deceived you.</p>` +
	`<p><b class="verse-num">4&nbsp;</b>Though you soar aloft like the eagle.</p>` +
	`<p><b class="verse-num">5&nbsp;</b>If thieves came to you.</p>` +
	`<p><b class="verse-num">6&nbsp;</b>How you have been pillaged.</p>` +
	`<p><b class="verse-num">7&nbsp;</b>All your allies have driven you.</p>` +
	`<p><b class="verse-num">8&nbsp;</b>Will I not on that day.</p>` +
	`<p><b class="verse-num">9&nbsp;</b>And your mighty men shall be dismayed.</p>` +
	`<p><b class="verse-num">10&nbsp;</b>Because of the violence done to your brother.</p>` +
	`<p><b class="verse-num">11&nbsp;</b>On the day that you stood aloof.</p>` +
	`<p>Scripture quotations are from the ESV® Bible, © 2001 by Crossway. Used by permission. All rights reserved.</p>`

func TestChapterClipsCachesAndHidesTheKey(t *testing.T) {
	const key = "test-key-not-real"
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if got := r.Header.Get("Authorization"); got != "Token "+key {
			t.Errorf("Authorization = %q", got)
		}
		if strings.Contains(r.URL.String(), key) {
			t.Errorf("key leaked into the URL: %s", r.URL)
		}
		if !strings.Contains(r.URL.Path, "/v3/passage/html/") {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprintf(w, `{"passages":[%q]}`, sampleHTML)
	}))
	defer srv.Close()

	c, err := New(key, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	c.now = func() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

	page, err := c.Chapter(31, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Verses) != MaxVerses(31) {
		t.Fatalf("showed %d verses, cap is %d", len(page.Verses), MaxVerses(31))
	}
	if !page.MoreAfter || page.MoreBefore {
		t.Fatalf("page flags before=%v after=%v", page.MoreBefore, page.MoreAfter)
	}
	if !strings.Contains(page.Notice, "Crossway") || !strings.Contains(Attribution(page.Notice), "esv.org") {
		t.Fatalf("notice = %q", page.Notice)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1", hits)
	}

	again, err := c.Chapter(31, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("cached chapter called the API again (%d hits)", hits)
	}
	if len(again.Verses) != len(page.Verses) {
		t.Fatalf("cached page len %d", len(again.Verses))
	}

	next, err := c.Chapter(31, 1, page.Last+1)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("the next page should fetch once more, hits=%d", hits)
	}
	if next.First != page.Last+1 || !next.MoreBefore {
		t.Fatalf("next page = %+v", next)
	}
	// Both pages together would be more than half of Obadiah, so the
	// first page is gone.
	if _, _, _, ok := c.cache.Get(31, 1, 1); ok {
		t.Fatal("first Obadiah page was kept past the half-book cap")
	}
}

func TestRejectedKeyIsRedacted(t *testing.T) {
	const key = "test-key-not-real"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "bad token %s", key)
	}))
	defer srv.Close()

	c, err := New(key, "")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	_, err = c.Chapter(43, 11, 1)
	if err == nil {
		t.Fatal("expected a rejected key")
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("error contains the key: %v", err)
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v", err)
	}
}

func TestServerRateLimitDoesNotIncludeTheKey(t *testing.T) {
	const key = "test-key-not-real"
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "%s", key)
	}))
	defer srv.Close()

	c, err := New(key, "")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	_, err = c.Chapter(43, 11, 1)
	if err == nil || strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("err = %v", err)
	}
	_, err = c.Chapter(43, 11, 1)
	if err == nil || hits != 1 {
		t.Fatalf("penalty did not stick: hits=%d err=%v", hits, err)
	}
}

func TestNoKeyAndEnvOverride(t *testing.T) {
	c, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Chapter(43, 3, 1); err != ErrNoKey {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchClipsAShortBook(t *testing.T) {
	const key = "test-key-not-real"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"page":1,"total_results":12,"results":[`))
		for i := 1; i <= 12; i++ {
			if i > 1 {
				w.Write([]byte(","))
			}
			fmt.Fprintf(w, `{"reference":"Obadiah 1:%d","content":"verse %d"}`, i, i)
		}
		w.Write([]byte(`]}`))
	}))
	defer srv.Close()

	c, err := New(key, "")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	resp, err := c.Search("aloof")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) > MaxVerses(31) {
		t.Fatalf("search returned %d Obadiah verses, cap %d", len(resp.Results), MaxVerses(31))
	}
	if resp.Results[0].Book != 31 || resp.Results[0].Verse != 1 {
		t.Fatalf("first hit = %+v", resp.Results[0])
	}
}
