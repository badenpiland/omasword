package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/settings"
)

type fakeBolls struct {
	verses []api.Verse
	search *api.SearchResponse
	err    error
	calls  int
}

func (f *fakeBolls) GetChapter(string, int, int) ([]api.Verse, error) {
	f.calls++
	return f.verses, f.err
}

func (f *fakeBolls) SearchVerses(string, string) (*api.SearchResponse, error) {
	f.calls++
	return f.search, f.err
}

type fakeESV struct {
	key    bool
	page   esv.Passage
	search *api.SearchResponse
	err    error
	calls  int
}

func (f *fakeESV) HasKey() bool { return f.key }
func (f *fakeESV) Notice() string {
	if f.page.Notice != "" {
		return f.page.Notice
	}
	return esv.CopyrightNotice
}
func (f *fakeESV) Chapter(int, int, int) (esv.Passage, error) {
	f.calls++
	return f.page, f.err
}
func (f *fakeESV) Search(string) (*api.SearchResponse, error) {
	f.calls++
	return f.search, f.err
}

func TestHelpDocumentsTheContract(t *testing.T) {
	var out, err bytes.Buffer
	if code := RunWith(&out, &err, []string{"--help"}); code != ExitOK {
		t.Fatalf("code %d, stderr %s", code, err.String())
	}
	text := out.String()
	for _, want := range []string{"--passage", "--search", "--json", "--translation", "Exit codes", "api.esv.org", "Public-domain", "500"} {
		if !strings.Contains(text, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestUsageAndDisallowedTranslations(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	var out, err bytes.Buffer
	if code := RunWith(&out, &err, nil); code != ExitUsage {
		t.Fatalf("code %d", code)
	}
	if code := RunWith(&out, &err, []string{"--translation", "NLT", "--passage", "John 3:16"}); code != ExitNotAllowed {
		t.Fatalf("NLT code %d, stderr %s", code, err.String())
	}
	if strings.Contains(err.String(), "http") && strings.Contains(err.String(), "bolls") {
		t.Fatal("a refused translation should not mention a fetch")
	}
	err.Reset()
	if code := RunWith(&out, &err, []string{"--translation", "ESV", "--passage", "John 3:16"}); code != ExitNotAllowed {
		t.Fatalf("ESV without a key: code %d, stderr %s", code, err.String())
	}
	if !strings.Contains(err.String(), "api.esv.org") {
		t.Fatalf("stderr = %s", err.String())
	}
}

func TestPassagePlainAndJSON(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	bolls := &fakeBolls{verses: []api.Verse{
		{Verse: 16, Text: "For God so loved the world."},
		{Verse: 17, Text: "For God did not send His Son."},
	}}
	opt := options{passage: "John 3:16-17", translation: "BSB", explicit: true}
	var out, err bytes.Buffer
	if code := execute(&out, &err, opt, bolls, &fakeESV{}); code != ExitOK {
		t.Fatalf("code %d stderr %s", code, err.String())
	}
	if !strings.Contains(out.String(), "BSB John 3:16-17") || !strings.Contains(out.String(), "For God so loved") {
		t.Fatalf("plain = %s", out.String())
	}
	if strings.Contains(out.String(), "did not send") == false {
		t.Fatal("range dropped verse 17")
	}
	if bolls.calls != 1 {
		t.Fatalf("calls %d", bolls.calls)
	}

	opt.json = true
	out.Reset()
	if code := execute(&out, &err, opt, bolls, &fakeESV{}); code != ExitOK {
		t.Fatal(err.String())
	}
	var doc passageDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Translation != "BSB" || doc.Book != 43 || doc.Chapter != 3 || len(doc.Verses) != 2 {
		t.Fatalf("doc = %+v", doc)
	}
	if doc.Notice != "" || doc.Link != "" {
		t.Fatal("public-domain output should not carry an ESV notice")
	}
	if doc.Verses[0].Book != 43 || doc.Verses[0].Text != "For God so loved the world." {
		t.Fatalf("verse = %+v", doc.Verses[0])
	}
}

func TestESVPassageIsClippedAndAttributed(t *testing.T) {
	verses := make([]api.Verse, 21)
	for i := range verses {
		verses[i] = api.Verse{Verse: i + 1, Text: "verse"}
	}
	src := &fakeESV{key: true, page: esv.Passage{
		Verses: verses, First: 1, Last: 21, Total: 21, MoreAfter: false,
		Notice: "Scripture quotations are from the ESV® Bible, © 2001 by Crossway.",
	}}
	opt := options{passage: "Obadiah 1", translation: "ESV", explicit: true}
	var out, err bytes.Buffer
	if code := execute(&out, &err, opt, &fakeBolls{}, src); code != ExitOK {
		t.Fatalf("code %d stderr %s", code, err.String())
	}
	if strings.Count(out.String(), "\n") > 0 && countVerseLines(out.String()) > esv.MaxVerses(31) {
		t.Fatalf("printed more than half of Obadiah:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Crossway") || !strings.Contains(out.String(), "esv.org") {
		t.Fatalf("missing copyright:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "…") {
		t.Fatal("a clipped passage should mark the omission")
	}

	opt.json = true
	out.Reset()
	if code := execute(&out, &err, opt, &fakeBolls{}, src); code != ExitOK {
		t.Fatal(err.String())
	}
	var doc passageDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Clipped || len(doc.Verses) != esv.MaxVerses(31) {
		t.Fatalf("clipped=%v verses=%d", doc.Clipped, len(doc.Verses))
	}
	if doc.Link != esv.Link || !strings.Contains(doc.Notice, "Crossway") {
		t.Fatalf("doc notice/link = %q %q", doc.Notice, doc.Link)
	}
}

func TestESVSearchClipsAShortBook(t *testing.T) {
	hits := make([]api.Verse, 12)
	for i := range hits {
		hits[i] = api.Verse{Book: 31, Chapter: 1, Verse: i + 1, Text: "aloof"}
	}
	hits[0].Text = "the <mark>rabble</mark> rose"
	src := &fakeESV{key: true, search: &api.SearchResponse{Total: 12, Results: hits}}
	opt := options{search: "aloof", translation: "ESV", explicit: true, json: true}
	var out, err bytes.Buffer
	if code := execute(&out, &err, opt, &fakeBolls{}, src); code != ExitOK {
		t.Fatal(err.String())
	}
	var doc searchDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Results) > esv.MaxVerses(31) {
		t.Fatalf("search returned %d verses", len(doc.Results))
	}
	if doc.Link != esv.Link {
		t.Fatalf("link = %q", doc.Link)
	}
	if doc.Results[0].Text != "the rabble rose" {
		t.Fatalf("search text = %q", doc.Results[0].Text)
	}
}

func TestFetchErrorAndSavedDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	if err := settings.Save(settings.Settings{SelectedTranslation: "WEB"}); err != nil {
		t.Fatal(err)
	}
	opt, err := parseArgs([]string{"--passage", "Psalm 23"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.translation != "WEB" || opt.passage != "Psalm 23" {
		t.Fatalf("opt = %+v", opt)
	}
	ref, err := parsePassage(opt.passage)
	if err != nil || ref.book != 19 || ref.chapter != 23 || !ref.whole {
		t.Fatalf("ref = %+v %v", ref, err)
	}

	bolls := &fakeBolls{err: errBusy}
	var out, errBuf bytes.Buffer
	if code := execute(&out, &errBuf, options{passage: "John 3:16", translation: "KJV", explicit: true}, bolls, &fakeESV{}); code != ExitFetch {
		t.Fatalf("code %d", code)
	}
}

var errBusy = errString("bolls.life is busy")

type errString string

func (e errString) Error() string { return string(e) }

func countVerseLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
			n++
		}
	}
	return n
}

func TestWantsCLI(t *testing.T) {
	if WantsCLI(nil) || WantsCLI([]string{"--version"}) {
		t.Fatal("the TUI and --version are not the CLI mode")
	}
	if !WantsCLI([]string{"John", "3:16"}) || !WantsCLI([]string{"--search", "love"}) || !WantsCLI([]string{"-h"}) {
		t.Fatal("passage, search, and help should select CLI mode")
	}
}
