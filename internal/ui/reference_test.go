package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"sword-tui/internal/api"
)

var testBooks = []api.Book{
	{BookID: 1, Name: "Genesis", Chapters: 50},
	{BookID: 43, Name: "John", Chapters: 21},
}

func TestValidateReference(t *testing.T) {
	tests := []struct {
		name                   string
		book, ch, vs, ve       int
		wantErr                bool
		wantCh, wantVs, wantVe int
	}{
		{name: "valid", book: 43, ch: 3, vs: 16, ve: 16, wantCh: 3, wantVs: 16, wantVe: 16},
		{name: "chapter too high", book: 1, ch: 99, wantErr: true},
		{name: "chapter zero", book: 1, ch: 0, wantErr: true},
		{name: "unknown book", book: 999, ch: 1, wantErr: true},
		{name: "reversed range", book: 1, ch: 1, vs: 5, ve: 2, wantCh: 1, wantVs: 2, wantVe: 5},
		{name: "last chapter", book: 1, ch: 50, wantCh: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ch, vs, ve, err := validateReference(testBooks, tt.book, tt.ch, tt.vs, tt.ve)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s %d:%d-%d", b.Name, ch, vs, ve)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ch != tt.wantCh || vs != tt.wantVs || ve != tt.wantVe {
				t.Fatalf("got %d:%d-%d, want %d:%d-%d", ch, vs, ve, tt.wantCh, tt.wantVs, tt.wantVe)
			}
		})
	}
}

func TestLookupRejectsOutOfRangeChapter(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	m.books = testBooks
	m.mode = modeSearch
	m.textInput.SetValue("gen 99")
	next, _ := m.Update(keyEnter())
	got := next.(Model)
	if got.mode != modeSearch {
		t.Fatal("lookup panel should stay open on an invalid reference")
	}
	if got.currentChapter == 99 {
		t.Fatal("invalid chapter must not become the reading position")
	}
	if got.searchErr == nil {
		t.Fatal("expected an explanation in the lookup panel")
	}
}
func keyEnter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
