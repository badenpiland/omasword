package ui

import (
	"errors"
	"fmt"
	"strings"

	"omasword/internal/esv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Offline ESV: S saves the selected verses (or unsaves them), o lists
// what is saved, and an ESV chapter that can't be loaded shows your
// saved verses from it, or the BSB when none are saved. All ESV storage
// goes through esv.VerseCache, which holds Crossway's 500-verse and
// half-book caps; nothing here stores ESV text itself.

// showingESV is true when the reading pane holds ESV text, so the
// Crossway notice applies. It is false while the BSB stands in.
func (m Model) showingESV() bool {
	return m.selectedTranslation == esv.ID && m.fallback == ""
}

// readerTranslation names the translation on screen, which is the BSB
// rather than the selected ESV while the fallback is shown.
func (m Model) readerTranslation() string {
	if m.fallback != "" {
		return m.fallback
	}
	return m.selectedTranslation
}

// degraded is true while the ESV couldn't be loaded and the pane shows a
// stand-in: the BSB, or only your saved verses.
func (m Model) degraded() bool {
	return m.fallbackErr != nil
}

// fallbackStatus is the short status-bar text while degraded.
func (m Model) fallbackStatus() string {
	what := "ESV offline"
	var rate *esv.RateError
	if errors.As(m.fallbackErr, &rate) {
		what = "ESV rate limit"
	}
	if m.fallback != "" {
		return what + " · " + m.fallback + " shown"
	}
	return what + " · saved verses only"
}

// fallbackNote is the line under a stand-in chapter.
func (m Model) fallbackNote() string {
	if m.fallback != "" {
		return fmt.Sprintf("The ESV can't be reached right now and none of this chapter is saved, so this is the %s. The ESV comes back on its own once it loads. To read ESV offline, select verses and press S while online.", m.fallback)
	}
	return "The ESV can't be reached right now. These are the verses you saved from this chapter; the rest comes back once the ESV loads."
}

// refreshSaved reloads which verses of the chapter on screen are saved.
func (m *Model) refreshSaved() {
	m.savedHere = nil
	if m.esvClient == nil || m.selectedTranslation != esv.ID {
		return
	}
	for _, v := range m.esvClient.SavedVerses(m.currentBook, m.currentChapter) {
		if m.savedHere == nil {
			m.savedHere = map[int]bool{}
		}
		m.savedHere[v.Verse] = true
	}
}

// selection returns the verses on screen inside the highlighted range.
func (m Model) selection() []int {
	var out []int
	for _, v := range m.currentVerses {
		if v.Verse >= m.highlightedVerseStart && v.Verse <= m.highlightedVerseEnd {
			out = append(out, v.Verse)
		}
	}
	return out
}

// extendSelection grows or shrinks the highlighted range by one verse,
// keeping the verse where the selection started fixed.
func (m Model) extendSelection(delta int) Model {
	if m.currentVerses == nil {
		return m
	}
	if m.highlightedVerseStart == 0 {
		return m.selectTopVerse()
	}
	start, end := m.highlightedVerseStart, m.highlightedVerseEnd
	anchor := m.dragAnchorVerse
	if anchor != start && anchor != end {
		anchor = start
		if delta < 0 {
			anchor = end
		}
	}
	cursor := end
	if anchor == end {
		cursor = start
	}
	idx := -1
	for i, v := range m.currentVerses {
		if v.Verse == cursor {
			idx = i
			break
		}
	}
	idx += delta
	if idx < 0 || idx >= len(m.currentVerses) {
		return m
	}
	cursor = m.currentVerses[idx].Verse
	m.dragAnchorVerse = anchor
	m.highlightedVerseStart, m.highlightedVerseEnd = min(anchor, cursor), max(anchor, cursor)
	m.rerenderContent()
	m.scrollToHighlightedVerse()
	return m
}

// toggleSave saves the selected ESV verses, or unsaves them when every
// one of them is saved already.
func (m Model) toggleSave() Model {
	switch {
	case m.selectedTranslation != esv.ID:
		m.status = "Saving is for the ESV; download other translations with d"
		return m
	case m.fallback != "":
		m.status = "Can't save: the ESV isn't loaded"
		return m
	case m.esvClient == nil:
		return m
	}
	sel := m.selection()
	if len(sel) == 0 {
		m.status = "Select verses to save"
		return m
	}
	ref := m.selectionRef()
	all := true
	for _, v := range sel {
		all = all && m.savedHere[v]
	}
	if all {
		if err := m.esvClient.UnsaveVerses(m.currentBook, m.currentChapter, sel[0], sel[len(sel)-1]); err != nil {
			m.status = "Couldn't unsave: " + err.Error()
			return m
		}
		m.status = fmt.Sprintf("Unsaved %s · %d/%d verses saved", ref, m.esvClient.SavedCount(), esv.MaxStoredVerses)
	} else {
		var verses = m.currentVerses[:0:0]
		for _, v := range m.currentVerses {
			if v.Verse >= sel[0] && v.Verse <= sel[len(sel)-1] {
				verses = append(verses, v)
			}
		}
		if err := m.esvClient.SaveVerses(m.currentBook, m.currentChapter, verses); err != nil {
			m.status = err.Error()
			return m
		}
		m.status = fmt.Sprintf("Saved %s · %d/%d verses saved · o lists them", ref, m.esvClient.SavedCount(), esv.MaxStoredVerses)
	}
	m.refreshSaved()
	m.rerenderContent()
	return m
}

func (m Model) selectionRef() string {
	if m.highlightedVerseStart == m.highlightedVerseEnd {
		return fmt.Sprintf("%s %d:%d", m.currentBookName, m.currentChapter, m.highlightedVerseStart)
	}
	return fmt.Sprintf("%s %d:%d–%d", m.currentBookName, m.currentChapter, m.highlightedVerseStart, m.highlightedVerseEnd)
}

// verseRanges writes sorted verse numbers as "1–3, 7".
func verseRanges(vs []int) string {
	var parts []string
	for i := 0; i < len(vs); {
		j := i
		for j+1 < len(vs) && vs[j+1] == vs[j]+1 {
			j++
		}
		if i == j {
			parts = append(parts, fmt.Sprint(vs[i]))
		} else {
			parts = append(parts, fmt.Sprintf("%d–%d", vs[i], vs[j]))
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
}

// openSavedList shows the saved verses.
func (m Model) openSavedList() Model {
	if m.esvClient == nil {
		m.status = "Saved verses are for the ESV (t, then a for a key)"
		return m
	}
	m.savedList = m.esvClient.SavedList()
	m.savedSelected = min(m.savedSelected, max(len(m.savedList)-1, 0))
	m.mode = modeSaved
	return m
}

// removeSavedSelected removes the highlighted chapter from the list.
func (m Model) removeSavedSelected() Model {
	if m.savedSelected >= len(m.savedList) {
		return m
	}
	sc := m.savedList[m.savedSelected]
	if err := m.esvClient.UnsaveVerses(sc.Book, sc.Chapter, 0, 0); err != nil {
		m.status = "Couldn't remove: " + err.Error()
		return m
	}
	m.savedList = m.esvClient.SavedList()
	m.savedSelected = min(m.savedSelected, max(len(m.savedList)-1, 0))
	if sc.Book == m.currentBook && sc.Chapter == m.currentChapter {
		m.refreshSaved()
		m.rerenderContent()
	}
	return m
}

// openSavedSelected goes to the highlighted chapter with its first saved
// range selected.
func (m Model) openSavedSelected() (Model, tea.Cmd) {
	if m.savedSelected >= len(m.savedList) {
		return m, nil
	}
	sc := m.savedList[m.savedSelected]
	start, end := sc.Verses[0], sc.Verses[0]
	for end+1 <= sc.Verses[len(sc.Verses)-1] && containsInt(sc.Verses, end+1) {
		end++
	}
	m.currentBook, m.currentChapter = sc.Book, sc.Chapter
	if name, ok := esv.BookName(sc.Book); ok {
		m.currentBookName = name
	}
	for _, b := range m.books {
		if b.BookID == sc.Book {
			m.currentBookName = b.Name
			break
		}
	}
	m.highlightedVerseStart, m.highlightedVerseEnd = start, end
	m.dragAnchorVerse = start
	m.mode = modeReader
	m.loading = true
	m = m.withESVAnchor(sc.Book, sc.Chapter, start)
	return m, m.loadReadingChapter(sc.Book, sc.Chapter)
}

func containsInt(vs []int, n int) bool {
	for _, v := range vs {
		if v == n {
			return true
		}
	}
	return false
}

// renderSaved is the "o" panel: each chapter with saved verses.
func (m Model) renderSaved() string {
	bg := m.currentTheme.Background
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(56).
		Padding(1, 2)
	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)
	selectedStyle := lipgloss.NewStyle().Foreground(bg).Background(m.currentTheme.Accent).Bold(true).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg).Padding(0, 1)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Saved ESV verses") + "\n")
	total := 0
	for _, sc := range m.savedList {
		total += len(sc.Verses)
	}
	content.WriteString(mutedStyle.Render(fmt.Sprintf("%d of %d verses · readable offline", total, esv.MaxStoredVerses)) + "\n\n")
	if len(m.savedList) == 0 {
		content.WriteString(mutedStyle.Render("Nothing saved yet. Select verses in the ESV (click, drag, or J/K) and press S.") + "\n\n")
		content.WriteString(m.panelKeys([][2]string{{"esc", "close"}}))
		return containerStyle.Render(content.String())
	}
	const window = 14
	start, end := 0, len(m.savedList)
	if end > window {
		start = max(min(m.savedSelected-window/2, end-window), 0)
		end = start + window
	}
	if start > 0 {
		content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		sc := m.savedList[i]
		name, _ := esv.BookName(sc.Book)
		line := fmt.Sprintf("%s %d:%s", name, sc.Chapter, verseRanges(sc.Verses))
		line = ansi.Truncate(line, 46, "…")
		if i == m.savedSelected {
			content.WriteString(selectedStyle.Render("▸ "+line) + "\n")
		} else {
			content.WriteString(normalStyle.Render("  "+line) + "\n")
		}
	}
	if end < len(m.savedList) {
		content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.savedList)-end)) + "\n")
	}
	content.WriteString("\n" + m.panelKeys([][2]string{{"↑↓", "move"}, {"⏎", "open"}, {"x", "remove"}, {"esc", "close"}}))
	return containerStyle.Render(content.String())
}

// panelKeys is a line of key hints inside a panel: "⏎ open · x remove".
func (m Model) panelKeys(keys [][2]string) string {
	bg := m.currentTheme.Background
	key := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	dim := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg)
	var parts []string
	for _, k := range keys {
		parts = append(parts, key.Render(k[0])+dim.Render(" "+k[1]))
	}
	return strings.Join(parts, dim.Render("  ·  "))
}

// clearSelection drops the verse selection (esc). Copy then takes the
// whole chapter, and scrolling doesn't select a verse again.
func (m Model) clearSelection() Model {
	m.highlightedVerseStart, m.highlightedVerseEnd, m.dragAnchorVerse = 0, 0, 0
	m.rerenderContent()
	return m
}

// selectTopVerse selects the verse at the top of the reading pane, where
// j/k/J/K start when nothing is selected.
func (m Model) selectTopVerse() Model {
	if len(m.currentVerses) == 0 {
		return m
	}
	v := m.calculateHighlightedVerse()
	m.highlightedVerseStart, m.highlightedVerseEnd, m.dragAnchorVerse = v, v, v
	m.rerenderContent()
	return m
}
