package ui

import (
	"strings"
	"testing"

	"omasword/internal/api"

	tea "charm.land/bubbletea/v2"
)

func pickerModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = next.(Model)
	m.books = []api.Book{
		{BookID: 1, Name: "Genesis", Chapters: 50},
		{BookID: 2, Name: "Exodus", Chapters: 40},
		{BookID: 43, Name: "John", Chapters: 21},
	}
	m.currentBook, m.currentBookName, m.currentChapter = 1, "Genesis", 1
	m.currentVerses = []api.Verse{{Verse: 1, Text: "In the beginning"}}
	return m
}

// press sends one key without running the returned command (which may
// hit the network) and hands the command back.
func press(t *testing.T, m Model, key string) (Model, tea.Cmd) {
	t.Helper()
	var msg tea.KeyPressMsg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		r := []rune(key)[0]
		msg = tea.KeyPressMsg{Code: r, Text: key}
	}
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func pressAll(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		m, _ = press(t, m, k)
	}
	return m
}

func TestPickerIsDrawn(t *testing.T) {
	m := pressAll(t, pickerModel(t), "v")
	view := m.renderView()
	for _, want := range []string{"BOOKS", "CHAPTERS", "VERSES", "Genesis", "In the beginning"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker view is missing %q", want)
		}
	}
}

func TestPickerOpensFilteredVerse(t *testing.T) {
	m := pressAll(t, pickerModel(t), "v", "/", "j", "o", "h", "enter", "l")
	if m.pickerBook != 43 || m.pickerColumn != pickerColChapters {
		t.Fatalf("after filtering to John: book %d column %d", m.pickerBook, m.pickerColumn)
	}

	m, cmd := press(t, m, "l")
	if cmd == nil {
		t.Fatal("entering the verses column should load John 1")
	}
	next, _ := m.Update(pickerVersesMsg{book: 43, chapter: 1, verses: []api.Verse{
		{Verse: 1, Text: "In the beginning was the Word"},
		{Verse: 2, Text: "He was in the beginning with God."},
	}})
	m = pressAll(t, next.(Model), "j")

	// Browsing must not touch the reader.
	if m.currentBook != 1 || m.currentChapter != 1 {
		t.Fatalf("reader moved to %d:%d while browsing", m.currentBook, m.currentChapter)
	}

	m, cmd = press(t, m, "enter")
	if cmd == nil {
		t.Fatal("enter on a verse should open it")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.mode != modeReader || m.currentBook != 43 || m.currentChapter != 1 || m.highlightedVerseStart != 2 {
		t.Fatalf("opened mode %d at %d:%d v%d, want reader at John 1:2",
			m.mode, m.currentBook, m.currentChapter, m.highlightedVerseStart)
	}
}

func TestPickerOwnsTheKeyboard(t *testing.T) {
	m := pressAll(t, pickerModel(t), "v", "n", "c", "t")
	if m.mode != modePicker || m.currentChapter != 1 {
		t.Fatalf("reader shortcuts acted behind the picker: mode %d chapter %d", m.mode, m.currentChapter)
	}

	// While filtering, letters go to the filter, including q and j.
	m = pressAll(t, m, "/", "q", "j")
	if got := m.pickerFilterInput.Value(); got != "qj" {
		t.Fatalf("filter = %q, want %q", got, "qj")
	}

	m = pressAll(t, m, "esc", "esc")
	if m.mode != modeReader {
		t.Fatalf("esc should close the picker, mode = %d", m.mode)
	}
}

func TestPickerIgnoresStaleVerses(t *testing.T) {
	m := pressAll(t, pickerModel(t), "v", "j", "l", "l") // Exodus 1
	next, _ := m.Update(pickerVersesMsg{book: 1, chapter: 1, verses: []api.Verse{{Verse: 1, Text: "stale"}}})
	m = next.(Model)
	if m.pickerVersesReady() {
		t.Fatal("a reply for a chapter no longer selected was accepted")
	}
}
