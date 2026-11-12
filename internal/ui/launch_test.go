package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// On an 80x24 terminal the ? panel is taller than the window: it fits,
// says what's cut, and scrolls to the last shortcut.
func TestAboutScrollsOnASmallWindow(t *testing.T) {
	m := pickerModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m, _ = press(t, m, "?")
	panel := m.renderAbout()
	if h := lipgloss.Height(panel); h > 24 {
		t.Fatalf("panel is %d rows on a 24-row window", h)
	}
	if !strings.Contains(plain(panel), "more") || strings.Contains(plain(panel), "quit") {
		t.Fatalf("top of panel: %q", plain(panel))
	}
	for range 60 {
		m, _ = press(t, m, "j")
	}
	panel = m.renderAbout()
	if lipgloss.Height(panel) > 24 || !strings.Contains(plain(panel), "q quit") {
		t.Fatalf("scrolled panel: %q", plain(panel))
	}
	if !strings.Contains(plain(m.renderStatusBar()), "scroll") {
		t.Fatal("status bar doesn't say the panel scrolls")
	}

	// On a tall window it doesn't scroll.
	next, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 60})
	m = next.(Model)
	if m.aboutMaxScroll() != 0 || strings.Contains(plain(m.renderAbout()), "more") {
		t.Fatal("the panel scrolls on a tall window")
	}
}

// The chapter error keeps the reading pane as tall as the books pane.
func TestChapterErrorKeepsThePaneHeight(t *testing.T) {
	m := pickerModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	want := lipgloss.Height(m.renderLeftPane(leftPaneOuterWidth, 18))
	m.currentVerses, m.err = nil, errors.New("can't reach bolls.life")
	if got := lipgloss.Height(m.renderRightPane(50, 18)); got != want {
		t.Fatalf("error pane is %d rows, books pane %d", got, want)
	}
}
