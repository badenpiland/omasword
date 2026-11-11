package ui

import (
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/settings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// inComparison is the reader after "c" has shown a comparison.
func inComparison(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	m.mode = modeComparison
	next, _ := m.Update(parallelVersesLoadedMsg{map[string][]api.Verse{
		"BSB": {{Verse: 1, Text: "In the beginning"}},
		"KJV": {{Verse: 1, Text: "In the beginning"}},
		"WEB": {{Verse: 1, Text: "In the beginning"}},
	}})
	return next.(Model)
}

func TestLeavingComparisonRedrawsTheReader(t *testing.T) {
	for _, key := range []string{"esc", "r", "c"} {
		m := inComparison(t)
		if !strings.Contains(m.content, "KJV") {
			t.Fatalf("comparison not shown: %q", m.content)
		}
		m, cmd := press(t, m, key)
		if m.mode != modeReader || cmd == nil {
			t.Fatalf("%s: mode %v, reload command %v", key, m.mode, cmd != nil)
		}
		next, _ := m.Update(chapterLoadedMsg{verses: []api.Verse{{Verse: 1, Text: "In the beginning"}}})
		if got := next.(Model).content; strings.Contains(got, "KJV") {
			t.Fatalf("%s: comparison columns still on screen: %q", key, got)
		}
	}
}

func TestLeavingAColumnPickerWithRReloadsTheReader(t *testing.T) {
	m := inComparison(t)
	m.mode, m.comparisonPickerColumn = modeTranslationSelect, 1
	m, cmd := press(t, m, "r")
	if m.mode != modeReader || m.comparisonPickerColumn != -1 || cmd == nil {
		t.Fatalf("mode %v column %d reload %v", m.mode, m.comparisonPickerColumn, cmd != nil)
	}
}

func TestComparisonAsksForTheWholeChapter(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := pickerModel(t)
	m.currentVerses = nil
	for v := 1; v <= 176; v++ {
		m.currentVerses = append(m.currentVerses, api.Verse{Verse: v})
	}
	if got := m.comparisonVerseList(); len(got) != 176 {
		t.Fatalf("comparison asks for %d verses, want 176", len(got))
	}
	m, cmd := press(t, m, "c")
	if m.mode != modeComparison || cmd == nil {
		t.Fatalf("c did not open comparison")
	}
}

func TestComparisonColumnsAreASavedSetting(t *testing.T) {
	m := inComparison(t)
	if got := strings.Join(m.comparisonTranslations, ","); got != "BSB,KJV" {
		t.Fatalf("default columns = %s, want BSB,KJV", got)
	}
	m, _ = press(t, m, "+")
	m, _ = press(t, m, "+")
	m, _ = press(t, m, "+") // already at the maximum
	if got := strings.Join(m.comparisonTranslations, ","); got != "BSB,KJV,WEB,ASV" {
		t.Fatalf("after + + + = %s", got)
	}
	cfg, err := settings.Load()
	if err != nil || cfg.ComparisonColumns != 4 {
		t.Fatalf("saved columns = %d, %v", cfg.ComparisonColumns, err)
	}
	for range 3 {
		m, _ = press(t, m, "-")
	}
	if len(m.comparisonTranslations) != 2 {
		t.Fatalf("- went below 2: %v", m.comparisonTranslations)
	}
	if again := NewModel(); len(again.comparisonTranslations) != 2 {
		t.Fatalf("reopened with %d columns, want the saved 2", len(again.comparisonTranslations))
	}
}

func TestComparisonRowsFitTheReadingPane(t *testing.T) {
	m := pickerModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = next.(Model)
	m.mode = modeComparison
	m.comparisonTranslations = []string{"BSB", "KJV", "WEB"}
	text := "My lips pour forth praise, for You teach me Your statutes."
	verses := []api.Verse{{Verse: 1, Text: text}, {Verse: 2, Text: text}}
	next, _ = m.Update(parallelVersesLoadedMsg{map[string][]api.Verse{"BSB": verses, "KJV": verses, "WEB": verses}})
	m = next.(Model)
	view := ansi.Strip(m.View().Content)
	// Every row of the comparison must stay one screen line. When the
	// pane wrapped a row, the separator split and verse lines started
	// at the pane's left edge instead of under a verse number.
	separators, starts := 0, 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "─ ─") == 2 { // three column rules on one line
			separators++
		}
		if strings.Contains(line, "││  1   My lips") || strings.Contains(line, "││  2   My lips") {
			starts++
		}
		if strings.Contains(line, "││  for") {
			t.Fatalf("a comparison row wrapped inside the pane:\n%s", view)
		}
	}
	if separators != 1 || starts != 2 {
		t.Fatalf("separator lines %d, verse rows %d; want 1 and 2:\n%s", separators, starts, view)
	}
}

func TestPlusFetchesOneColumnAndMinusFetchesNothing(t *testing.T) {
	m := inComparison(t) // BSB and KJV shown; WEB already loaded too
	m, cmd := press(t, m, "+")
	if cmd != nil || !strings.Contains(m.content, "WEB") {
		t.Fatalf("WEB was loaded already, so + should draw it without fetching (cmd %v)", cmd != nil)
	}
	m, cmd = press(t, m, "+")
	if cmd == nil {
		t.Fatal("+ did not fetch the ASV column")
	}
	// The fetch for another chapter is dropped; the right one is drawn.
	next, _ := m.Update(comparisonColumnMsg{book: 2, chapter: 1, translation: "ASV", verses: []api.Verse{{Verse: 1, Text: "wrong chapter"}}})
	m = next.(Model)
	next, _ = m.Update(comparisonColumnMsg{book: m.currentBook, chapter: m.currentChapter, translation: "ASV", verses: []api.Verse{{Verse: 1, Text: "In the beginning ASV"}}})
	m = next.(Model)
	if strings.Contains(m.content, "wrong chapter") || !strings.Contains(m.content, "ASV") {
		t.Fatalf("ASV column not drawn correctly:\n%s", ansi.Strip(m.content))
	}
	m, cmd = press(t, m, "-")
	if cmd != nil || strings.Contains(m.content, "ASV") {
		t.Fatalf("- should drop ASV without fetching (cmd %v)", cmd != nil)
	}
	if _, cmd = press(t, m, "+"); cmd != nil {
		t.Fatal("ASV was fetched again for the same chapter")
	}
}
