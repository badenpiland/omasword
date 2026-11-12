package ui

import (
	"errors"
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/esv"

	"github.com/charmbracelet/x/ansi"
)

// esvReader is the reader on Exodus 1 in the ESV, with five verses on
// screen, verse 1 highlighted, and an in-memory ESV cache.
func esvReader(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	client, err := esv.New("test-key-not-real", "")
	if err != nil {
		t.Fatal(err)
	}
	m.selectedTranslation, m.esvClient = esv.ID, client
	m.currentBook, m.currentBookName, m.currentChapter = 2, "Exodus", 1
	m.highlightedVerseStart, m.highlightedVerseEnd = 0, 0
	var verses []api.Verse
	for v := 1; v <= 5; v++ {
		verses = append(verses, api.Verse{Verse: v, Text: "ESV fixture verse"})
	}
	next, _ := m.Update(chapterLoadedMsg{verses: verses, book: 2, chapter: 1, esvFirst: 1, esvLast: 5, esvTotal: 5})
	return next.(Model)
}

func plain(s string) string {
	return strings.Join(strings.Fields(ansi.Strip(s)), " ")
}

func TestSSavesOnlyTheSelection(t *testing.T) {
	m := esvReader(t)
	if m.highlightedVerseStart != 1 || m.highlightedVerseEnd != 1 {
		t.Fatalf("selection %d-%d", m.highlightedVerseStart, m.highlightedVerseEnd)
	}
	m, _ = press(t, m, "S")
	if n := m.esvClient.SavedCount(); n != 1 {
		t.Fatalf("saved %d verses, want 1", n)
	}
	if !strings.Contains(m.status, "Saved Exodus 1:1 ") || !strings.Contains(m.status, "1/500") {
		t.Fatalf("status %q", m.status)
	}
	if !strings.Contains(plain(m.content), "•1") || strings.Contains(plain(m.content), "•2") {
		t.Fatal("saved verse isn't marked, or an unsaved one is")
	}
	m.status = "" // the "Saved…" message shows first; the count is underneath
	if !strings.Contains(m.renderStatusBar(), "1 saved here") {
		t.Fatal("status bar doesn't count the saved verse")
	}

	// J extends the selection; S saves verses 1-3 (verse 1 isn't counted
	// twice); S again unsaves them.
	m = pressAll(t, m, "J", "J")
	if m.highlightedVerseStart != 1 || m.highlightedVerseEnd != 3 {
		t.Fatalf("after J J: %d-%d", m.highlightedVerseStart, m.highlightedVerseEnd)
	}
	m, _ = press(t, m, "S")
	if n := m.esvClient.SavedCount(); n != 3 {
		t.Fatalf("saved %d verses, want 3", n)
	}
	m, _ = press(t, m, "S")
	if n := m.esvClient.SavedCount(); n != 0 || !strings.Contains(m.status, "Unsaved Exodus 1:1–3") {
		t.Fatalf("saved %d after unsaving; status %q", n, m.status)
	}
}

func TestJAndKKeepTheStartingVerse(t *testing.T) {
	m := esvReader(t)
	m = pressAll(t, m, "j", "j") // verse 3
	m = pressAll(t, m, "K", "K") // 1-3, anchored at 3
	if m.highlightedVerseStart != 1 || m.highlightedVerseEnd != 3 {
		t.Fatalf("after K K: %d-%d", m.highlightedVerseStart, m.highlightedVerseEnd)
	}
	m, _ = press(t, m, "J") // shrinks back from the top
	if m.highlightedVerseStart != 2 || m.highlightedVerseEnd != 3 {
		t.Fatalf("after J: %d-%d", m.highlightedVerseStart, m.highlightedVerseEnd)
	}
}

func TestSavedListOpensAndRemoves(t *testing.T) {
	m := esvReader(t)
	if err := m.esvClient.SaveVerses(43, 3, []api.Verse{{Verse: 16, Text: "x"}, {Verse: 17, Text: "y"}}); err != nil {
		t.Fatal(err)
	}
	m, _ = press(t, m, "S") // Exodus 1:1
	m, _ = press(t, m, "o")
	if m.mode != modeSaved || len(m.savedList) != 2 {
		t.Fatalf("mode %v list %+v", m.mode, m.savedList)
	}
	panel := plain(m.renderSaved())
	if !strings.Contains(panel, "Exodus 1:1") || !strings.Contains(panel, "John 3:16–17") || !strings.Contains(panel, "3 of 500") {
		t.Fatalf("panel %q", panel)
	}
	// Remove Exodus 1 (first in canonical order); its marker goes too.
	m, _ = press(t, m, "x")
	if len(m.savedList) != 1 || m.savedHere[1] {
		t.Fatalf("after x: list %+v savedHere %v", m.savedList, m.savedHere)
	}
	m, cmd := press(t, m, "enter")
	if m.mode != modeReader || cmd == nil || m.currentBook != 43 || m.currentChapter != 3 ||
		m.highlightedVerseStart != 16 || m.highlightedVerseEnd != 17 {
		t.Fatalf("open: mode %v book %d:%d sel %d-%d", m.mode, m.currentBook, m.currentChapter, m.highlightedVerseStart, m.highlightedVerseEnd)
	}
}

func TestOfflineShowsSavedVersesAsESV(t *testing.T) {
	m := esvReader(t)
	next, cmd := m.Update(chapterLoadedMsg{
		verses: []api.Verse{{Verse: 1, Text: "Saved ESV verse"}},
		book:   2, chapter: 1, fallbackErr: errors.New("dial tcp: network is unreachable"),
	})
	m = next.(Model)
	if !m.showingESV() || !m.degraded() || cmd == nil {
		t.Fatalf("showingESV %v degraded %v retry %v", m.showingESV(), m.degraded(), cmd != nil)
	}
	if text := plain(m.content); !strings.Contains(text, "verses you saved") || !strings.Contains(text, "esv.org") {
		t.Fatalf("content %q", text)
	}
	if !strings.Contains(m.renderStatusBar(), "saved verses only") {
		t.Fatal("status bar doesn't say only saved verses are shown")
	}
}

func TestBSBStandsInWhenNothingIsSaved(t *testing.T) {
	m := esvReader(t)
	next, cmd := m.Update(chapterLoadedMsg{
		verses: []api.Verse{{Verse: 1, Text: "These are the names"}},
		book:   2, chapter: 1,
		fallback: "BSB", fallbackErr: errors.New("dial tcp: network is unreachable"),
	})
	m = next.(Model)
	if m.readerTranslation() != "BSB" || m.showingESV() || cmd == nil {
		t.Fatalf("on screen %s showingESV %v retry %v", m.readerTranslation(), m.showingESV(), cmd != nil)
	}
	if strings.Contains(m.content, "Crossway") || strings.Contains(m.content, "esv.org") {
		t.Fatal("the ESV notice is on BSB text")
	}
	if !strings.Contains(plain(m.content), "this is the BSB") || !strings.Contains(m.renderHeader(), "BSB") {
		t.Fatal("stand-in isn't labeled as the BSB")
	}
	if _, cmd := press(t, m, "."); cmd != nil {
		t.Fatal("paged the BSB stand-in as ESV")
	}
	if m, _ := press(t, m, "S"); m.esvClient.SavedCount() != 0 || !strings.Contains(m.status, "isn't loaded") {
		t.Fatalf("saved BSB text as ESV; status %q", m.status)
	}

	// The ESV comes back on a retry and replaces the BSB.
	next, _ = m.Update(chapterLoadedMsg{
		verses: []api.Verse{{Verse: 1, Text: "These are the names"}},
		book:   2, chapter: 1, esvFirst: 1, esvLast: 1, esvTotal: 22,
	})
	m = next.(Model)
	if m.degraded() || !m.showingESV() || !strings.Contains(m.content, "esv.org") {
		t.Fatalf("degraded %v showingESV %v", m.degraded(), m.showingESV())
	}
}

func TestAMissingKeyIsNotHiddenByAStandIn(t *testing.T) {
	m := esvReader(t)
	client, err := esv.New("", "")
	if err != nil {
		t.Fatal(err)
	}
	m.esvClient = client
	msg := m.loadReadingChapter(43, 3)()
	if failed, ok := msg.(chapterFailedMsg); !ok || !errors.Is(failed.err, esv.ErrNoKey) {
		t.Fatalf("got %#v", msg)
	}
}

func TestSavingIsForTheESV(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	m.selectedTranslation = "BSB"
	m, _ = press(t, m, "S")
	if !strings.Contains(m.status, "ESV") {
		t.Fatalf("S on the BSB: %q", m.status)
	}
}

func TestVerseRanges(t *testing.T) {
	if got := verseRanges([]int{1, 2, 3, 7, 9, 10}); got != "1–3, 7, 9–10" {
		t.Fatalf("got %q", got)
	}
}
