package ui

import (
	"testing"

	"omasword/internal/api"
	"omasword/internal/settings"
)

func TestChapterLoadSavesPosition(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	m.currentBook, m.currentChapter = 43, 3

	next, _ := m.Update(chapterLoadedMsg{verses: []api.Verse{{Verse: 16, Text: "For God so loved", Book: 43, Chapter: 3}}})
	_ = next

	got, err := settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentBook != 43 || got.CurrentChapter != 3 {
		t.Fatalf("saved position = %d:%d, want 43:3 without quitting", got.CurrentBook, got.CurrentChapter)
	}
}
