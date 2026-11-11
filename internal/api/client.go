package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omasword/internal/translations"
)

const baseURL = "https://bolls.life"

// requestTimeout bounds every API call. Without it a stalled connection
// left the UI on "loading" forever with no way to retry.
const requestTimeout = 30 * time.Second

type CacheInterface interface {
	IsCached(translation string) bool
	GetChapter(translation string, book, chapter int) ([]Verse, error)
}

type Client struct {
	httpClient *http.Client
	cache      CacheInterface
	// retryDelays are the waits before each retry of a request that was
	// rate limited, hit a gateway error, or failed to connect. Its length
	// is the number of retries.
	retryDelays []time.Duration
	sleep       func(time.Duration)
}

func NewClient() *Client {
	return &Client{
		httpClient:  &http.Client{Timeout: requestTimeout},
		retryDelays: []time.Duration{1 * time.Second, 3 * time.Second},
		sleep:       time.Sleep,
	}
}

// maxRetryAfter is the longest Retry-After the client will wait out; a
// server asking for more gets its error reported instead.
const maxRetryAfter = 10 * time.Second

func (c *Client) SetCache(cache CacheInterface) {
	c.cache = cache
}

type Translation struct {
	ShortName string `json:"short_name"`
	FullName  string `json:"full_name"`
	Updated   int64  `json:"updated"`
	Dir       string `json:"dir,omitempty"`
}

type LanguageGroup struct {
	Language     string        `json:"language"`
	Translations []Translation `json:"translations"`
}

type Book struct {
	BookID     int    `json:"bookid"`
	ChronOrder int    `json:"chronorder"`
	Name       string `json:"name"`
	Chapters   int    `json:"chapters"`
}

type Verse struct {
	PK          int    `json:"pk"`
	Verse       int    `json:"verse"`
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
	Book        int    `json:"book,omitempty"`
	Chapter     int    `json:"chapter,omitempty"`
}

type ParallelVerseRequest struct {
	Translations []string `json:"translations"`
	Verses       []int    `json:"verses"`
	Chapter      int      `json:"chapter"`
	Book         int      `json:"book"`
}

// maxErrorBody caps how much of an error response is read into an error
// message (it is shown in the status bar).
const maxErrorBody = 512

func errorFromResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		return errors.New("bolls.life is busy (too many requests)")
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	msg := strings.TrimSpace(SanitizeText(string(body)))
	if msg == "" {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return fmt.Errorf("API returned status %d: %s", resp.StatusCode, msg)
}

// retryable reports whether a failed response is worth repeating: rate
// limits and gateway errors usually clear within seconds.
func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// retryAfter reads a Retry-After header given in seconds (the form
// bolls.life and most rate limiters use); dates are ignored.
func retryAfter(resp *http.Response) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// maxResponseBytes caps how much JSON one API response may decode. The
// largest real responses (a long chapter, 500 search hits) are well under
// 1 MiB.
const maxResponseBytes = 16 << 20

// doJSON sends req and decodes a 200 response into v, retrying rate
// limits, gateway errors and connection failures a few times. Timeouts
// are not retried: each attempt already waited requestTimeout.
func (c *Client) doJSON(req *http.Request, v any) error {
	for attempt := 0; ; attempt++ {
		retry, wait, err := c.doJSONOnce(req, v)
		if !retry || attempt >= len(c.retryDelays) {
			return err
		}
		if wait > maxRetryAfter {
			return err
		}
		c.sleep(max(wait, c.retryDelays[attempt]))
		if req.GetBody != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return err
			}
			req.Body = body
		}
	}
}

// doJSONOnce makes one attempt. retry says whether the failure is worth
// repeating, and wait is how long the server asked us to back off.
func (c *Client) doJSONOnce(req *http.Request, v any) (retry bool, wait time.Duration, err error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Drop Go's "Get \"https://…\":" prefix; the status bar is short.
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return false, 0, errors.New("bolls.life timed out")
		}
		if inner := errors.Unwrap(err); inner != nil {
			err = inner
		}
		return true, 0, fmt.Errorf("can't reach bolls.life (%w)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return retryable(resp.StatusCode), retryAfter(resp), errorFromResponse(resp)
	}
	return false, 0, json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(v)
}

func (c *Client) getJSON(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, v)
}

func checkTranslation(name string) error {
	if !ValidTranslationName(name) {
		return fmt.Errorf("invalid translation name %q", name)
	}
	return nil
}

// requireBuiltin rejects anything outside the public-domain allowlist
// before a URL is built or a cache file is opened. bolls.life also hosts
// copyrighted translations; those must not be requested or served.
func requireBuiltin(name string) error {
	if err := checkTranslation(name); err != nil {
		return err
	}
	if !translations.IsBuiltin(name) {
		return fmt.Errorf("%s is not in the public-domain library", name)
	}
	return nil
}

func sanitizeVerses(verses []Verse) {
	for i := range verses {
		verses[i].Text = SanitizeText(verses[i].Text)
		verses[i].Translation = SanitizeText(verses[i].Translation)
	}
}

type SearchResponse struct {
	ExactMatches int     `json:"exact_matches"`
	Total        int     `json:"total"`
	Results      []Verse `json:"results"`
}

func (c *Client) GetTranslations() ([]Translation, error) {
	var languageGroups []LanguageGroup
	if err := c.getJSON(baseURL+"/static/bolls/app/views/languages.json", &languageGroups); err != nil {
		return nil, err
	}
	return englishBuiltins(languageGroups), nil
}

// englishBuiltins keeps the public-domain English translations, in catalog
// order. Copyrighted names are dropped even when the server lists them.
func englishBuiltins(groups []LanguageGroup) []Translation {
	byName := make(map[string]Translation)
	for _, group := range groups {
		if group.Language != "English" {
			continue
		}
		for _, t := range group.Translations {
			if !translations.IsBuiltin(t.ShortName) {
				continue
			}
			t.FullName = SanitizeText(t.FullName)
			byName[t.ShortName] = t
		}
		break
	}
	out := make([]Translation, 0, len(byName))
	for _, name := range translations.ShortNames() {
		if t, ok := byName[name]; ok {
			out = append(out, t)
		}
	}
	return out
}

func (c *Client) GetBooks(translation string) ([]Book, error) {
	if err := requireBuiltin(translation); err != nil {
		return nil, err
	}
	var books []Book
	if err := c.getJSON(fmt.Sprintf("%s/get-books/%s/", baseURL, translation), &books); err != nil {
		return nil, err
	}
	for i := range books {
		books[i].Name = SanitizeText(books[i].Name)
	}
	return books, nil
}

func (c *Client) GetChapter(translation string, book, chapter int) ([]Verse, error) {
	if err := requireBuiltin(translation); err != nil {
		return nil, err
	}
	// Try cache first if available. A cache file that can't be read
	// (e.g. truncated) falls back to the API instead of failing forever.
	if c.cache != nil && c.cache.IsCached(translation) {
		if verses, err := c.cache.GetChapter(translation, book, chapter); err == nil {
			return verses, nil
		}
	}

	var verses []Verse
	if err := c.getJSON(fmt.Sprintf("%s/get-text/%s/%d/%d/", baseURL, translation, book, chapter), &verses); err != nil {
		return nil, err
	}
	sanitizeVerses(verses)
	return verses, nil
}

func (c *Client) GetParallelVerses(req ParallelVerseRequest) (map[string][]Verse, error) {
	for _, t := range req.Translations {
		if err := requireBuiltin(t); err != nil {
			return nil, err
		}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, baseURL+"/get-parallel-verses/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// The response is one verse list per requested translation, in order.
	var rawResponse [][]Verse
	if err := c.doJSON(httpReq, &rawResponse); err != nil {
		return nil, err
	}

	// Convert to map for easier access
	result := make(map[string][]Verse)
	for i, translation := range req.Translations {
		if i < len(rawResponse) {
			sanitizeVerses(rawResponse[i])
			result[translation] = rawResponse[i]
		}
	}

	return result, nil
}

func (c *Client) SearchVerses(translation, query string) (*SearchResponse, error) {
	if err := requireBuiltin(translation); err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("search", query)
	params.Set("limit", "500")

	var searchResp SearchResponse
	if err := c.getJSON(fmt.Sprintf("%s/v2/find/%s?%s", baseURL, translation, params.Encode()), &searchResp); err != nil {
		return nil, err
	}
	sanitizeVerses(searchResp.Results)

	return &searchResp, nil
}
