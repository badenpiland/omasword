package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func pressRunes(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		next, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(Model)
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Fatalf("typing %q quit the app at %q", s, string(r))
			}
		}
	}
	return m
}

func TestTypingQInWordSearchDoesNotQuit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	m.mode = modeWordSearch
	m.wordSearchInput.Focus()
	m = pressRunes(t, m, "quiet")
	if got := m.wordSearchInput.Value(); got != "quiet" {
		t.Fatalf("word search input = %q, want %q", got, "quiet")
	}
}

func TestTypingQInVerseLookupDoesNotQuit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	m.mode = modeSearch
	m.textInput.Focus()
	m = pressRunes(t, m, "q")
	if got := m.textInput.Value(); got != "q" {
		t.Fatalf("verse lookup input = %q, want %q", got, "q")
	}
}

func TestQQuitsFromReader(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q in the reader should quit")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Fatal("q in the reader should quit")
	}
}
