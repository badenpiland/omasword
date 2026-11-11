package ui

import (
	"strings"

	"omasword/internal/translations"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// comparisonColumnAtX returns the 0-based column index in
// m.comparisonTranslations whose header sits under screen X x, or -1
// if x is outside any column header. Only meaningful in modeComparison.
func (m Model) comparisonColumnAtX(x int) int {
	if m.mode != modeComparison || len(m.comparisonTranslations) == 0 {
		return -1
	}
	// Right pane content area starts at: left pane (30) + right pane
	// left border (1) + left padding (2) = 33.
	contentX := leftPaneOuterWidth + 1 + 2
	n := len(m.comparisonTranslations)
	gaps := n - 1
	colWidth := (m.viewport.Width() - gaps) / n
	if colWidth < 20 {
		colWidth = 20
	}
	relX := x - contentX
	if relX < 0 {
		return -1
	}
	stride := colWidth + 1 // colWidth + 1-cell gutter
	for j := 0; j < n; j++ {
		start := j * stride
		if relX >= start && relX < start+colWidth {
			return j
		}
	}
	return -1
}

// verseAtMouseY returns the verse number the mouse cursor is currently
// over inside the right pane viewport, or 0 if the cursor is somewhere
// else (left pane, chrome, overlay, header/status bar). It mirrors the
// line-counting logic in formatChapter so its mapping is consistent
// with what's actually drawn.
func (m Model) verseAtMouseY(y int) int {
	if len(m.currentVerses) == 0 {
		return 0
	}
	if m.mouseX < leftPaneOuterWidth || m.overlayActive() {
		return 0
	}
	// Viewport content starts at: app header (3) + right-pane border (1)
	// + top padding (1) + title row (1) + spacer row (1) = 7.
	viewportTopY := headerOuterHeight + 4
	bottomY := viewportTopY + m.viewport.Height()
	if y < viewportTopY || y >= bottomY {
		return 0
	}
	line := y - viewportTopY + m.viewport.YOffset()
	if line < 0 {
		return 0
	}

	textWidth := readerTextWidth(m.viewport.Width())
	indent := 6
	currentLine := 0

	for i, v := range m.currentVerses {
		text := stripHTMLTags(v.Text)
		isHighlighted := m.highlightedVerseStart > 0 && v.Verse >= m.highlightedVerseStart && v.Verse <= m.highlightedVerseEnd
		nextIsHighlighted := false
		if i+1 < len(m.currentVerses) {
			nv := m.currentVerses[i+1]
			nextIsHighlighted = m.highlightedVerseStart > 0 && nv.Verse >= m.highlightedVerseStart && nv.Verse <= m.highlightedVerseEnd
		}

		var verseLines int
		if isHighlighted {
			wrapped := wrapTextWithIndent(text, textWidth-4, indent)
			ln := strings.Count(wrapped, "\n") + 1
			if !nextIsHighlighted {
				// end of range: wrapped content + top + bottom borders + 1 trailing blank
				verseLines = ln + 2 + 1
			} else {
				// in-range verse contributes its lines plus a 1-line spacer
				verseLines = ln + 1
			}
		} else {
			wrapped := wrapTextWithIndent(text, textWidth, indent)
			ln := strings.Count(wrapped, "\n") + 1
			verseLines = ln + 1
		}

		if currentLine+verseLines > line {
			return v.Verse
		}
		currentLine += verseLines
	}
	return 0
}

// overlayPanelBounds returns the (x, y, width, height) of the floating
// overlay panel as it appears on screen. Must match composeOverlay.
func (m Model) overlayPanelBounds() (int, int, int, int) {
	panel := m.renderOverlayPanel()
	if panel == "" {
		return 0, 0, 0, 0
	}
	panelW := lipgloss.Width(panel)
	panelH := lipgloss.Height(panel)
	x, y := m.overlayOrigin(panelW, panelH)
	return x, y, panelW, panelH
}

// overlayClick resolves a click at row N (0-indexed from the first
// list item) of the active overlay.
func (m *Model) overlayClick(row int) tea.Cmd {
	if row < 0 {
		return nil
	}
	switch m.mode {
	case modeTranslationSelect:
		if m.translations == nil {
			return nil
		}
		start := m.overlayWindowStart(m.translationSelected, len(m.translations), translationWindow)
		offset := 0
		if start > 0 {
			offset = 1 // the "↑ N more" indicator line
		}
		itemRow := row - offset
		if itemRow < 0 {
			return nil
		}
		// Each translation is a name line plus a flavor line.
		idx := start + itemRow/translationItemLines
		if idx < 0 || idx >= len(m.translations) {
			return nil
		}
		m.translationSelected = idx
		newTrans := m.translations[idx].ShortName
		if newTrans == m.selectedTranslation && m.comparisonPickerColumn < 0 {
			m.mode = modeReader
			return nil
		}
		next, cmd := m.chooseTranslation(newTrans)
		*m = next
		return cmd
	case modeCacheManager:
		if m.translations == nil {
			return nil
		}
		start := m.overlayWindowStart(m.cacheSelected, len(m.translations), 14)
		offset := 0
		if start > 0 {
			offset = 1
		}
		idx := start + row - offset
		if idx < 0 || idx >= len(m.translations) {
			return nil
		}
		m.cacheSelected = idx
		trans := m.translations[idx].ShortName
		if !translations.IsBuiltin(trans) {
			return nil
		}
		if m.cache != nil && !m.cache.IsCached(trans) && m.downloadingTranslation == "" {
			m.downloadingTranslation = trans
			m.downloadProgress = 0
			return tea.Batch(downloadTranslation(m.cache, trans), downloadTick())
		}
	}
	return nil
}

// overlayNudge moves the selection in the active overlay by delta (±1).
func (m *Model) overlayNudge(delta int) {
	switch m.mode {
	case modePicker:
		m.pickerMove(delta)
	case modeTranslationSelect:
		if m.translations == nil {
			return
		}
		next := m.translationSelected + delta
		if next < 0 {
			next = 0
		}
		if next > len(m.translations)-1 {
			next = len(m.translations) - 1
		}
		m.translationSelected = next
	case modeCacheManager:
		if m.translations == nil {
			return
		}
		next := m.cacheSelected + delta
		if next < 0 {
			next = 0
		}
		if next > len(m.translations)-1 {
			next = len(m.translations) - 1
		}
		m.cacheSelected = next
	case modeWordSearch:
		if m.wordSearchResults == nil {
			return
		}
		next := m.wordSearchSelected + delta
		if next < 0 {
			next = 0
		}
		if next > len(m.wordSearchResults)-1 {
			next = len(m.wordSearchResults) - 1
		}
		m.wordSearchSelected = next
	}
}

// overlayWindowStart returns the start index of the visible window for a
// list of n items with the given centered selection and window size.
// Must mirror the windowing math used by the modal renderers.
func (Model) overlayWindowStart(sel, n, window int) int {
	if n <= window {
		return 0
	}
	start := sel - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > n {
		start = n - window
	}
	return start
}

// bookAtRow returns the book index whose row matches screen y in the
// books pane, or false if y doesn't land on a book.
func (m Model) bookAtRow(y int) (int, bool) {
	if m.books == nil {
		return 0, false
	}

	// Inner content area starts at: header(3) + top border(1) + top padding(1).
	contentY := y - headerOuterHeight - 2
	if contentY < 0 {
		return 0, false
	}

	// Replay the same windowing the renderer uses.
	innerH := m.height - headerOuterHeight - statusOuterHeight - 4 // -2 border -2 padding
	contentLines := innerH - 1 - 2                                 // -title -indicators
	if contentLines < 1 {
		contentLines = 1
	}

	type entry struct {
		isHeader bool
		bookIdx  int
	}
	var entries []entry
	entries = append(entries, entry{isHeader: true})
	for i, b := range m.books {
		if b.BookID > 39 {
			continue
		}
		entries = append(entries, entry{bookIdx: i})
	}
	entries = append(entries, entry{isHeader: true})
	for i, b := range m.books {
		if b.BookID < 40 {
			continue
		}
		entries = append(entries, entry{bookIdx: i})
	}

	selIdx := -1
	for i := range entries {
		if !entries[i].isHeader && entries[i].bookIdx == m.sidebarSelected {
			selIdx = i
			break
		}
	}
	start, end := 0, len(entries)
	if len(entries) > contentLines {
		if selIdx < 0 {
			selIdx = 0
		}
		half := contentLines / 2
		start = selIdx - half
		if start < 0 {
			start = 0
		}
		end = start + contentLines
		if end > len(entries) {
			end = len(entries)
			start = end - contentLines
			if start < 0 {
				start = 0
			}
		}
	}

	// Now walk the rendered rows starting at contentY = 0 = "Books" title.
	row := 0
	if contentY == row {
		return 0, false // "Books" title
	}
	row++
	if start > 0 {
		if contentY == row {
			return 0, false // "↑ N more" indicator
		}
		row++
	}
	for i := start; i < end; i++ {
		if contentY == row {
			if entries[i].isHeader {
				return 0, false
			}
			return entries[i].bookIdx, true
		}
		row++
	}
	return 0, false
}
