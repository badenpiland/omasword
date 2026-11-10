package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"omasword/internal/api"
)

func TestOmarchyThemeChangeRedrawsChapterText(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := filepath.Join(state, "omarchy", "current", "theme")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(fg string) {
		data := "foreground = \"" + fg + "\"\nbackground = \"#101010\"\n"
		if err := os.WriteFile(filepath.Join(dir, "colors.toml"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("#aaaaaa")

	m := NewModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	next, _ = m.Update(chapterLoadedMsg{verses: []api.Verse{{Verse: 1, Text: "In the beginning", Book: 1, Chapter: 1}}})
	m = next.(Model)
	before := m.content

	write("#ff0000")
	next, _ = m.Update(omarchyTickMsg{})
	m = next.(Model)

	if m.content == before {
		t.Fatal("chapter text was not redrawn after the Omarchy theme changed")
	}
}
