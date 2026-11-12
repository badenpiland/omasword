package ui

import (
	"fmt"
	"omasword/internal/esv"
	"omasword/internal/version"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() tea.View {
	return tea.View{
		Content:   m.renderView(),
		AltScreen: true,
		// AllMotion gives us motion events even when no button is held,
		// which is what we need for hover highlights.
		MouseMode: tea.MouseModeAllMotion,
	}
}

// viewportSize returns the inner content width/height of the right pane.
// The right pane outer width is m.width - leftPaneOuterWidth.
// We subtract: 2 for the rounded border and 4 for padding(1, 2) so the
// viewport fills the pane's inner content area exactly (no unstyled gap
// at the right edge).
func (m Model) viewportSize() (int, int) {
	// The reading pane box is 2 narrower than its slot, then loses its
	// border (2) and padding (4). Content wider than that is wrapped
	// again by the box, which split comparison rows mid-line.
	w := m.width - leftPaneOuterWidth - 2 - 2 - 4
	if w < 20 {
		w = 20
	}
	h := m.height - headerOuterHeight - statusOuterHeight - 2 - 2 - 2
	if h < 5 {
		h = 5
	}
	return w, h
}

// overlayActive reports whether the current mode draws a floating panel
// on top of the two-pane shell.
func (m Model) overlayActive() bool {
	switch m.mode {
	case modeSearch, modeTranslationSelect,
		modeCacheManager, modeAbout, modeWordSearch, modePicker, modeESVKey, modeSaved:
		return true
	}
	return false
}

func (m Model) renderView() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	if m.width < 60 || m.height < 18 {
		fitStyle := lipgloss.NewStyle().
			Foreground(m.currentTheme.Warning).
			Bold(true)
		return "\n  " + fitStyle.Render("Terminal too small — resize to at least 60×18.")
	}

	header := m.renderHeader()
	body := m.renderBody()
	status := m.renderStatusBar()

	base := lipgloss.JoinVertical(lipgloss.Left, header, body, status)

	if !m.overlayActive() {
		return base
	}

	return m.composeOverlay(base)
}

func (m Model) renderHeader() string {
	width := m.width

	bg := m.currentTheme.Background
	accent := m.currentTheme.Accent
	successCol := m.currentTheme.Success

	// Nerd Font "fa-book_bible": the terminal-sized version of the
	// omasword icon (assets/omasword.svg). Omarchy ships JetBrainsMono
	// Nerd Font, so it renders everywhere omasword targets.
	logo := "\ueebe"
	logoStyle := lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true)
	breadcrumbStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	separatorStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg)
	versionStyle := lipgloss.NewStyle().Foreground(successCol).Background(bg).Bold(true)

	bookName := m.currentBookName
	if bookName == "" {
		bookName = "—"
	}
	chapter := fmt.Sprintf("%d", m.currentChapter)
	if m.currentChapter == 0 {
		chapter = "—"
	}

	breadcrumb := logoStyle.Render(logo+" omasword") +
		separatorStyle.Render("  ·  ") +
		breadcrumbStyle.Render(m.readerTranslation()) +
		separatorStyle.Render(" › ") +
		breadcrumbStyle.Render(bookName) +
		separatorStyle.Render(" › ") +
		breadcrumbStyle.Render(chapter)

	versionStr := versionStyle.Render(version.Version)

	innerWidth := width - 4 - 2 // -2 border -2 padding -2 safety
	rightW := lipgloss.Width(versionStr)
	leftW := innerWidth - rightW - 1
	if leftW < 1 {
		leftW = 1
	}
	leftSlot := lipgloss.NewStyle().
		Background(bg).
		Width(leftW).
		MaxWidth(leftW).
		Render(breadcrumb)
	gap := lipgloss.NewStyle().Background(bg).Render(" ")
	content := leftSlot + gap + versionStr

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width-2).
		Padding(0, 1)

	return box.Render(content)
}

func (m Model) renderStatusBar() string {
	width := m.width
	bg := m.currentTheme.Background

	hintStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg)
	keyStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	rightStyle := lipgloss.NewStyle().Background(bg)

	hints := m.statusHints(keyStyle, hintStyle)

	// Right side: loading indicator or error condensed
	var right string
	if m.loading {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Background(bg).Bold(true).Render("● loading")
	} else if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Error).Background(bg).Bold(true)
		msg := m.err.Error()
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}
		right = errStyle.Render("⚠ " + msg)
	} else if m.status != "" {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Render(ansi.Truncate("● "+m.status, max(m.width/2, 20), "…"))
	} else if m.degraded() {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Background(bg).Render("● " + m.fallbackStatus())
	} else if m.showingESV() && len(m.savedHere) > 0 {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Success).Background(bg).Render(fmt.Sprintf("● %d saved here", len(m.savedHere)))
	} else if m.updateNotice != "" {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Render(ansi.Truncate("● "+m.updateNotice+" available: omasword update", max(m.width/2, 20), "…"))
	} else if m.cache != nil && m.cache.IsCached(m.selectedTranslation) {
		right = lipgloss.NewStyle().Foreground(m.currentTheme.Success).Background(bg).Render("● offline")
	} else {
		right = hintStyle.Render("● online")
	}

	innerWidth := width - 4 - 2 // -2 border -2 padding -2 safety
	rightW := lipgloss.Width(right)
	leftW := innerWidth - rightW - 1
	if leftW < 1 {
		leftW = 1
	}
	// Cut the hints rather than wrap them: a second line would push the
	// bottom border off the screen.
	hintsSlot := rightStyle.Width(leftW).Render(ansi.Truncate(hints, leftW, "…"))
	gap := lipgloss.NewStyle().Background(bg).Render(" ")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.Border).
		BorderBackground(bg).
		Background(bg).
		Width(width-2).
		Padding(0, 1)

	return box.Render(hintsSlot + gap + right)
}

func (m Model) statusHints(key, dim lipgloss.Style) string {
	type hint struct{ k, label string }

	var hs []hint
	switch m.mode {
	case modeTranslationSelect:
		hs = []hint{{"↑↓", "navigate"}, {"⏎", "select"}, {"a", "ESV key"}, {"esc", "close"}}
	case modeESVKey:
		hs = []hint{{"⏎", "save key"}, {"esc", "cancel"}}
	case modeCacheManager:
		hs = []hint{{"↑↓", "navigate"}, {"⏎", "download"}, {"x", "delete"}, {"esc", "close"}}
	case modeAbout:
		hs = []hint{{"esc", "close"}}
	case modeSaved:
		hs = []hint{{"↑↓", "navigate"}, {"⏎", "open"}, {"x", "remove"}, {"esc", "close"}}
	case modeWordSearch:
		if m.wordSearchResults != nil {
			hs = []hint{{"↑↓", "navigate"}, {"⏎", "go to verse"}, {"esc", "close"}}
		} else {
			hs = []hint{{"⏎", "search"}, {"esc", "close"}}
		}
	case modeComparison:
		hs = []hint{{"↑↓", "scroll"}, {"+/-", "columns"}, {"c", "close"}}
	case modeSearch:
		hs = []hint{{"⏎", "go"}, {"esc", "cancel"}}
	case modePicker:
		if m.pickerFiltering {
			hs = []hint{{"⏎", "keep filter"}, {"esc", "clear"}}
		} else {
			hs = []hint{{"↑↓", "move"}, {"←→", "column"}, {"/", "filter"}, {"⏎", "open"}, {"esc", "close"}}
		}
	default:
		hs = []hint{
			{"tab", "focus"},
			{"⏎", "open"},
			{"n/p", "chapter"},
			{"t", "translation"},
			{"/", "verse"},
			{"s", "search"},
			{"?", "about"},
			{"q", "quit"},
		}
		if m.selectedTranslation == esv.ID {
			hs = append(hs[:6], append([]hint{{"S", "save"}, {"o", "saved"}}, hs[6:]...)...)
		}
	}

	// Each part bundles the key + leading space + label into a single
	// dim.Render() so the gap between the styled key and label gets the
	// pane background (raw spaces between two styled spans would inherit
	// the terminal default and show through as black blocks).
	var parts []string
	for _, h := range hs {
		parts = append(parts, key.Render(h.k)+dim.Render(" "+h.label))
	}
	return strings.Join(parts, dim.Render("  ·  "))
}

func (m Model) renderBody() string {
	bodyHeight := m.height - headerOuterHeight - statusOuterHeight
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	leftW := leftPaneOuterWidth
	rightW := m.width - leftW

	left := m.renderLeftPane(leftW, bodyHeight)
	right := m.renderRightPane(rightW, bodyHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderLeftPane(outerW, outerH int) string {
	active := m.focus == paneBooks && !m.overlayActive()
	border := m.currentTheme.Border
	if active {
		border = m.currentTheme.BorderActive
	}
	bg := m.currentTheme.Background

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Success).Background(bg).Bold(true)
	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Background).
		Background(m.currentTheme.Accent).
		Bold(true)
	currentStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning).Background(bg).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	hoverStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(m.currentTheme.Highlight)
	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	// Resolve hover only when the mouse is actually inside this pane.
	hoveredBookIdx := -1
	if !m.overlayActive() && m.mouseX >= 0 && m.mouseX < leftPaneOuterWidth {
		if hi, ok := m.bookAtRow(m.mouseY); ok {
			hoveredBookIdx = hi
		}
	}

	innerW := outerW - 4 // 2 border + 2 padding
	innerH := outerH - 4 // 2 border + 2 padding

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Books"))
	sb.WriteString("\n")

	// Reserve: 1 for the "Books" title above, plus 2 (one per direction)
	// for the "↑ N more" / "↓ N more" indicators when virtual scrolling.
	contentLines := innerH - 1 - 2
	if contentLines < 1 {
		contentLines = 1
	}

	if m.books == nil && m.booksErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Error).Background(bg)
		sb.WriteString(errStyle.Render("Couldn't load books") + "\n" + mutedStyle.Render("retrying…"))
	} else if m.books == nil {
		sb.WriteString(mutedStyle.Render("Loading…"))
	} else {
		type entry struct {
			isHeader bool
			label    string
			bookIdx  int
			isCur    bool
			isSel    bool
		}
		var entries []entry
		entries = append(entries, entry{isHeader: true, label: "OLD TESTAMENT"})
		for i, b := range m.books {
			if b.BookID > 39 {
				continue
			}
			label := b.Name
			if lipgloss.Width(label) > innerW-2 {
				label = label[:innerW-2]
			}
			entries = append(entries, entry{
				label:   label,
				bookIdx: i,
				isCur:   b.BookID == m.currentBook,
				isSel:   i == m.sidebarSelected,
			})
		}
		entries = append(entries, entry{isHeader: true, label: "NEW TESTAMENT"})
		for i, b := range m.books {
			if b.BookID < 40 {
				continue
			}
			label := b.Name
			if lipgloss.Width(label) > innerW-2 {
				label = label[:innerW-2]
			}
			entries = append(entries, entry{
				label:   label,
				bookIdx: i,
				isCur:   b.BookID == m.currentBook,
				isSel:   i == m.sidebarSelected,
			})
		}

		// Virtual scrolling: center on selected index
		selIdx := -1
		for i, e := range entries {
			if e.isSel {
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

		if start > 0 {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("↑ %d more", start)) + "\n")
		}
		for i := start; i < end; i++ {
			e := entries[i]
			if e.isHeader {
				sb.WriteString(sectionStyle.Render(e.label) + "\n")
				continue
			}
			line := "  " + e.label
			isHovered := e.bookIdx == hoveredBookIdx && !e.isSel
			if e.isSel {
				line = "▸ " + e.label
				// pad to innerW to fill the highlight bar
				if lipgloss.Width(line) < innerW {
					line = line + strings.Repeat(" ", innerW-lipgloss.Width(line))
				}
				sb.WriteString(selectedStyle.Render(line) + "\n")
				continue
			}
			if isHovered {
				// Pad to fill the row so the hover bg covers the entire
				// pane width, not just the text.
				if lipgloss.Width(line) < innerW {
					line = line + strings.Repeat(" ", innerW-lipgloss.Width(line))
				}
				sb.WriteString(hoverStyle.Render(line) + "\n")
				continue
			}
			if e.isCur {
				sb.WriteString(currentStyle.Render(line) + "\n")
				continue
			}
			sb.WriteString(normalStyle.Render(line) + "\n")
		}
		if end < len(entries) {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("↓ %d more", len(entries)-end)))
		}
	}

	// Pad content to exactly fill innerH so the box sizes consistently with
	// the right pane.
	written := sb.String()
	lines := strings.Count(written, "\n") + 1
	for lines < innerH {
		sb.WriteString("\n")
		lines++
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(outerW-2).
		Padding(1, 1)

	return box.Render(sb.String())
}

func (m Model) renderRightPane(outerW, outerH int) string {
	active := m.focus == paneContent && !m.overlayActive()
	border := m.currentTheme.Border
	if active {
		border = m.currentTheme.BorderActive
	}
	bg := m.currentTheme.Background

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	var titleText string
	switch m.mode {
	case modeComparison:
		titleText = fmt.Sprintf("Comparison · %s %d", m.currentBookName, m.currentChapter)
	default:
		if m.currentBookName == "" {
			titleText = "Reader"
		} else {
			titleText = fmt.Sprintf("%s %d", m.currentBookName, m.currentChapter)
		}
	}

	title := titleStyle.Render(titleText)

	// Single sticky header: the title row itself surfaces the current
	// reading position. When the viewport is scrolled past the top,
	// append "↑ v. N" so the reader always sees where they are. When
	// the user has explicitly selected a verse range but isn't scrolled
	// (j/k inside the visible area), surface the verse range. The two
	// states never duplicate.
	scrolled := m.ready && m.viewport.YOffset() > 0 && m.mode == modeReader
	var locator string
	switch {
	case scrolled && m.highlightedVerseStart > 0:
		locator = mutedStyle.Render(fmt.Sprintf("  ↑ v. %d", m.highlightedVerseStart))
	case scrolled && m.topVisibleVerse > 0:
		locator = mutedStyle.Render(fmt.Sprintf("  ↑ v. %d", m.topVisibleVerse))
	case !scrolled && m.highlightedVerseStart > 0 && m.highlightedVerseStart != m.highlightedVerseEnd:
		locator = mutedStyle.Render(fmt.Sprintf("  v. %d–%d", m.highlightedVerseStart, m.highlightedVerseEnd))
	case !scrolled && m.highlightedVerseStart > 1:
		// Show the verse only when it's not the obvious "verse 1 at top".
		locator = mutedStyle.Render(fmt.Sprintf("  v. %d", m.highlightedVerseStart))
	}

	// Hover indicator: when the mouse cursor is over a verse in the
	// viewport, surface that verse number with a distinct ⊙ marker.
	// Suppressed when the hovered verse happens to be the same one the
	// locator above is already showing — no point saying it twice.
	if hoveredVerse := m.verseAtMouseY(m.mouseY); hoveredVerse > 0 && hoveredVerse != m.highlightedVerseStart {
		hoverStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
		locator += hoverStyle.Render(fmt.Sprintf("  ⊙ v. %d", hoveredVerse))
	}

	header := title + locator

	body := m.viewport.View()
	if m.mode == modeReader && m.currentVerses == nil && m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Error).Background(bg)
		muted := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg)
		body = errStyle.Render("Couldn't load this chapter") + "\n" + muted.Render("retrying…")
	}

	innerW := outerW - 2 - 2 - 4 // slot margin + border + padding(1,2)
	spacer := lipgloss.NewStyle().Background(bg).Width(innerW).Render("")

	content := header + "\n" + spacer + "\n" + body

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(outerW-2).
		Height(outerH-2).
		Padding(1, 2)

	return box.Render(content)
}

func (m Model) composeOverlay(base string) string {
	panel := m.renderOverlayPanel()
	if panel == "" {
		return base
	}
	panelW := lipgloss.Width(panel)
	panelH := lipgloss.Height(panel)
	x, y := m.overlayOrigin(panelW, panelH)

	shadowStyle := lipgloss.NewStyle().
		Background(m.currentTheme.Shadow).
		Foreground(m.currentTheme.Shadow)
	shadowLine := shadowStyle.Render(strings.Repeat(" ", panelW))
	shadowLines := make([]string, panelH)
	for i := range shadowLines {
		shadowLines[i] = shadowLine
	}
	shadow := strings.Join(shadowLines, "\n")

	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(base).X(0).Y(0).Z(0),
		lipgloss.NewLayer(shadow).X(x+2).Y(y+1).Z(1),
		lipgloss.NewLayer(panel).X(x).Y(y).Z(2),
	)
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(compositor)
	return canvas.Render()
}

// overlayOrigin places a panel centered over the right pane, or over the
// whole window when it is too wide for the right pane.
func (m Model) overlayOrigin(panelW, panelH int) (x, y int) {
	rightX := leftPaneOuterWidth
	x = rightX + (m.width-rightX-panelW)/2
	if x+panelW > m.width {
		x = (m.width - panelW) / 2
	}
	y = (m.height-panelH)/2 + 1
	return max(x, 1), max(y, 1)
}

func (m Model) renderOverlayPanel() string {
	switch m.mode {
	case modeSearch:
		return m.renderSearchPanel()
	case modeTranslationSelect:
		return m.renderTranslationSelect()
	case modeCacheManager:
		return m.renderCacheManager()
	case modeAbout:
		return m.renderAbout()
	case modeWordSearch:
		return m.renderWordSearch()
	case modePicker:
		return m.renderPicker()
	case modeESVKey:
		return m.renderESVKey()
	case modeSaved:
		return m.renderSaved()
	}
	return ""
}
