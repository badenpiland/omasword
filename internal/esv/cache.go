package esv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// Saved holds the verses you chose to keep, one entry per chapter
	// with its verses in order (not necessarily contiguous).
	Saved []entry `json:"saved,omitempty"`
}

// VerseCache stores ESV text under Crossway's cap: at most 500 verses in
// total, and at most half of any one book, whichever limit binds first.
// Saved verses (SaveVerses) and recently read pages (Put) share those
// caps. Saved verses are kept until you remove them; reading only uses
// the room they leave, and evicts its least recently used pages past it.
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

// VerseCount is the number of verses currently stored, saved and read.
func (c *VerseCache) VerseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return countVerses(c.all())
}

func (c *VerseCache) all() []entry {
	return append(append([]entry(nil), c.file.Saved...), c.file.Entries...)
}

// SaveLimitError is returned when saving would pass a cap. Book is set
// when the half-book cap is the one in the way.
type SaveLimitError struct {
	Book         int
	Limit, Saved int
}

func (e *SaveLimitError) Error() string {
	if e.Book == 0 {
		return fmt.Sprintf("can't save: Crossway allows %d saved ESV verses and %d are saved", e.Limit, e.Saved)
	}
	name, _ := BookName(e.Book)
	return fmt.Sprintf("can't save: Crossway allows %d verses of %s (under half) and %d are saved", e.Limit, name, e.Saved)
}

// SaveVerses adds verses of one chapter to what you have saved. Verses
// already saved are not counted twice. It fails with *SaveLimitError,
// saving nothing, when the saved verses would pass 500 or half the book.
// Read pages are evicted to make room.
func (c *VerseCache) SaveVerses(book, chapter int, verses []api.Verse, notice string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(verses) == 0 {
		return nil
	}
	i := c.savedIndex(book, chapter)
	var have []api.Verse
	if i >= 0 {
		have = c.file.Saved[i].Verses
	}
	merged := mergeVerses(have, verses)
	added := len(merged) - len(have)
	if added == 0 {
		return nil
	}
	total, inBook := 0, 0
	for _, e := range c.file.Saved {
		total += len(e.Verses)
		if e.Book == book {
			inBook += len(e.Verses)
		}
	}
	if limit := MaxVerses(book); inBook+added > limit {
		return &SaveLimitError{Book: book, Limit: limit, Saved: inBook}
	}
	if total+added > MaxStoredVerses {
		return &SaveLimitError{Limit: MaxStoredVerses, Saved: total}
	}
	if i >= 0 {
		c.file.Saved[i].Verses = merged
		c.file.Saved[i].Used = c.now().UnixNano()
	} else {
		c.file.Saved = append(c.file.Saved, entry{
			Book: book, Chapter: chapter, Used: c.now().UnixNano(), Verses: merged,
		})
	}
	if notice != "" {
		c.file.Notice = notice
	}
	c.evict()
	return c.save()
}

// UnsaveVerses removes saved verses from through to of a chapter. to < 1
// removes the whole chapter.
func (c *VerseCache) UnsaveVerses(book, chapter, from, to int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i := c.savedIndex(book, chapter)
	if i < 0 {
		return nil
	}
	kept := c.file.Saved[i].Verses[:0]
	for _, v := range c.file.Saved[i].Verses {
		if to >= 1 && (v.Verse < from || v.Verse > to) {
			kept = append(kept, v)
		}
	}
	if len(kept) == 0 {
		c.file.Saved = append(c.file.Saved[:i], c.file.Saved[i+1:]...)
	} else {
		c.file.Saved[i].Verses = kept
	}
	return c.save()
}

// SavedVerses returns the saved verses of one chapter, in order.
func (c *VerseCache) SavedVerses(book, chapter int) []api.Verse {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i := c.savedIndex(book, chapter); i >= 0 {
		return append([]api.Verse(nil), c.file.Saved[i].Verses...)
	}
	return nil
}

// SavedChapter is one chapter's saved verse numbers.
type SavedChapter struct {
	Book, Chapter int
	Verses        []int
}

// SavedList lists what is saved, in canonical order.
func (c *VerseCache) SavedList() []SavedChapter {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]SavedChapter, 0, len(c.file.Saved))
	for _, e := range c.file.Saved {
		sc := SavedChapter{Book: e.Book, Chapter: e.Chapter}
		for _, v := range e.Verses {
			sc.Verses = append(sc.Verses, v.Verse)
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Book != out[b].Book {
			return out[a].Book < out[b].Book
		}
		return out[a].Chapter < out[b].Chapter
	})
	return out
}

// SavedCount is the number of saved verses.
func (c *VerseCache) SavedCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return countVerses(c.file.Saved)
}

func (c *VerseCache) savedIndex(book, chapter int) int {
	for i, e := range c.file.Saved {
		if e.Book == book && e.Chapter == chapter {
			return i
		}
	}
	return -1
}

// mergeVerses is the union of a and b by verse number, sorted. A verse in
// both keeps b's text.
func mergeVerses(a, b []api.Verse) []api.Verse {
	byNum := map[int]api.Verse{}
	for _, v := range a {
		byNum[v.Verse] = v
	}
	for _, v := range b {
		byNum[v.Verse] = v
	}
	out := make([]api.Verse, 0, len(byNum))
	for _, v := range byNum {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Verse < out[j].Verse })
	return out
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

// evict drops the least recently used read pages until both caps hold
// for saved and read verses together. Saved verses are never evicted;
// SaveVerses keeps them within the caps on their own. A book
// that is over its own half-book cap loses its oldest page first, so
// paging inside Obadiah does not flush an unrelated book. A single page
// that is itself over a cap is trimmed. Reports whether anything changed.
func (c *VerseCache) evict() bool {
	changed := false
	for {
		if len(c.file.Entries) == 1 && len(c.file.Saved) == 0 {
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
		book, bookOver := overBook(c.all())
		totalOver := countVerses(c.all()) > MaxStoredVerses
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
