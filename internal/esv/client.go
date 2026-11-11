package esv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"omasword/internal/api"
)

const (
	defaultBase = "https://api.esv.org"
	// A chapter plus the copyright notice is far smaller than this.
	maxBody = 2 << 20
)

// ErrNoKey is returned when a request needs a key and none is configured.
var ErrNoKey = errors.New("ESV needs your API key from https://api.esv.org/")

// Passage is one legal page of ESV text.
type Passage struct {
	Verses      []api.Verse
	First, Last int
	Total       int // verses in the chapter, before the page was clipped
	Notice      string
	MoreBefore  bool
	MoreAfter   bool
}

// Client fetches ESV text from api.esv.org. It never bulk-downloads a
// translation. Local storage goes through VerseCache, which enforces
// Crossway's 500-verse and half-book caps.
type Client struct {
	mu      sync.Mutex
	key     string
	baseURL string
	http    *http.Client
	cache   *VerseCache
	limiter *Limiter
	now     func() time.Time
	notice  string
}

// DefaultCacheDir is ~/.cache/omasword (honoring XDG_CACHE_HOME). An
// empty string means the caller should keep the ESV cache in memory.
func DefaultCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "omasword")
}

// New builds a client. cacheDir holds esv-verses.json and esv-usage.json;
// an empty cacheDir keeps both in memory. key may be empty, in which case
// Chapter and Search return ErrNoKey.
func New(key, cacheDir string) (*Client, error) {
	var verses *VerseCache
	var limiter *Limiter
	var err error
	if cacheDir == "" {
		verses, err = OpenVerseCache("")
		if err != nil {
			return nil, err
		}
		limiter, err = OpenLimiter("")
		if err != nil {
			return nil, err
		}
	} else {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return nil, err
		}
		verses, err = OpenVerseCache(filepath.Join(cacheDir, "esv-verses.json"))
		if err != nil {
			return nil, err
		}
		limiter, err = OpenLimiter(filepath.Join(cacheDir, "esv-usage.json"))
		if err != nil {
			return nil, err
		}
	}
	return &Client{
		key:     strings.TrimSpace(key),
		baseURL: defaultBase,
		http:    newHTTPClient(),
		cache:   verses,
		limiter: limiter,
		now:     time.Now,
		notice:  verses.file.Notice,
	}, nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("ESV API redirected too many times")
			}
			// Don't take the key to a different host.
			if req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}
}

// SetKey replaces the key used on later requests. It does not write config.
func (c *Client) SetKey(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.key = strings.TrimSpace(key)
	c.mu.Unlock()
}

// HasKey reports whether a request can be attempted.
func (c *Client) HasKey() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key != ""
}

// Notice is the copyright string to display: the one Crossway last sent,
// or the standard notice when none has arrived yet.
func (c *Client) Notice() string {
	if c == nil {
		return CopyrightNotice
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.notice == "" {
		return CopyrightNotice
	}
	return c.notice
}

func (c *Client) setNotice(notice string) {
	if notice == "" {
		return
	}
	c.mu.Lock()
	c.notice = notice
	c.mu.Unlock()
}

func (c *Client) snapshot() (key, base string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key, c.baseURL
}

// Chapter fetches one chapter and returns the page that starts at
// startVerse, clipped to the per-page cap. A cached page is used when
// it is present, so repeat visits do not call the API.
func (c *Client) Chapter(bookID, chapter, startVerse int) (Passage, error) {
	if !c.HasKey() {
		return Passage{}, ErrNoKey
	}
	name, ok := BookName(bookID)
	if !ok || chapter < 1 {
		return Passage{}, fmt.Errorf("unknown passage %d:%d", bookID, chapter)
	}
	if startVerse < 1 {
		startVerse = 1
	}
	if verses, total, notice, ok := c.cache.Get(bookID, chapter, startVerse); ok {
		c.setNotice(notice)
		p := pageFrom(verses, total)
		p.Notice = c.Notice()
		return p, nil
	}
	all, notice, err := c.fetchChapter(name, chapter)
	if err != nil {
		return Passage{}, err
	}
	c.setNotice(notice)
	page := clipPage(all, startVerse, MaxVerses(bookID))
	page.Notice = c.Notice()
	for i := range page.Verses {
		page.Verses[i].Book = bookID
		page.Verses[i].Chapter = chapter
		page.Verses[i].Translation = ID
	}
	if err := c.cache.Put(bookID, chapter, len(all), page.Verses, notice); err != nil {
		return Passage{}, err
	}
	return page, nil
}

// Search runs a word search and returns at most one legal page of hits.
// Results are not cached: a search page is easy to over-store, and the
// chapter cache already covers passages the reader opens.
func (c *Client) Search(query string) (*api.SearchResponse, error) {
	if !c.HasKey() {
		return nil, ErrNoKey
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("empty search")
	}
	body, err := c.get("/v3/passage/search/", url.Values{
		"q":         {query},
		"page-size": {"20"},
	})
	if err != nil {
		return nil, err
	}
	var raw struct {
		Total   int `json:"total_results"`
		Results []struct {
			Reference string `json:"reference"`
			Content   string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("ESV search response was not valid JSON")
	}
	resp := &api.SearchResponse{Total: raw.Total}
	perBook := map[int]int{}
	for _, hit := range raw.Results {
		if len(resp.Results) >= MaxStoredVerses {
			break
		}
		book, chapter, verse, ok := ParseReference(hit.Reference)
		if !ok {
			continue
		}
		if perBook[book] >= MaxVerses(book) {
			continue
		}
		text := strings.TrimSpace(api.SanitizeText(hit.Content))
		if text == "" {
			continue
		}
		resp.Results = append(resp.Results, api.Verse{
			Verse: verse, Text: text, Book: book, Chapter: chapter, Translation: ID,
		})
		perBook[book]++
	}
	return resp, nil
}

func (c *Client) fetchChapter(name string, chapter int) ([]api.Verse, string, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("%s %d", name, chapter))
	// Ask for verse numbers and the copyright notice, and turn off the
	// furniture (headings, footnotes, audio) so the verse text stays
	// the words Crossway sent.
	for _, off := range []string{
		"include-headings", "include-footnotes", "include-footnote-body",
		"include-passage-references", "include-short-copyright",
		"include-audio-link", "include-chapter-numbers", "include-subheadings",
		"indent-poetry",
	} {
		q.Set(off, "false")
	}
	q.Set("include-verse-numbers", "true")
	q.Set("include-first-verse-numbers", "true")
	q.Set("include-copyright", "true")
	q.Set("indent-paragraphs", "0")
	body, err := c.get("/v3/passage/html/", q)
	if err != nil {
		return nil, "", err
	}
	var raw struct {
		Passages []string `json:"passages"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || len(raw.Passages) == 0 {
		return nil, "", errors.New("ESV passage response was empty")
	}
	return ParseHTML(raw.Passages[0])
}

func (c *Client) get(path string, query url.Values) ([]byte, error) {
	if err := c.limiter.Acquire(c.now()); err != nil {
		return nil, c.redact(err)
	}
	key, base := c.snapshot()
	endpoint, err := url.Parse(base + path)
	if err != nil {
		return nil, c.redact(err)
	}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, c.redact(err)
	}
	req.Header.Set("Authorization", "Token "+key)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.redact(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, c.redact(err)
	}
	if int64(len(body)) > maxBody {
		return nil, errors.New("ESV response was too large")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		wait := retryAfter(resp)
		c.limiter.Penalize(c.now(), wait)
		return nil, &RateError{RetryAfter: wait}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("ESV API key was rejected; check the key from https://api.esv.org/")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.redact(fmt.Errorf("ESV API returned status %d", resp.StatusCode))
	}
	return body, nil
}

func retryAfter(resp *http.Response) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err != nil || secs < 1 {
		return time.Minute
	}
	d := time.Duration(secs) * time.Second
	if d > 24*time.Hour {
		return 24 * time.Hour
	}
	return d
}

// redact strips the API key out of an error string. HTTP failures can
// echo the request, header included.
func (c *Client) redact(err error) error {
	if err == nil {
		return err
	}
	key, _ := c.snapshot()
	if key == "" {
		return err
	}
	msg := err.Error()
	if !strings.Contains(msg, key) {
		return err
	}
	return errors.New(strings.ReplaceAll(msg, key, "[redacted]"))
}

func clipPage(all []api.Verse, startVerse, limit int) Passage {
	if limit < 1 {
		limit = 1
	}
	var page []api.Verse
	for _, v := range all {
		if v.Verse < startVerse {
			continue
		}
		if len(page) >= limit {
			break
		}
		page = append(page, v)
	}
	return pageFrom(page, len(all))
}

// pageFrom marks whether the chapter continues past this page. total is
// the number of verses in the chapter; ESV chapters are numbered from 1
// with no gaps, so the last verse number is that count.
func pageFrom(verses []api.Verse, total int) Passage {
	p := Passage{Verses: verses, Total: total}
	if len(verses) == 0 {
		return p
	}
	p.First = verses[0].Verse
	p.Last = verses[len(verses)-1].Verse
	p.MoreBefore = p.First > 1
	if total > 0 && p.Last < total {
		p.MoreAfter = true
	}
	return p
}

// ParseReference reads "John 11:35" or "Song of Solomon 2:4" into a book
// ID, chapter, and verse. Verse may be 0 when the reference has no verse.
func ParseReference(ref string) (book, chapter, verse int, ok bool) {
	ref = strings.TrimSpace(ref)
	id, rest, matched := LookupBook(ref)
	if !matched {
		return 0, 0, 0, false
	}
	rest = strings.TrimSpace(strings.TrimLeft(rest, ":"))
	if rest == "" {
		return id, 1, 0, true
	}
	parts := strings.FieldsFunc(rest, func(r rune) bool {
		return r == ':' || r == '.' || r == ' '
	})
	if len(parts) == 0 {
		return id, 1, 0, true
	}
	chapter, err := strconv.Atoi(parts[0])
	if err != nil || chapter < 1 {
		return 0, 0, 0, false
	}
	if len(parts) >= 2 {
		verse, err = strconv.Atoi(parts[1])
		if err != nil || verse < 1 {
			return 0, 0, 0, false
		}
	}
	return id, chapter, verse, true
}
