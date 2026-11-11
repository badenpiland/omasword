package ui

import (
	"strings"
	"testing"

	"omasword/internal/api"
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
	for _, key := range []string{"esc", "r"} {
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
