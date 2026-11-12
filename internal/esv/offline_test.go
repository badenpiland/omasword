package esv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// thirtyVerseServer answers every passage query with a 30-verse chapter.
func thirtyVerseServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&b, "[%d] Fixture verse %d. ", i, i)
	}
	body := b.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprintf(w, `{"passages":[%q]}`, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// tickingClient returns a client whose clock moves a second per call, so
// recency order is well defined.
func tickingClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New("test-key-not-real", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = base
	var tick atomic.Int64
	clock := func() time.Time {
		return time.Unix(1_700_000_000+tick.Add(1), 0)
	}
	c.now = clock
	c.cache.now = clock
	c.limiter.now = clock
	return c
}

func TestPinnedPagesSurviveEviction(t *testing.T) {
	c, err := OpenVerseCache("")
	if err != nil {
		t.Fatal(err)
	}
	var tick int64
	c.now = func() time.Time { tick++; return time.Unix(tick, 0) }

	ps23 := verses(6)
	if err := c.Put(19, 23, 6, ps23, ""); err != nil {
		t.Fatal(err)
	}
	if found, err := c.SetPinned(19, 23, true); err != nil || !found {
		t.Fatalf("pin found=%v err=%v", found, err)
	}
	// Twelve Genesis-sized pages push 1,200 verses through a 500 cap.
	for ch := 1; ch <= 12; ch++ {
		if err := c.Put(1, ch, 100, verses(100), ""); err != nil {
			t.Fatal(err)
		}
	}
	if stored, pinned := c.Has(19, 23, 1); !stored || !pinned {
		t.Fatalf("Psalm 23 stored=%v pinned=%v after eviction", stored, pinned)
	}
	if n := c.VerseCount(); n > MaxStoredVerses {
		t.Fatalf("stored %d verses", n)
	}

	// A refreshed page keeps its pin.
	if err := c.Put(19, 23, 6, verses(6), ""); err != nil {
		t.Fatal(err)
	}
	if _, pinned := c.Has(19, 23, 1); !pinned {
		t.Fatal("refreshing Psalm 23 dropped its pin")
	}

	if found, _ := c.SetPinned(43, 3, true); found {
		t.Fatal("pinned a chapter that is not stored")
	}
	if s := c.Stats(); s.PinnedChapters != 1 || s.PinnedVerses != 6 {
		t.Fatalf("stats = %+v", s)
	}
}

func TestPinsFillingTheCapStopNewPagesNotOldOnes(t *testing.T) {
	c, err := OpenVerseCache("")
	if err != nil {
		t.Fatal(err)
	}
	var tick int64
	c.now = func() time.Time { tick++; return time.Unix(tick, 0) }
	for ch := 1; ch <= 5; ch++ {
		if err := c.Put(1, ch, 100, verses(100), ""); err != nil {
			t.Fatal(err)
		}
		if _, err := c.SetPinned(1, ch, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Put(2, 1, 22, verses(22), ""); err != nil {
		t.Fatal(err)
	}
	if stored, _ := c.Has(2, 1, 1); stored {
		t.Fatal("stored a page past a cap that pins fill")
	}
	for ch := 1; ch <= 5; ch++ {
		if stored, _ := c.Has(1, ch, 1); !stored {
			t.Fatalf("lost pinned Genesis %d", ch)
		}
	}
}

func TestSaveAheadFillsHalfTheBookAndStops(t *testing.T) {
	var hits atomic.Int32
	srv := thirtyVerseServer(t, &hits)
	c := tickingClient(t, srv.URL)

	// An old page elsewhere may be evicted; a pinned one may not.
	if err := c.cache.Put(45, 1, 32, verses(32), ""); err != nil {
		t.Fatal(err)
	}
	if err := c.cache.Put(19, 23, 6, verses(6), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetPinned(19, 23, true); err != nil {
		t.Fatal(err)
	}

	res, err := c.SaveAhead(43, 1)
	if err != nil {
		t.Fatal(err)
	}
	// John may hold 439 verses: fourteen 30-verse chapters. The fifteenth
	// is fetched, does not fit, and is not stored.
	if res.Fetched != 14 || !res.Full || !res.BookLimit || res.FullBook != 43 || res.ThroughBook != 43 || res.ThroughChapter != 15 {
		t.Fatalf("result = %+v", res)
	}
	if got := hits.Load(); got != 15 {
		t.Fatalf("made %d requests, want 15", got)
	}
	if stored, _ := c.cache.Has(43, 16, 1); stored {
		t.Fatal("stored John 16 past the half-book cap")
	}
	if stored, pinned := c.cache.Has(19, 23, 1); !stored || !pinned {
		t.Fatal("save ahead evicted a pinned chapter")
	}
	s := c.Stats()
	if s.Verses > MaxStoredVerses {
		t.Fatalf("stored %d verses", s.Verses)
	}

	// A second run finds everything saved. Chapter lengths aren't known
	// before a fetch, so it spends one request learning John 16 won't fit.
	before := hits.Load()
	res, err = c.SaveAhead(43, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.AlreadySaved != 14 || res.Fetched != 0 || !res.Full {
		t.Fatalf("second result = %+v", res)
	}
	if got := hits.Load() - before; got != 1 {
		t.Fatalf("second run made %d requests, want 1", got)
	}

	// Saved pages read back as saved, and offline-ready.
	page, err := c.Chapter(43, 2, 1)
	if err != nil || !page.Saved || page.Pinned {
		t.Fatalf("John 2 saved=%v pinned=%v err=%v", page.Saved, page.Pinned, err)
	}
}

func TestSaveAheadCrossesIntoTheNextBookAndEndsAtRevelation(t *testing.T) {
	var hits atomic.Int32
	srv := thirtyVerseServer(t, &hits)
	c := tickingClient(t, srv.URL)

	res, err := c.SaveAhead(65, 1) // Jude has one chapter
	if err != nil {
		t.Fatal(err)
	}
	if res.ThroughBook != 66 || !res.Full {
		t.Fatalf("result = %+v", res)
	}
	// Revelation is 404 verses; half is 201, so six 30-verse chapters fit.
	if res.Fetched != 6 {
		t.Fatalf("fetched %d Revelation chapters, want 6", res.Fetched)
	}

	if _, _, ok := nextChapter(66, 22); ok {
		t.Fatal("found a chapter after Revelation 22")
	}
	if b, ch, ok := nextChapter(43, 21); !ok || b != 44 || ch != 1 {
		t.Fatalf("after John 21: %d:%d %v", b, ch, ok)
	}
}

func TestSaveAheadNeedsAKey(t *testing.T) {
	c, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SaveAhead(43, 1); err != ErrNoKey {
		t.Fatalf("err = %v", err)
	}
}
