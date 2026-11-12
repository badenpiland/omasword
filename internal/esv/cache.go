package esv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"omasword/internal/api"
)

// entry is one stored page of a chapter, keyed by the first verse number
// so two pages of a short book do not both have to stay on disk.
type entry struct {
	Book    int         `json:"book"`
	Chapter int         `json:"chapter"`
	Total   int         `json:"total"`
	Used    int64       `json:"used"`
	Pinned  bool        `json:"pinned,omitempty"`
	Verses  []api.Verse `json:"verses"`
}

type cacheFile struct {
	Notice  string  `json:"notice,omitempty"`
	Entries []entry `json:"entries"`
}

// VerseCache stores ESV pages under Crossway's cap: at most 500 verses in
// total, and at most half of any one book, whichever limit binds first.
// Puts that would pass a limit evict the least recently used pages that
// are not pinned. Pinned pages still count toward both caps.
type VerseCache struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
	file cacheFile
}

// OpenVerseCache loads path, or an empty cache when the file is missing.
// A file that already exceeds the cap is evicted down on open. An empty
// path keeps the cache in memory only.
func OpenVerseCache(path string) (*VerseCache, error) {
	c := &VerseCache{path: path, now: time.Now}
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &c.file); err != nil {
		// A corrupt cache is discarded rather than served.
		c.file = cacheFile{}
		return c, nil
	}
	if c.evict() {
		if err := c.save(); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Get returns the page whose first verse is startVerse. total is the
// number of verses in the chapter before clipping, so a cache hit can
// still tell the reader that another page exists.
func (c *VerseCache) Get(book, chapter, startVerse int) (verses []api.Verse, total int, notice string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.file.Entries {
		e := &c.file.Entries[i]
		if e.Book != book || e.Chapter != chapter || len(e.Verses) == 0 {
			continue
		}
		if e.Verses[0].Verse != startVerse {
			continue
		}
		// Recency is kept in memory and written with the next Put; a
		// cache hit doesn't rewrite the file.
		e.Used = c.now().UnixNano()
		return append([]api.Verse(nil), e.Verses...), e.Total, c.file.Notice, true
	}
	return nil, 0, "", false
}

// Put stores verses, trimming them to the per-book and 500-verse caps
// before eviction. total is the unclipped chapter length. notice replaces
// the stored copyright string when set.
func (c *VerseCache) Put(book, chapter, total int, verses []api.Verse, notice string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	verses = trimPage(book, verses)
	if len(verses) == 0 {
		return nil
	}
	fresh := make([]api.Verse, len(verses))
	copy(fresh, verses)
	start := fresh[0].Verse
	pinned := false
	kept := c.file.Entries[:0]
	for _, e := range c.file.Entries {
		if e.Book == book && e.Chapter == chapter && len(e.Verses) > 0 && e.Verses[0].Verse == start {
			// A refreshed page keeps its pin.
			pinned = pinned || e.Pinned
			continue
		}
		kept = append(kept, e)
	}
	c.file.Entries = append(kept, entry{
		Book: book, Chapter: chapter, Total: total, Used: c.now().UnixNano(),
		Pinned: pinned, Verses: fresh,
	})
	if notice != "" {
		c.file.Notice = notice
	}
	c.evict()
	return c.save()
}

// VerseCount is the number of verses currently stored.
func (c *VerseCache) VerseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return countVerses(c.file.Entries)
}

// Has reports whether the page starting at startVerse is stored, and
// whether it is pinned. It does not change recency.
func (c *VerseCache) Has(book, chapter, startVerse int) (stored, pinned bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.file.Entries {
		if e.Book == book && e.Chapter == chapter && len(e.Verses) > 0 && e.Verses[0].Verse == startVerse {
			return true, e.Pinned
		}
	}
	return false, false
}

// SetPinned pins or unpins every stored page of a chapter. found is false
// when no page of that chapter is stored, since only stored text can be
// kept.
func (c *VerseCache) SetPinned(book, chapter int, pinned bool) (found bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := false
	for i := range c.file.Entries {
		e := &c.file.Entries[i]
		if e.Book != book || e.Chapter != chapter {
			continue
		}
		found = true
		if e.Pinned != pinned {
			e.Pinned = pinned
			changed = true
		}
	}
	if !changed {
		return found, nil
	}
	return found, c.save()
}

// Touch marks every stored page of a chapter as just used, so the next
// evictions take other pages first. It is not written until the next Put.
func (c *VerseCache) Touch(book, chapter int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now().UnixNano()
	for i := range c.file.Entries {
		if e := &c.file.Entries[i]; e.Book == book && e.Chapter == chapter {
			e.Used = now
		}
	}
}

// Room is how many more verses of book could be stored if every page
// that is unpinned and was last used before since were evicted. It is the
// smaller of the room left under the 500-verse cap and under that book's
// half-book cap, and never negative. bookBound is true when the half-book
// cap is the smaller one.
func (c *VerseCache) Room(book int, since time.Time) (room int, bookBound bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := since.UnixNano()
	total, inBook := MaxStoredVerses, MaxVerses(book)
	for _, e := range c.file.Entries {
		if !e.Pinned && e.Used < cutoff {
			continue
		}
		total -= len(e.Verses)
		if e.Book == book {
			inBook -= len(e.Verses)
		}
	}
	return max(min(total, inBook), 0), inBook < total
}

// Stats counts stored verses, and how many of them and how many chapters
// are pinned.
type Stats struct {
	Verses, PinnedVerses, PinnedChapters int
}

// Stats reports what is stored now.
func (c *VerseCache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	var s Stats
	chapters := map[[2]int]bool{}
	for _, e := range c.file.Entries {
		s.Verses += len(e.Verses)
		if e.Pinned {
			s.PinnedVerses += len(e.Verses)
			chapters[[2]int{e.Book, e.Chapter}] = true
		}
	}
	s.PinnedChapters = len(chapters)
	return s
}

func trimPage(book int, verses []api.Verse) []api.Verse {
	limit := MaxVerses(book)
	if limit > MaxStoredVerses {
		limit = MaxStoredVerses
	}
	if len(verses) > limit {
		verses = verses[:limit]
	}
	return verses
}

// evict drops the least recently used unpinned pages until both caps
// hold. A book that is over its own half-book cap loses its oldest page
// first, so paging inside Obadiah does not flush an unrelated book. A
// single page that is itself over a cap is trimmed. Pins only ever cover
// pages that were stored within the caps, so once every unpinned page is
// gone the caps hold again. Reports whether anything changed.
func (c *VerseCache) evict() bool {
	changed := false
	for {
		if len(c.file.Entries) == 1 {
			e := &c.file.Entries[0]
			limit := MaxVerses(e.Book)
			if len(e.Verses) > limit {
				e.Verses = e.Verses[:limit]
				changed = true
			}
			if len(e.Verses) > MaxStoredVerses {
				e.Verses = e.Verses[:MaxStoredVerses]
				changed = true
			}
			return changed
		}
		book, bookOver := overBook(c.file.Entries)
		totalOver := countVerses(c.file.Entries) > MaxStoredVerses
		if !bookOver && !totalOver {
			return changed
		}
		only := book
		if !bookOver {
			only = 0
		}
		if !c.dropOldest(only) {
			return changed
		}
		changed = true
	}
}

// overBook reports a book whose stored verses exceed half that book.
func overBook(entries []entry) (book int, over bool) {
	per := map[int]int{}
	for _, e := range entries {
		per[e.Book] += len(e.Verses)
	}
	for id, n := range per {
		if n > MaxVerses(id) {
			return id, true
		}
	}
	return 0, false
}

// dropOldest removes the least recently used unpinned entry. book > 0
// limits the choice to that book. Returns false when nothing could be
// removed.
func (c *VerseCache) dropOldest(book int) bool {
	oldest := -1
	for i, e := range c.file.Entries {
		if e.Pinned || (book > 0 && e.Book != book) {
			continue
		}
		if oldest < 0 || e.Used < c.file.Entries[oldest].Used {
			oldest = i
		}
	}
	if oldest < 0 {
		return false
	}
	c.file.Entries = append(c.file.Entries[:oldest], c.file.Entries[oldest+1:]...)
	return true
}

func countVerses(entries []entry) int {
	n := 0
	for _, e := range entries {
		n += len(e.Verses)
	}
	return n
}

func (c *VerseCache) save() error {
	if c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c.file)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), "esv-verses.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, c.path)
}
