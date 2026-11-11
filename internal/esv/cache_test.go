package esv

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"omasword/internal/api"
)

func verses(n int) []api.Verse {
	out := make([]api.Verse, n)
	for i := range out {
		out[i] = api.Verse{Verse: i + 1, Text: "word"}
	}
	return out
}

func TestCacheStaysWithin500AndHalfABook(t *testing.T) {
	c, err := OpenVerseCache(filepath.Join(t.TempDir(), "esv-verses.json"))
	if err != nil {
		t.Fatal(err)
	}
	c.now = func() time.Time { return time.Unix(0, 0) }
	// Six Genesis chapters of 100 verses would be 600. The cache keeps 500.
	base := time.Unix(1_700_000_000, 0)
	for i := 1; i <= 6; i++ {
		now := base.Add(time.Duration(i) * time.Second)
		c.now = func() time.Time { return now }
		page := verses(100)
		for j := range page {
			page[j].Chapter = i
		}
		if err := c.Put(1, i, 100, page, CopyrightNotice); err != nil {
			t.Fatal(err)
		}
	}
	if got := c.VerseCount(); got > MaxStoredVerses {
		t.Fatalf("stored %d verses, cap is %d", got, MaxStoredVerses)
	}
	if got := c.VerseCount(); got != MaxStoredVerses {
		t.Fatalf("stored %d verses, want exactly %d after filling the cap", got, MaxStoredVerses)
	}

	// Obadiah is 21 verses; only half of the book may be stored.
	ob := verses(21)
	c.now = func() time.Time { return base.Add(time.Minute) }
	if err := c.Put(31, 1, 21, ob, ""); err != nil {
		t.Fatal(err)
	}
	got, _, _, ok := c.Get(31, 1, 1)
	if !ok || len(got) > MaxVerses(31) {
		t.Fatalf("obadiah page len %d ok=%v, cap %d", len(got), ok, MaxVerses(31))
	}

	// A second Obadiah page must evict the first so the book stays at half.
	second := verses(10)
	for i := range second {
		second[i].Verse = 11 + i
	}
	c.now = func() time.Time { return base.Add(2 * time.Minute) }
	if err := c.Put(31, 1, 21, second, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := c.Get(31, 1, 1); ok {
		t.Fatal("the first Obadiah page should have been evicted")
	}
	got, total, _, ok := c.Get(31, 1, 11)
	if !ok || len(got) != 10 || total != 21 {
		t.Fatalf("second page len %d total %d ok=%v", len(got), total, ok)
	}
	if c.VerseCount() > MaxStoredVerses {
		t.Fatalf("stored %d after eviction", c.VerseCount())
	}
}

func TestCacheReloadsUnderTheCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "esv-verses.json")
	c, err := OpenVerseCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put(43, 3, 36, verses(36), "from crossway"); err != nil {
		t.Fatal(err)
	}
	again, err := OpenVerseCache(path)
	if err != nil {
		t.Fatal(err)
	}
	got, total, notice, ok := again.Get(43, 3, 1)
	if !ok || len(got) != 36 || total != 36 || notice != "from crossway" {
		t.Fatalf("reloaded len %d total %d notice %q ok=%v", len(got), total, notice, ok)
	}
}

func TestCacheHitDoesNotRewriteTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "esv-verses-v2.json")
	c, err := OpenVerseCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Put(43, 3, 36, []api.Verse{{Verse: 1, Text: "a"}}, ""); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	old := before.ModTime().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := c.Get(43, 3, 1); !ok {
		t.Fatal("cache miss")
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(old) {
		t.Fatal("a cache hit rewrote the file")
	}
}
