package esv

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"omasword/internal/api"
)

func versesFrom(first, n int) []api.Verse {
	out := make([]api.Verse, n)
	for i := range out {
		out[i] = api.Verse{Verse: first + i, Text: "word"}
	}
	return out
}

func tickingCache(t *testing.T, path string) *VerseCache {
	t.Helper()
	c, err := OpenVerseCache(path)
	if err != nil {
		t.Fatal(err)
	}
	var tick int64
	c.now = func() time.Time { tick++; return time.Unix(tick, 0) }
	return c
}

func TestSavingOneVerseSavesOneVerse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "esv.json")
	c := tickingCache(t, path)
	if err := c.SaveVerses(2, 1, versesFrom(1, 1), CopyrightNotice); err != nil {
		t.Fatal(err)
	}
	if n := c.SavedCount(); n != 1 {
		t.Fatalf("saved %d verses, want 1", n)
	}
	// Saving it again, or an overlapping range, doesn't count it twice.
	if err := c.SaveVerses(2, 1, versesFrom(1, 3), ""); err != nil {
		t.Fatal(err)
	}
	if err := c.SaveVerses(2, 1, versesFrom(7, 1), ""); err != nil {
		t.Fatal(err)
	}
	list := c.SavedList()
	if len(list) != 1 || len(list[0].Verses) != 4 || list[0].Verses[3] != 7 {
		t.Fatalf("list = %+v", list)
	}

	// It survives a restart.
	again, err := OpenVerseCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.SavedVerses(2, 1); len(got) != 4 {
		t.Fatalf("after reopening: %d verses", len(got))
	}

	// Unsave a range, then the rest of the chapter.
	if err := c.UnsaveVerses(2, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if got := c.SavedVerses(2, 1); len(got) != 2 || got[0].Verse != 1 || got[1].Verse != 7 {
		t.Fatalf("after unsaving 2-3: %+v", got)
	}
	if err := c.UnsaveVerses(2, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(c.SavedList()) != 0 {
		t.Fatal("chapter still listed after removing it")
	}
}

func TestSavedVersesStayWithinTheCaps(t *testing.T) {
	c := tickingCache(t, "")
	// Obadiah (21 verses) allows 10.
	err := c.SaveVerses(31, 1, versesFrom(1, 11), "")
	var limit *SaveLimitError
	if !errors.As(err, &limit) || limit.Book != 31 || limit.Limit != MaxVerses(31) {
		t.Fatalf("err = %v", err)
	}
	if c.SavedCount() != 0 {
		t.Fatal("a refused save stored verses")
	}
	if err := c.SaveVerses(31, 1, versesFrom(1, 10), ""); err != nil {
		t.Fatal(err)
	}

	// 500 in total: five 98-verse chapters of Genesis fill 490 more.
	for ch := 1; ch <= 5; ch++ {
		if err := c.SaveVerses(1, ch, versesFrom(1, 98), ""); err != nil {
			t.Fatal(err)
		}
	}
	err = c.SaveVerses(19, 23, versesFrom(1, 6), "")
	if !errors.As(err, &limit) || limit.Book != 0 || limit.Saved != MaxStoredVerses {
		t.Fatalf("err = %v", err)
	}
}

func TestReadingMakesRoomForSavedVersesNotTheOtherWay(t *testing.T) {
	c := tickingCache(t, "")
	// Read pages fill the cache.
	for ch := 1; ch <= 5; ch++ {
		if err := c.Put(1, ch, 100, versesFrom(1, 100), ""); err != nil {
			t.Fatal(err)
		}
	}
	if c.VerseCount() != MaxStoredVerses {
		t.Fatalf("stored %d", c.VerseCount())
	}
	// Saving still works and evicts read pages.
	if err := c.SaveVerses(2, 1, versesFrom(1, 22), ""); err != nil {
		t.Fatal(err)
	}
	if c.VerseCount() > MaxStoredVerses || c.SavedCount() != 22 {
		t.Fatalf("stored %d, saved %d", c.VerseCount(), c.SavedCount())
	}
	// More reading never drops saved verses.
	for ch := 6; ch <= 12; ch++ {
		if err := c.Put(1, ch, 100, versesFrom(1, 100), ""); err != nil {
			t.Fatal(err)
		}
	}
	if c.SavedCount() != 22 || c.VerseCount() > MaxStoredVerses {
		t.Fatalf("stored %d, saved %d", c.VerseCount(), c.SavedCount())
	}
	// A saved verse is not served as a read page, so reading Exodus 1
	// online still fetches the whole chapter.
	if _, _, _, ok := c.Get(2, 1, 1); ok {
		t.Fatal("saved verses answered a chapter read")
	}
}
