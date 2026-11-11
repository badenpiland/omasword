package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/settings"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

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

func TestOutputNeverContainsTheAPIKey(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := esv.SaveKey("test-key-not-real"); err != nil {
		t.Fatal(err)
	}
	src := &fakeESV{key: true, page: esv.Passage{
		Verses: []api.Verse{{Verse: 35, Text: "Jesus wept."}},
		First:  35, Last: 35, Total: 57,
		Notice: esv.CopyrightNotice,
	}, err: errString("rejected test-key-not-real")}
	opt := options{
		passage: "John 11:35", translation: "ESV", explicit: true, json: true,
		key: "test-key-not-real",
	}
	var out, errBuf bytes.Buffer
	// A fetch error that echoes the key must be redacted.
	if code := execute(&out, &errBuf, opt, &fakeBolls{}, src); code != ExitFetch {
		t.Fatalf("code %d", code)
	}
	if strings.Contains(out.String()+errBuf.String(), "test-key-not-real") {
		t.Fatal("fetch error printed the API key")
	}
	if !strings.Contains(errBuf.String(), "[redacted]") {
		t.Fatal("expected the key to be redacted")
	}

	src.err = nil
	out.Reset()
	errBuf.Reset()
	if code := execute(&out, &errBuf, opt, &fakeBolls{}, src); code != ExitOK {
		t.Fatalf("code %d stderr %s", code, errBuf.String())
	}
	if strings.Contains(out.String(), "test-key-not-real") || strings.Contains(errBuf.String(), "test-key-not-real") {
		t.Fatal("JSON output contains the API key")
	}
	if !strings.Contains(out.String(), "Jesus wept.") || !strings.Contains(out.String(), "esv.org") {
		t.Fatalf("passage JSON = %s", out.String())
	}
}

func TestFlagsAfterPositionalReference(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	args := []string{"Psalm 23:1-3", "--translation", "ESV", "--json"}
	opt, err := parseArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	if opt.passage != "Psalm 23:1-3" || opt.translation != "ESV" || !opt.json || !opt.explicit {
		t.Fatalf("parsed %+v", opt)
	}
	got, err := parsePassage(opt.passage)
	if err != nil {
		t.Fatal(err)
	}
	if got.book != 19 || got.chapter != 23 || got.start != 1 || got.end != 3 || got.whole {
		t.Fatalf("reference = %+v", got)
	}
	var out, errBuf bytes.Buffer
	if code := RunWith(&out, &errBuf, args); code != ExitNotAllowed {
		t.Fatalf("code %d, stderr %s, stdout %s", code, errBuf.String(), out.String())
	}
	if strings.Contains(errBuf.String(), "bad verse") || strings.Contains(errBuf.String(), "--json") {
		t.Fatalf("flags were glued into the reference: %s", errBuf.String())
	}

	opt, err = parseArgs([]string{"--json", "--translation=WEB", "John", "3:16"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.passage != "John 3:16" || opt.translation != "WEB" || !opt.json {
		t.Fatalf("flags before the reference parsed as %+v", opt)
	}
}

func TestUsageErrorsExit1(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cases := [][]string{
		nil,
		{"--passage"},
		{"--no-such-flag"},
		{"--passage", "Not A Book 1:1"},
		{"--passage", "John 3:0"},
		{"--passage", "John 3:16", "--search", "love"},
		{"Psalm 23:1-3", "--translation"},
	}
	for _, args := range cases {
		var out, errBuf bytes.Buffer
		code := RunWith(&out, &errBuf, args)
		if code != ExitUsage {
			t.Errorf("%q exit %d, want %d; stderr %s", args, code, ExitUsage, errBuf.String())
		}
		if code == ExitOK {
			t.Errorf("%q exited 0", args)
		}
	}
}

func TestEmptyRequestedVerseExits3(t *testing.T) {
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	src := &fakeESV{key: true, page: esv.Passage{
		Verses: []api.Verse{{Verse: 5, Text: "Jesus answered."}},
		First:  5, Last: 5, Total: 36,
		Notice: esv.CopyrightNotice,
	}}
	opt := options{passage: "John 3:16", translation: "ESV", explicit: true, json: true}
	var out, errBuf bytes.Buffer
	code := execute(&out, &errBuf, opt, &fakeBolls{}, src)
	if code != ExitFetch {
		t.Fatalf("code %d, stdout %q stderr %q", code, out.String(), errBuf.String())
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Fatalf("empty verse still printed a body: %s", out.String())
	}
	if !strings.Contains(errBuf.String(), "no text") {
		t.Fatalf("stderr = %s", errBuf.String())
	}
}

func TestWantsCLI(t *testing.T) {
	if WantsCLI(nil) || WantsCLI([]string{"--version"}) {
		t.Fatal("the TUI and --version are not the CLI mode")
	}
	if !WantsCLI([]string{"John", "3:16"}) || !WantsCLI([]string{"--search", "love"}) || !WantsCLI([]string{"-h"}) {
		t.Fatal("passage, search, and help should select CLI mode")
	}
}

func TestReaderStyleReferences(t *testing.T) {
	cases := map[string]string{
		"rom 8:28":          "Romans 8:28",
		"1jn 1:9":           "1 John 1:9",
		"ps 23 1-3":         "Psalms 23:1-3",
		"Psalm 23":          "Psalms 23",
		"song of solomon 2": "Song of Solomon 2",
		"John 3:5-2":        "John 3:2-5",
	}
	for in, want := range cases {
		got, err := parsePassage(in)
		if err != nil || got.label != want {
			t.Errorf("parsePassage(%q) = %q, %v; want %q", in, got.label, err, want)
		}
	}
}

func TestTranslationNameIgnoresCase(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	opt, err := parseArgs([]string{"John 3:16", "--translation", "kjv"})
	if err != nil || opt.translation != "KJV" {
		t.Fatalf("translation %q, %v", opt.translation, err)
	}
}

func TestBSBRequestDoesNotLookUpTheKey(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	if err := esv.SaveKey("test-key-not-real"); err != nil {
		t.Fatal(err)
	}
	opt, err := parseArgs([]string{"John 3:16", "--translation", "BSB"})
	if err != nil || opt.key != "" {
		t.Fatalf("key looked up for BSB (%v)", err)
	}
}

func TestStrongsNumbersAreStripped(t *testing.T) {
	bolls := &fakeBolls{
		search: &api.SearchResponse{Total: 1, Results: []api.Verse{
			{Book: 62, Chapter: 5, Verse: 18, Text: "he that is <mark>begotten</mark><S>1080</S> of<S>1537</S> God<S>2316</S>"},
		}},
	}
	opt := options{search: "begotten", translation: "KJV", explicit: true}
	var out, errBuf bytes.Buffer
	if code := execute(&out, &errBuf, opt, bolls, &fakeESV{}); code != ExitOK {
		t.Fatalf("code %d: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "1 John 5:18 he that is begotten of God") {
		t.Errorf("Strong's numbers or tags left in: %s", got)
	}
}
