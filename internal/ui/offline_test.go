package ui

import (
	"errors"
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/esv"

	"github.com/charmbracelet/x/ansi"
)

func TestBSBStandsInForAnUnreachableESVChapter(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	client, err := esv.New("test-key-not-real", "")
	if err != nil {
		t.Fatal(err)
	}
	m.selectedTranslation, m.esvClient = esv.ID, client

	next, cmd := m.Update(chapterLoadedMsg{
		verses: []api.Verse{{Verse: 1, Text: "In the beginning God created"}},
		book:   1, chapter: 1,
		fallback: "BSB", fallbackErr: errors.New("dial tcp: network is unreachable"),
	})
	m = next.(Model)
	if m.readerTranslation() != "BSB" || m.showingESV() {
		t.Fatalf("on screen %s, showingESV %v", m.readerTranslation(), m.showingESV())
	}
	if cmd == nil {
		t.Fatal("no retry of the ESV was scheduled")
	}
	if strings.Contains(m.content, "Crossway") || strings.Contains(m.content, "esv.org") {
		t.Fatal("the ESV notice is on BSB text")
	}
	if plain := strings.Join(strings.Fields(ansi.Strip(m.content)), " "); !strings.Contains(plain, "this is the BSB") {
		t.Fatalf("stand-in isn't labeled: %q", m.content)
	}
	if !strings.Contains(m.renderHeader(), "BSB") {
		t.Fatal("header doesn't name the BSB")
	}
	if !strings.Contains(m.renderStatusBar(), "ESV offline") {
		t.Fatal("status bar doesn't say the ESV is offline")
	}
	// ESV paging doesn't apply to the stand-in.
	if _, cmd := press(t, m, "."); cmd != nil {
		t.Fatal("paged the BSB stand-in as ESV")
	}
	if m, _ := press(t, m, "S"); !strings.Contains(m.status, "connect") {
		t.Fatalf("save ahead offline: %q", m.status)
	}

	// The ESV comes back on a retry and replaces the BSB.
	next, _ = m.Update(chapterLoadedMsg{
		verses: []api.Verse{{Verse: 1, Text: "In the beginning, God created"}},
		book:   1, chapter: 1, esvFirst: 1, esvLast: 1, esvTotal: 31, esvSaved: true,
	})
	m = next.(Model)
	if m.fallback != "" || !m.showingESV() || !m.esvSaved {
		t.Fatalf("fallback %q showingESV %v saved %v", m.fallback, m.showingESV(), m.esvSaved)
	}
	if !strings.Contains(m.content, "esv.org") {
		t.Fatal("ESV text without the Crossway notice")
	}
	if !strings.Contains(m.renderStatusBar(), "saved") {
		t.Fatal("status bar doesn't show the page is saved")
	}
}

func TestAMissingKeyIsNotHiddenByTheBSB(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	m.selectedTranslation = esv.ID
	client, err := esv.New("", "")
	if err != nil {
		t.Fatal(err)
	}
	m.esvClient = client
	msg := m.loadReadingChapter(43, 3)()
	failed, ok := msg.(chapterFailedMsg)
	if !ok || !errors.Is(failed.err, esv.ErrNoKey) {
		t.Fatalf("got %#v", msg)
	}
}

func TestPinAndSaveAheadAreForTheESV(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	m.selectedTranslation = "BSB"
	m, _ = press(t, m, "P")
	if !strings.Contains(m.status, "ESV") {
		t.Fatalf("P on the BSB: %q", m.status)
	}
	m, cmd := press(t, m, "S")
	if cmd != nil || !strings.Contains(m.status, "ESV") {
		t.Fatalf("S on the BSB: %q", m.status)
	}

	// An ESV page that isn't saved can't be pinned.
	client, err := esv.New("test-key-not-real", "")
	if err != nil {
		t.Fatal(err)
	}
	m.selectedTranslation, m.esvClient = esv.ID, client
	m, _ = press(t, m, "P")
	if !strings.Contains(m.status, "isn't saved") || m.esvPinned {
		t.Fatalf("pin unsaved page: %q pinned %v", m.status, m.esvPinned)
	}

	// Moving to another chapter clears the message.
	next, _ := m.Update(chapterLoadedMsg{verses: []api.Verse{{Verse: 1, Text: "x"}}, book: 1, chapter: 2})
	if s := next.(Model).status; s != "" {
		t.Fatalf("status kept after moving: %q", s)
	}
}

func TestSaveAheadStatus(t *testing.T) {
	for _, tc := range []struct {
		res  esv.SaveResult
		err  error
		want string
	}{
		{esv.SaveResult{Fetched: 14, ThroughBook: 43, ThroughChapter: 15, Full: true, BookLimit: true, FullBook: 43}, nil,
			"Saved through John 15 (14 new) · Crossway allows under half of John"},
		{esv.SaveResult{Fetched: 3, ThroughBook: 19, ThroughChapter: 9, Full: true, FullBook: 19}, nil,
			"Saved through Psalms 9 (3 new) · offline space (500 verses) is full"},
		{esv.SaveResult{AlreadySaved: 2, ThroughBook: 43, ThroughChapter: 3}, nil,
			"Saved through John 3"},
		{esv.SaveResult{}, &esv.RateError{RetryAfter: 30e9}, "Nothing new saved · stopped: ESV rate limit reached; try again in 30s"},
	} {
		if got := saveAheadStatus(tc.res, tc.err); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
