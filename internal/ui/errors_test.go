package ui

import (
	"errors"
	"testing"

	"omasword/internal/api"
)

func TestSuccessfulLoadClearsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	next, _ := m.Update(errMsg{errors.New("network down")})
	m = next.(Model)
	if m.err == nil {
		t.Fatal("error should be recorded")
	}
	next, _ = m.Update(chapterLoadedMsg{verses: []api.Verse{{Verse: 1, Text: "x", Book: 1, Chapter: 1}}})
	m = next.(Model)
	if m.err != nil {
		t.Fatalf("error should clear after a successful load, still %v", m.err)
	}
}
