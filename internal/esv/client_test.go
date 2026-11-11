package esv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// obadiahChapter is 21 verses, the shape api.esv.org returns for
// "Obadiah". A bare "Obadiah 1" is only verse 1.
var obadiahChapter = func() string {
	var b strings.Builder
	b.WriteString("[1] The vision of Obadiah. [2] Behold, I will make you small.\n")
	for i := 3; i <= 21; i++ {
		fmt.Fprintf(&b, "[%d] Obadiah fixture verse %d.\n", i, i)
	}
	b.WriteString("ESV ® Bible. Please visit our permission site .\n")
	return b.String()
}()

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
		if !strings.Contains(r.URL.Path, "/v3/passage/text/") {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("include-verse-numbers") != "true" || q.Get("include-copyright") != "false" || q.Get("include-footnotes") != "false" || q.Get("include-headings") != "false" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		// Crossway reads "Obadiah 1" as verse 1. Only the explicit range
		// is the chapter.
		switch q.Get("q") {
		case "Obadiah 1":
			fmt.Fprintf(w, `{"passages":["[1] The vision of Obadiah."]}`)
		case "Obadiah":
			fmt.Fprintf(w, `{"passages":[%q]}`, obadiahChapter)
		default:
			t.Errorf("query q = %q", q.Get("q"))
			fmt.Fprintf(w, `{"passages":["[1] The vision of Obadiah."]}`)
		}
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
	if page.Verses[0].Text != "The vision of Obadiah." || strings.Contains(page.Verses[0].Text, "Behold") || strings.Contains(page.Verses[0].Text, "permission") {
		t.Fatalf("verse 1 = %q", page.Verses[0].Text)
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

func TestNewDeletesTheOldVerseCache(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "esv-verses.json")
	if err := os.WriteFile(old, []byte(`{"entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New("", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old verse cache still on disk (err %v)", err)
	}
}
