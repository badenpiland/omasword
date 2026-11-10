package ui

import (
	"fmt"
	"omasword/internal/api"
	"omasword/internal/version"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) renderSearchPanel() string {
	bg := m.currentTheme.Background
	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	// Size from the available right-pane area.
	maxAvail := m.width - leftPaneOuterWidth - 8
	width := maxAvail
	if width > 64 {
		width = 64
	}
	if width < 40 {
		width = 40
	}
	innerW := width - 6

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	// Apply the theme's background to the bubbles textinput so it doesn't
	// punch a terminal-default-colored hole through the panel.
	ti := m.textInput
	ti.SetStyles(m.themedInputStyles())
	ti.SetWidth(innerW - 2)

	hint := hintStyle.Render("e.g. \"John 3:16\" or \"1 1:1\"")
	if m.searchErr != nil {
		hint = lipgloss.NewStyle().Foreground(m.currentTheme.Error).Background(bg).Render("⚠ " + m.searchErr.Error())
	}
	body := titleStyle.Render("Go to verse") + "\n\n" +
		ti.View() + "\n\n" +
		hint

	return box.Render(body)
}

// themedInputStyles returns textinput Styles that paint every cell of the
// rendered input (prompt, text, placeholder, cursor backdrop) with the
// current theme's background so the input blends into its panel.
func (m Model) themedInputStyles() textinput.Styles {
	bg := m.currentTheme.Background
	primary := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	muted := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)
	prompt := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)

	state := textinput.StyleState{
		Text:        primary,
		Placeholder: muted,
		Suggestion:  muted,
		Prompt:      prompt,
	}
	return textinput.Styles{
		Focused: state,
		Blurred: state,
		Cursor: textinput.CursorStyle{
			Color: m.currentTheme.Accent,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
}

func (m *Model) applyMillerFilter() {
	if m.millerFilter == "" {
		// No filter, clear filtered lists
		m.millerFilteredBooks = nil
		m.millerFilteredVerses = nil
		return
	}

	filterLower := strings.ToLower(m.millerFilter)

	// Filter books based on current column
	if m.millerColumn == 0 && m.books != nil {
		m.millerFilteredBooks = []api.Book{}
		for _, book := range m.books {
			if strings.Contains(strings.ToLower(book.Name), filterLower) {
				m.millerFilteredBooks = append(m.millerFilteredBooks, book)
			}
		}
	}

	// Filter verses based on current column
	if m.millerColumn == 2 && m.currentVerses != nil {
		m.millerFilteredVerses = []api.Verse{}
		for _, verse := range m.currentVerses {
			verseText := stripHTMLTags(verse.Text)
			verseNumStr := fmt.Sprintf("%d", verse.Verse)
			if strings.Contains(strings.ToLower(verseText), filterLower) || strings.Contains(verseNumStr, m.millerFilter) {
				m.millerFilteredVerses = append(m.millerFilteredVerses, verse)
			}
		}
	}
}

func (m Model) renderMillerColumns() string {
	columnWidth := 30

	columnStyle := lipgloss.NewStyle().
		Width(columnWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.Border).
		Padding(1)

	activeColumnStyle := lipgloss.NewStyle().
		Width(columnWidth).
		Height(m.height - 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		Background(m.currentTheme.Background).
		Padding(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(m.currentTheme.Background).
		Bold(true).
		Padding(0, 1).
		Width(columnWidth - 2)

	// Column 1: Books
	var booksContent strings.Builder
	booksContent.WriteString(headerStyle.Render("BOOKS") + "\n")

	// Show filter input if in books column
	if m.millerColumn == 0 && m.millerFilterMode {
		booksContent.WriteString(m.millerFilterInput.View() + "\n")
	} else if m.millerColumn == 0 && m.millerFilter != "" {
		filterStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning)
		booksContent.WriteString(filterStyle.Render("Filter: "+m.millerFilter) + "\n\n")
	} else {
		booksContent.WriteString("\n")
	}

	// Use filtered books if filter is active, otherwise use all books
	booksToDisplay := m.books
	if m.millerColumn == 0 && m.millerFilter != "" && m.millerFilteredBooks != nil {
		booksToDisplay = m.millerFilteredBooks
	}

	if booksToDisplay != nil {
		visibleItems := m.height - 8
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := m.millerBookIdx - visibleItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(booksToDisplay) {
			endIdx = len(booksToDisplay)
			startIdx = endIdx - visibleItems
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			booksContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", startIdx)))
		}

		for i := startIdx; i < endIdx && i < len(booksToDisplay); i++ {
			book := booksToDisplay[i]
			name := book.Name
			if len(name) > 26 {
				name = name[:23] + "..."
			}

			if i == m.millerBookIdx {
				booksContent.WriteString(selectedStyle.Render("> "+name) + "\n")
			} else {
				booksContent.WriteString(normalStyle.Render("  "+name) + "\n")
			}
		}

		if endIdx < len(booksToDisplay) {
			booksContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", len(booksToDisplay)-endIdx)))
		}
	}

	var booksColumn string
	if m.millerColumn == 0 {
		booksColumn = activeColumnStyle.Render(booksContent.String())
	} else {
		booksColumn = columnStyle.Render(booksContent.String())
	}

	// Column 2: Chapters
	var chaptersContent strings.Builder
	chaptersContent.WriteString(headerStyle.Render("CHAPTERS") + "\n\n")

	if m.books != nil && m.millerBookIdx < len(m.books) {
		selectedBook := m.books[m.millerBookIdx]
		for i := 0; i < selectedBook.Chapters; i++ {
			chapterNum := fmt.Sprintf("Chapter %d", i+1)
			if i == m.millerChapterIdx {
				chaptersContent.WriteString(selectedStyle.Render("> "+chapterNum) + "\n")
			} else {
				chaptersContent.WriteString(normalStyle.Render("  "+chapterNum) + "\n")
			}
		}
	}

	var chaptersColumn string
	if m.millerColumn == 1 {
		chaptersColumn = activeColumnStyle.Render(chaptersContent.String())
	} else {
		chaptersColumn = columnStyle.Render(chaptersContent.String())
	}

	// Column 3: Verses
	var versesContent strings.Builder
	versesContent.WriteString(headerStyle.Render("VERSES") + "\n")

	// Show filter input if in verses column
	if m.millerColumn == 2 && m.millerFilterMode {
		versesContent.WriteString(m.millerFilterInput.View() + "\n")
	} else if m.millerColumn == 2 && m.millerFilter != "" {
		filterStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Warning)
		versesContent.WriteString(filterStyle.Render("Filter: "+m.millerFilter) + "\n\n")
	} else {
		versesContent.WriteString("\n")
	}

	// Use filtered verses if filter is active, otherwise use all verses
	versesToDisplay := m.currentVerses
	if m.millerColumn == 2 && m.millerFilter != "" && m.millerFilteredVerses != nil {
		versesToDisplay = m.millerFilteredVerses
	}

	if versesToDisplay != nil {
		visibleItems := m.height - 8
		if visibleItems < 5 {
			visibleItems = 5
		}

		startIdx := m.millerVerseIdx - visibleItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + visibleItems
		if endIdx > len(versesToDisplay) {
			endIdx = len(versesToDisplay)
			startIdx = endIdx - visibleItems
			if startIdx < 0 {
				startIdx = 0
			}
		}

		if startIdx > 0 {
			versesContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", startIdx)))
		}

		for i := startIdx; i < endIdx && i < len(versesToDisplay); i++ {
			verse := versesToDisplay[i]
			text := stripHTMLTags(verse.Text)
			if len(text) > 23 {
				text = text[:20] + "..."
			}
			verseLabel := fmt.Sprintf("%d. %s", verse.Verse, text)

			if i == m.millerVerseIdx {
				versesContent.WriteString(selectedStyle.Render("> "+verseLabel) + "\n")
			} else {
				versesContent.WriteString(normalStyle.Render("  "+verseLabel) + "\n")
			}
		}

		if endIdx < len(versesToDisplay) {
			versesContent.WriteString(normalStyle.Render(fmt.Sprintf("... (%d)\n", len(versesToDisplay)-endIdx)))
		}
	} else {
		versesContent.WriteString(normalStyle.Render("  Loading..."))
	}

	var versesColumn string
	if m.millerColumn == 2 {
		versesColumn = activeColumnStyle.Render(versesContent.String())
	} else {
		versesColumn = columnStyle.Render(versesContent.String())
	}

	// Join the three columns horizontally
	columnsView := lipgloss.JoinHorizontal(lipgloss.Top, booksColumn, chaptersColumn, versesColumn)

	// Add shadow effect to the right of the columns with gradient
	shadow1Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Background(lipgloss.Color("#333333"))
	shadow2Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#222222")).
		Background(lipgloss.Color("#222222"))
	shadow3Style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#111111"))

	columnsLines := strings.Split(columnsView, "\n")
	shadowLines := make([]string, len(columnsLines))
	for i := range columnsLines {
		shadowLines[i] = shadow1Style.Render("▌") + shadow2Style.Render("▌") + shadow3Style.Render("▌")
	}

	// Combine columns with shadow
	var columnsWithShadow strings.Builder
	for i := 0; i < len(columnsLines); i++ {
		columnsWithShadow.WriteString(columnsLines[i])
		if i < len(shadowLines) {
			columnsWithShadow.WriteString(shadowLines[i])
		}
		if i < len(columnsLines)-1 {
			columnsWithShadow.WriteString("\n")
		}
	}

	// Add status bar at the bottom
	statusBarStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Width(columnWidth*3+6). // 3 columns + borders
		Align(lipgloss.Center).
		Padding(0, 1)

	statusText := "Press / to filter"
	if m.millerFilterMode {
		statusText = "Filtering... (press enter or esc to exit)"
	} else if m.millerFilter != "" {
		statusText = fmt.Sprintf("Filter active: \"%s\" (press / to edit)", m.millerFilter)
	}

	statusBar := statusBarStyle.Render(statusText)

	// Join columns and status bar vertically
	return lipgloss.JoinVertical(lipgloss.Left, columnsWithShadow.String(), statusBar)
}

func (m Model) renderTranslationSelect() string {
	bg := m.currentTheme.Background

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(56).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(bg).
		Background(m.currentTheme.Accent).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg).
		Padding(0, 1)

	currentStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(bg).
		Padding(0, 1)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Select Translation") + "\n\n")

	if m.translations != nil {
		// Show at most 16 translations centered on selection.
		const window = 16
		start, end := 0, len(m.translations)
		if len(m.translations) > window {
			start = m.translationSelected - window/2
			if start < 0 {
				start = 0
			}
			end = start + window
			if end > len(m.translations) {
				end = len(m.translations)
				start = end - window
			}
		}
		mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)
		if start > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more\n", start)))
		}
		for i := start; i < end; i++ {
			trans := m.translations[i]
			prefix := "  "
			style := normalStyle
			suffix := ""
			isCurrent := trans.ShortName == m.selectedTranslation
			if i == m.translationSelected {
				prefix = "▸ "
				style = selectedStyle
			} else if isCurrent {
				style = currentStyle
			}
			name := fmt.Sprintf("%-6s · %s", trans.ShortName, trans.FullName)
			if isCurrent && i != m.translationSelected {
				suffix = "  ●"
			}
			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}
		if end < len(m.translations) {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.translations)-end)))
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderCacheManager() string {
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

	selectedStyle := lipgloss.NewStyle().
		Foreground(bg).
		Background(m.currentTheme.Accent).
		Bold(true).
		Padding(0, 1)
	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg).
		Padding(0, 1)
	cachedStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Success).
		Background(bg).
		Padding(0, 1)
	downloadingStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(bg).
		Padding(0, 1)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Download Translations") + "\n\n")

	if m.translations != nil {
		const window = 14
		start, end := 0, len(m.translations)
		if len(m.translations) > window {
			start = m.cacheSelected - window/2
			if start < 0 {
				start = 0
			}
			end = start + window
			if end > len(m.translations) {
				end = len(m.translations)
				start = end - window
			}
		}
		if start > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more\n", start)))
		}
		for i := start; i < end; i++ {
			trans := m.translations[i]
			prefix := "  "
			style := normalStyle
			suffix := ""
			isCached := m.cache != nil && m.cache.IsCached(trans.ShortName)
			isDownloading := m.downloadingTranslation == trans.ShortName
			if i == m.cacheSelected {
				prefix = "▸ "
				style = selectedStyle
			}
			name := fmt.Sprintf("%-6s · %s", trans.ShortName, trans.FullName)
			if isDownloading {
				suffix = "  ⟳ downloading"
				if i != m.cacheSelected {
					style = downloadingStyle
				}
			} else if isCached {
				suffix = "  ✓"
				if i != m.cacheSelected {
					style = cachedStyle
				}
			}
			content.WriteString(style.Render(prefix+name+suffix) + "\n")
		}
		if end < len(m.translations) {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", len(m.translations)-end)))
		}
	} else {
		content.WriteString(normalStyle.Render("  Loading translations..."))
	}

	// Live download progress bar, rendered just inside the panel below
	// the list when a download is running. The bubbles/v2/progress bar
	// is asked for a static view via ViewAs(p) so it doesn't animate
	// independently of our poll-driven m.downloadProgress.
	if m.downloadingTranslation != "" {
		bar := m.progressBar
		bar.SetWidth(48)
		content.WriteString("\n\n" + mutedStyle.Render(fmt.Sprintf("Downloading %s", m.downloadingTranslation)) + "\n")
		content.WriteString(bar.ViewAs(m.downloadProgress))
	}

	if m.cache != nil {
		if size, err := m.cache.GetCacheSize(); err == nil && size > 0 {
			content.WriteString("\n\n" + mutedStyle.Render(fmt.Sprintf("Cache: %.2f MB", float64(size)/(1024*1024))))
		}
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderAbout() string {
	bg := m.currentTheme.Background

	width := 64
	if m.width-leftPaneOuterWidth-6 < width {
		width = m.width - leftPaneOuterWidth - 6
		if width < 40 {
			width = 40
		}
	}

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	labelStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Secondary).Background(bg).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	linkStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Underline(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("\ueebe omasword") + "\n")
	content.WriteString(sectionStyle.Render("A terminal-based Bible application") + "\n\n")

	content.WriteString(labelStyle.Render("Version: ") + valueStyle.Render(version.Version) + "\n")
	content.WriteString(labelStyle.Render("Build:   ") + valueStyle.Render(version.BuildNumber) + "\n\n")

	content.WriteString(labelStyle.Render("Repo:    ") + linkStyle.Render("github.com/badenpiland/omasword") + "\n")
	content.WriteString(labelStyle.Render("Fork of: ") + linkStyle.Render("github.com/kmf/sword-tui") + "\n")
	content.WriteString(labelStyle.Render("API:     ") + valueStyle.Render("bolls.life") + "\n")
	content.WriteString(labelStyle.Render("License: ") + valueStyle.Render("GPL-2.0-or-later") + "\n\n")

	content.WriteString(titleStyle.Render("Shortcuts") + "\n\n")
	shortcuts := []struct{ key, desc string }{
		{"tab", "switch focused pane"},
		{"⏎", "open book / submit"},
		{"n / p", "next / prev chapter"},
		{"/", "go to verse"},
		{"s", "search Bible"},
		{"c", "compare translations"},
		{"t", "select translation"},
		{"d", "download translations"},
		{"y", "yank current verse"},
		{"?", "about"},
		{"q", "quit"},
	}
	for _, s := range shortcuts {
		content.WriteString(labelStyle.Render(fmt.Sprintf("%-8s", s.key)) + sectionStyle.Render(s.desc) + "\n")
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderWordSearch() string {
	bg := m.currentTheme.Background

	// Panel sizes from the available right-pane area, capped at 100 cells
	// and floored at 40 so it stays usable on narrow terminals.
	maxAvail := m.width - leftPaneOuterWidth - 8
	width := maxAvail
	if width > 100 {
		width = 100
	}
	if width < 40 {
		width = 40
	}
	// Inner content width: panel - border(2) - padding(2*2)
	innerW := width - 6

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Foreground(bg).
		Background(m.currentTheme.Accent).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg)

	bookNameStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(bg).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Search Bible") + "\n\n")

	if m.wordSearchResults == nil && !m.wordSearchLoading {
		ti := m.wordSearchInput
		ti.SetStyles(m.themedInputStyles())
		ti.SetWidth(innerW - 2) // leave a couple of cells of breathing room
		content.WriteString(ti.View() + "\n\n")
		content.WriteString(mutedStyle.Render("Type a word or phrase, then ⏎"))
	} else if m.wordSearchLoading {
		content.WriteString(mutedStyle.Render("Searching…"))
	} else if len(m.wordSearchResults) == 0 {
		content.WriteString(normalStyle.Render(fmt.Sprintf("No results for \"%s\"", m.wordSearchQuery)) + "\n\n")
		content.WriteString(mutedStyle.Render("esc to close"))
	} else {
		content.WriteString(mutedStyle.Render(fmt.Sprintf("%d results for \"%s\" — showing %d",
			m.wordSearchTotal, m.wordSearchQuery, len(m.wordSearchResults))) + "\n\n")

		// Row-based virtual scrolling: each result may wrap to multiple
		// lines, so we budget by rendered row count instead of by item.
		// Total panel rows we can afford = m.height - top header bar (3)
		// - status bar (3) - 2 cells of breathing room. Inside the panel
		// we lose: border (2) + padding (2) + title (1) + blank (1) +
		// "X results for…" (1) + blank (1) + the "↓ N more" trailer (1)
		// = 9 rows of chrome. Anything left is for the wrapped items.
		availRows := m.height - 3 - 3 - 2 - 9
		if availRows < 4 {
			availRows = 4
		}

		// Pre-render each result with wrapping so we know its row cost.
		type item struct {
			lines   []string
			isBook  bool
			isSel   bool
			origIdx int
		}
		var items []item

		bodyPrefixW := 2 // "▸ " or "  "
		refTemplate := "999:999 "
		textWidth := innerW - bodyPrefixW - len(refTemplate)
		if textWidth < 20 {
			textWidth = 20
		}

		currentBook := -1
		for i, result := range m.wordSearchResults {
			if result.Book != currentBook {
				currentBook = result.Book
				bookName := m.getBookName(result.Book)
				items = append(items, item{
					lines:  []string{"", bookNameStyle.Render(bookName)},
					isBook: true,
				})
			}
			ref := fmt.Sprintf("%-7s", fmt.Sprintf("%d:%d", result.Chapter, result.Verse))
			verseText := stripHTMLTags(result.Text)
			wrapped := wrapTextWithIndent(verseText, textWidth, 2+len(refTemplate))
			wrappedLines := strings.Split(wrapped, "\n")

			lines := make([]string, len(wrappedLines))
			for j, ln := range wrappedLines {
				var row string
				if j == 0 {
					row = ref + " " + ln
				} else {
					row = ln // continuation already has indent inside wrappedText
				}
				lines[j] = row
			}
			items = append(items, item{
				lines:   lines,
				isSel:   i == m.wordSearchSelected,
				origIdx: i,
			})
		}

		// Find the index in `items` of the selected result so we can
		// center the window around it.
		selItemIdx := -1
		for i, it := range items {
			if !it.isBook && it.origIdx == m.wordSearchSelected {
				selItemIdx = i
				break
			}
		}

		// Walk items expanding rows around the selection until we fill
		// availRows. Symmetric expansion keeps the cursor roughly centered.
		startIdx, endIdx := selItemIdx, selItemIdx+1
		if selItemIdx < 0 {
			startIdx, endIdx = 0, 0
		}
		usedRows := 0
		if selItemIdx >= 0 {
			usedRows = len(items[selItemIdx].lines)
		}
		for usedRows < availRows && (startIdx > 0 || endIdx < len(items)) {
			// Prefer expanding downward first to mimic typical list scroll.
			if endIdx < len(items) && usedRows+len(items[endIdx].lines) <= availRows {
				usedRows += len(items[endIdx].lines)
				endIdx++
			} else if startIdx > 0 && usedRows+len(items[startIdx-1].lines) <= availRows {
				startIdx--
				usedRows += len(items[startIdx].lines)
			} else {
				break
			}
		}
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(items) {
			endIdx = len(items)
		}

		// Count above/below results (excluding book headers).
		var above, below int
		for i := 0; i < startIdx; i++ {
			if !items[i].isBook {
				above++
			}
		}
		for i := endIdx; i < len(items); i++ {
			if !items[i].isBook {
				below++
			}
		}

		if above > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more", above)) + "\n")
		}

		for i := startIdx; i < endIdx; i++ {
			it := items[i]
			if it.isBook {
				for _, ln := range it.lines {
					content.WriteString(ln + "\n")
				}
				continue
			}
			for j, ln := range it.lines {
				var styled string
				if it.isSel {
					if j == 0 {
						styled = selectedStyle.Render("▸ " + ln)
					} else {
						styled = selectedStyle.Render("  " + ln)
					}
				} else {
					if j == 0 {
						styled = normalStyle.Render("  " + ln)
					} else {
						styled = normalStyle.Render("  " + ln)
					}
				}
				content.WriteString(styled + "\n")
			}
		}

		if below > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
		}
	}

	return containerStyle.Render(content.String())
}
