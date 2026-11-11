package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"omasword/internal/api"
	"omasword/internal/settings"

	tea "charm.land/bubbletea/v2"
)

var errBusy = errors.New("bolls.life is busy (too many requests)")

func sizedModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	return next.(Model)
}

func TestListRetryDelayBacksOff(t *testing.T) {
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second, 30 * time.Second}
	for i, w := range want {
		if got := listRetryDelay(i); got != w {
			t.Errorf("attempt %d: %v, want %v", i, got, w)
		}
	}
}

func TestBooksFailureShowsErrorAndRetries(t *testing.T) {
	m := sizedModel(t)
	next, cmd := m.Update(booksFailedMsg{errBusy})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("a failed books load should schedule a retry")
	}
	if view := m.renderView(); !strings.Contains(view, "Couldn't load books") {
		t.Fatal("books pane should say the load failed, not Loading…")
	}

	next, _ = m.Update(booksLoadedMsg{[]api.Book{{BookID: 1, Name: "Genesis", Chapters: 50}}})
	m = next.(Model)
	if m.booksErr != nil || m.booksRetries != 0 {
		t.Fatalf("a successful load should reset the retry state: err %v retries %d", m.booksErr, m.booksRetries)
	}
}

func TestChapterSuccessDoesNotHideBooksFailure(t *testing.T) {
	m := sizedModel(t)
	next, _ := m.Update(booksFailedMsg{errBusy})
	next, _ = next.(Model).Update(chapterLoadedMsg{[]api.Verse{{Verse: 1, Text: "x"}}})
	if m := next.(Model); m.booksErr == nil {
		t.Fatal("the chapter loading must not clear the books failure")
	}
}

func TestTitleUsesRealBookNameBeforeBooksLoad(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := settings.Save(settings.Settings{SelectedTranslation: "ESV", CurrentBook: 48, CurrentChapter: 4}); err != nil {
		t.Fatal(err)
	}
	if m := NewModel(); m.currentBookName != "Galatians" {
		t.Fatalf("book 48 before books load = %q, want Galatians", m.currentBookName)
	}
}

func TestCurrentChapterFailureRetries(t *testing.T) {
	m := sizedModel(t)
	m.currentBook, m.currentChapter = 48, 4

	next, cmd := m.Update(chapterFailedMsg{book: 48, chapter: 4, err: errBusy})
	m = next.(Model)
	if cmd == nil || m.err == nil {
		t.Fatal("a failed current chapter should show the error and schedule a retry")
	}

	// A late failure for a chapter you've already left is not retried.
	if _, cmd := m.Update(chapterFailedMsg{book: 1, chapter: 1, err: errBusy}); cmd != nil {
		t.Fatal("retried a chapter that is no longer current")
	}
	if _, cmd := m.Update(retryChapterMsg{book: 1, chapter: 1}); cmd != nil {
		t.Fatal("reloaded a chapter that is no longer current")
	}
}
