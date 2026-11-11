package ui

import (
	"fmt"
	"omasword/internal/esv"
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

func (m Model) renderTranslationSelect() string {
	bg := m.currentTheme.Background

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(68).
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

	mutedStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true).Padding(0, 1)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Select Translation") + "\n\n")

	if m.translations != nil {
		// Each row is a name line and a one-line flavor. Keep the window
		// in items so a flavor line is never split off its name.
		start := m.overlayWindowStart(m.translationSelected, len(m.translations), translationWindow)
		end := start + translationWindow
		if end > len(m.translations) {
			end = len(m.translations)
		}
		if start > 0 {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("↑ %d more\n", start)))
		}
		for i := start; i < end; i++ {
			trans := m.translations[i]
			prefix := "  "
			style := normalStyle
			isCurrent := trans.ShortName == m.selectedTranslation
			if i == m.translationSelected {
				prefix = "▸ "
				style = selectedStyle
			} else if isCurrent {
				style = currentStyle
			}
			name := fmt.Sprintf("%-6s  %s", trans.ShortName, trans.FullName)
			if isCurrent && i != m.translationSelected {
				name += "  ●"
			}
			content.WriteString(style.Render(prefix+name) + "\n")
			flavor := m.translationFlavor(trans.ShortName)
			flavorStyle := mutedStyle
			if i == m.translationSelected {
				flavorStyle = selectedStyle
			}
			content.WriteString(flavorStyle.Render("        "+flavor) + "\n")
		}
		if end < len(m.translations) {
			content.WriteString(mutedStyle.Render(fmt.Sprintf("↓ %d more", len(m.translations)-end)))
		}
	} else {
		content.WriteString(normalStyle.Render("  " + m.translationsLoadingText()))
	}

	return containerStyle.Render(content.String())
}

func (m Model) renderESVKey() string {
	bg := m.currentTheme.Background
	titleStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Accent).Background(bg).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Primary).Background(bg)
	hintStyle := lipgloss.NewStyle().Foreground(m.currentTheme.Muted).Background(bg).Italic(true)

	width := 72
	if m.width-leftPaneOuterWidth-6 < width && m.width > 0 {
		width = m.width - leftPaneOuterWidth - 6
	}
	if width < 48 {
		width = 48
	}
	innerW := width - 6
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Width(width).
		Padding(1, 2)

	ti := m.esvKeyInput
	ti.SetStyles(m.themedInputStyles())
	ti.SetWidth(innerW - 2)

	var body strings.Builder
	body.WriteString(titleStyle.Render("ESV API key") + "\n\n")
	body.WriteString(bodyStyle.Render("The ESV is online-only, with your own non-commercial key from Crossway. Create one at https://api.esv.org/ and paste it here.") + "\n\n")
	body.WriteString(ti.View() + "\n\n")
	body.WriteString(hintStyle.Render("Enter saves the key in config.json (mode 0600). An empty line removes it. OMASWORD_ESV_API_KEY overrides the file and is not written down."))
	return box.Render(body.String())
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
	// The subtitle sits on the line the click math treats as the gap
	// under the title, so item rows stay aligned with the mouse.
	content.WriteString(titleStyle.Render("Download Translations") + "\n")
	content.WriteString(mutedStyle.Render("Public-domain texts only") + "\n")

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
			if trans.ShortName == "ESV" {
				suffix = "  online only"
			} else if isDownloading {
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
		content.WriteString(normalStyle.Render("  " + m.translationsLoadingText()))
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
	content.WriteString(labelStyle.Render("Texts:   ") + valueStyle.Render("public domain, BSB default") + "\n")
	esvLine := "add a key from api.esv.org"
	if m.esvClient != nil && m.esvClient.HasKey() {
		esvLine = "key set, online only"
	}
	content.WriteString(labelStyle.Render("ESV:     ") + valueStyle.Render(esvLine) + "\n")
	content.WriteString(labelStyle.Render("License: ") + valueStyle.Render("GPL-2.0-or-later") + "\n\n")

	content.WriteString(titleStyle.Render("Shortcuts") + "\n\n")
	shortcuts := []struct{ key, desc string }{
		{"tab", "switch focused pane"},
		{"⏎", "open book / submit"},
		{"n / p", "next / prev chapter"},
		{"/", "go to verse"},
		{"v", "browse to a passage"},
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

	if m.selectedTranslation == esv.ID && (m.wordSearchResults != nil || m.wordSearchLoading) {
		content.WriteString("\n" + mutedStyle.Render(wrapText(esv.Attribution(m.displayedESVNotice()), innerW)))
	}

	return containerStyle.Render(content.String())
}

func (m Model) translationsLoadingText() string {
	if m.translationsErr != nil {
		return "Couldn't load translations, retrying…"
	}
	return "Loading translations..."
}
