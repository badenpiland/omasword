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
	Verses  []api.Verse `json:"verses"`
}

type cacheFile struct {
	Notice  string  `json:"notice,omitempty"`
	Entries []entry `json:"entries"`
}

// VerseCache stores ESV pages under Crossway's cap: at most 500 verses in
// total, and at most half of any one book, whichever limit binds first.
// Puts that would pass a limit evict the least recently used pages.
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
	kept := c.file.Entries[:0]
	for _, e := range c.file.Entries {
		if e.Book == book && e.Chapter == chapter && len(e.Verses) > 0 && e.Verses[0].Verse == start {
			continue
		}
		kept = append(kept, e)
	}
	c.file.Entries = append(kept, entry{
		Book: book, Chapter: chapter, Total: total, Used: c.now().UnixNano(), Verses: fresh,
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

// evict drops the least recently used pages until both caps hold. A book
// that is over its own half-book cap loses its oldest page first, so
// paging inside Obadiah does not flush an unrelated book. A single page
// that is itself over a cap is trimmed. Reports whether anything changed.
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

// dropOldest removes the least recently used entry. book > 0 limits the
// choice to that book. Returns false when nothing could be removed.
func (c *VerseCache) dropOldest(book int) bool {
	oldest := -1
	for i, e := range c.file.Entries {
		if book > 0 && e.Book != book {
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
