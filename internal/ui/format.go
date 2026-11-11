package ui

import (
	"fmt"
	"omasword/internal/api"
	"omasword/internal/esv"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// comparisonVerseList returns the verse-number list we want
// loadParallelVerses to fetch. In comparison mode m.currentVerses is
// nil (cleared when parallelVersesLoadedMsg lands), so we derive the
// list from the longest column we already have. Falls back to 1..31
// when nothing's loaded yet (matches the initial "c" trigger).
func (m Model) comparisonVerseList() []int {
	maxV := 0
	if m.currentVerses != nil {
		for _, v := range m.currentVerses {
			if v.Verse > maxV {
				maxV = v.Verse
			}
		}
	}
	if m.currentParallelVerses != nil {
		for _, vs := range m.currentParallelVerses {
			for _, v := range vs {
				if v.Verse > maxV {
					maxV = v.Verse
				}
			}
		}
	}
	if maxV == 0 {
		maxV = 31
	}
	out := make([]int, maxV)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func (m Model) calculateHighlightedVerse() int {
	if m.currentVerses == nil || len(m.currentVerses) == 0 {
		return 1
	}

	// Calculate which verse is at the top of the viewport
	yOffset := m.viewport.YOffset()

	// Calculate text width the same way formatChapter does. We use the
	// viewport's width (not m.width — that's the full terminal) and
	// match formatChapter's `width - 8` for the textWidth.
	textWidth := m.viewport.Width() - 8
	if textWidth < 12 {
		textWidth = 12
	}

	// Count lines to find which verse we're at
	currentLine := 0
	indent := 6 // verse number width + 2 spaces

	for i, verse := range m.currentVerses {
		text := stripHTMLTags(verse.Text)

		// Check if this verse is highlighted (which would add a border)
		isHighlighted := m.highlightedVerseStart > 0 && verse.Verse >= m.highlightedVerseStart && verse.Verse <= m.highlightedVerseEnd

		// Check if next verse is also highlighted
		nextIsHighlighted := false
		if i+1 < len(m.currentVerses) {
			nextVerse := m.currentVerses[i+1]
			nextIsHighlighted = m.highlightedVerseStart > 0 && nextVerse.Verse >= m.highlightedVerseStart && nextVerse.Verse <= m.highlightedVerseEnd
		}

		var verseTotalLines int
		if isHighlighted {
			// Highlighted verses use narrower width due to border padding
			wrappedText := wrapTextWithIndent(text, textWidth-4, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1

			if !nextIsHighlighted {
				// End of highlighted range: wrapped content + 2 border
				// lines (top + bottom) + 1 trailing blank.
				verseTotalLines = linesInVerse + 2 + 1
			} else {
				// Middle of highlighted range
				verseTotalLines = linesInVerse + 1
			}
		} else {
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1
			verseTotalLines = linesInVerse + 1
		}

		// If the current line + verse lines exceeds yOffset, this is our verse
		if currentLine+verseTotalLines > yOffset {
			return verse.Verse
		}

		currentLine += verseTotalLines
	}

	// If we've scrolled past all verses, return the last one
	if len(m.currentVerses) > 0 {
		return m.currentVerses[len(m.currentVerses)-1].Verse
	}

	return 1
}

func (m *Model) scrollToHighlightedVerse() {
	if m.currentVerses == nil || len(m.currentVerses) == 0 {
		return
	}

	// Calculate the line position of the highlighted verse
	// This must match exactly how formatChapter renders verses
	currentLine := 0

	// Calculate text width the same way formatChapter does
	textWidth := m.width - 6
	if textWidth < 20 {
		textWidth = 20
	}
	if textWidth > m.width-2 {
		textWidth = m.width - 2
	}

	for i, verse := range m.currentVerses {
		if verse.Verse == m.highlightedVerseStart {
			// Found the verse, scroll to it
			// Keep it near the top of the viewport (with some padding)
			targetOffset := currentLine
			if targetOffset < 0 {
				targetOffset = 0
			}

			// Get the total number of lines in content
			totalLines := strings.Count(m.content, "\n") + 1
			maxOffset := totalLines - m.viewport.Height()
			if maxOffset < 0 {
				maxOffset = 0
			}

			if targetOffset > maxOffset {
				targetOffset = maxOffset
			}

			m.viewport.SetYOffset(targetOffset)
			return
		}

		// Calculate lines for this verse - must match formatChapter logic
		text := stripHTMLTags(verse.Text)
		indent := 6 // verse number width + 2 spaces

		// Check if this verse is highlighted (which would add a border)
		isHighlighted := m.highlightedVerseStart > 0 && verse.Verse >= m.highlightedVerseStart && verse.Verse <= m.highlightedVerseEnd

		// Check if next verse is also highlighted
		nextIsHighlighted := false
		if i+1 < len(m.currentVerses) {
			nextVerse := m.currentVerses[i+1]
			nextIsHighlighted = m.highlightedVerseStart > 0 && nextVerse.Verse >= m.highlightedVerseStart && nextVerse.Verse <= m.highlightedVerseEnd
		}

		if isHighlighted {
			// Highlighted verses use narrower width due to border padding
			wrappedText := wrapTextWithIndent(text, textWidth-4, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1

			if !nextIsHighlighted {
				// End of highlighted range - border adds 2 lines (top + bottom)
				// Plus verse lines, plus blank line after
				verseTotalLines := linesInVerse + 2 + 2 // border top/bottom + blank line + content
				currentLine += verseTotalLines
			} else {
				// Middle of highlighted range - just the verse text + blank line within border
				verseTotalLines := linesInVerse + 1 // +1 for blank line between verses in border
				currentLine += verseTotalLines
			}
		} else {
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			linesInVerse := strings.Count(wrappedText, "\n") + 1
			verseTotalLines := linesInVerse + 1 // +1 for blank line after verse
			currentLine += verseTotalLines
		}
	}
}

func (m Model) formatChapter(verses []api.Verse, bookName string, chapter int, width int, highlightedVerseStart, highlightedVerseEnd int) string {
	bg := m.currentTheme.Background
	hbg := m.currentTheme.Highlight

	verseStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(bg).
		Bold(true).
		Width(4).
		Align(lipgloss.Right)

	highlightedVerseStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(hbg).
		Bold(true).
		Width(4).
		Align(lipgloss.Right)

	textStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg)

	highlightedTextStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(hbg).
		Bold(true)

	highlightedContainerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.currentTheme.BorderActive).
		BorderBackground(bg).
		Background(hbg).
		Padding(0, 1)

	// Styled separators so the gap between verse number and text gets the
	// pane background, and the trailing blank line between verses does too.
	sep := lipgloss.NewStyle().Background(bg).Render("  ")
	hsep := lipgloss.NewStyle().Background(hbg).Render("  ")
	blankLine := lipgloss.NewStyle().Background(bg).Width(width).Render("")
	bgPadStyle := lipgloss.NewStyle().Background(bg)
	padToWidth := func(line string) string {
		w := lipgloss.Width(line)
		if w >= width {
			return line
		}
		return line + bgPadStyle.Render(strings.Repeat(" ", width-w))
	}

	var sb strings.Builder

	// Calculate available width for text. Verse number is right-aligned
	// in 4 chars + 2 spaces = 6 chars total. We leave an extra 2 cells of
	// safety so the highlighted-verse rounded box (which costs 6 cells of
	// border+padding around the inner text) doesn't equal viewport width
	// exactly (lipgloss wraps on exact-width matches).
	textWidth := width - 8
	if textWidth < 20 {
		textWidth = 20 // Minimum width for readability
	}
	if textWidth > width-2 {
		textWidth = width - 2
	}

	// Track if we're currently in a highlighted range
	inHighlightedRange := false
	var highlightedContent strings.Builder

	for i, v := range verses {
		// Remove HTML tags
		text := stripHTMLTags(v.Text)
		verseNumStr := fmt.Sprintf("%d", v.Verse)

		// Check if this verse is in the highlighted range
		isHighlighted := highlightedVerseStart > 0 && v.Verse >= highlightedVerseStart && v.Verse <= highlightedVerseEnd

		// Check if next verse is also highlighted
		nextIsHighlighted := false
		if i+1 < len(verses) {
			nextVerse := verses[i+1]
			nextIsHighlighted = highlightedVerseStart > 0 && nextVerse.Verse >= highlightedVerseStart && nextVerse.Verse <= highlightedVerseEnd
		}

		if isHighlighted {
			if !inHighlightedRange {
				// Start of highlighted range
				inHighlightedRange = true
				highlightedContent.Reset()
			}

			verseNum := highlightedVerseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number width + 2 spaces)
			indent := 6
			// Account for border padding (2 chars on each side)
			wrappedText := wrapTextWithIndent(text, textWidth-4, indent)
			// Apply color with width set to prevent terminal wrapping
			verseText := highlightedTextStyle.Width(textWidth - 4).Render(wrappedText)

			highlightedContent.WriteString(verseNum + hsep + verseText)

			// If next verse is also highlighted, add spacing within the border
			if nextIsHighlighted {
				highlightedContent.WriteString("\n\n")
			} else {
				// End of highlighted range - render the border, then pad
				// each rendered row out to width so the right edge meets
				// the pane background instead of the terminal default.
				borderedVerse := highlightedContainerStyle.Render(highlightedContent.String())
				for _, ln := range strings.Split(borderedVerse, "\n") {
					sb.WriteString(padToWidth(ln) + "\n")
				}
				sb.WriteString(blankLine + "\n")
				inHighlightedRange = false
			}
		} else {
			verseNum := verseStyle.Render(verseNumStr)

			// Calculate indent for wrapped lines (verse number width + 2 spaces)
			indent := 6
			wrappedText := wrapTextWithIndent(text, textWidth, indent)
			verseText := textStyle.Width(textWidth).Render(wrappedText)

			// Each wrapped line of the verse is verseNum (4) + sep (2) +
			// verseText (textWidth). The continuation lines already carry
			// their leading indent inside wrappedText (from wrapTextWithIndent),
			// so we only prepend the verse-number block on the first line.
			// padToWidth then fills the right edge with bg for every row.
			textLines := strings.Split(verseText, "\n")
			for idx, ln := range textLines {
				if idx == 0 {
					sb.WriteString(padToWidth(verseNum+sep+ln) + "\n")
				} else {
					sb.WriteString(padToWidth(ln) + "\n")
				}
			}
			sb.WriteString(blankLine + "\n")
		}
	}

	if m.selectedTranslation == esv.ID {
		sb.WriteString(m.esvNoticeBlock(width))
	}
	return sb.String()
}

// esvNoticeBlock is the Crossway copyright notice, the esv.org link, and
// a paging hint when this chapter was clipped to the per-page cap.
func (m Model) esvNoticeBlock(width int) string {
	if width < 20 {
		width = 20
	}
	var text strings.Builder
	if m.esvTotal > 0 && (m.esvFirst > 1 || m.esvLast < m.esvTotal) {
		fmt.Fprintf(&text, "Showing verses %d–%d of %d. Press , or . for another page. …\n", m.esvFirst, m.esvLast, m.esvTotal)
	}
	text.WriteString(esv.Attribution(m.displayedESVNotice()))
	style := lipgloss.NewStyle().
		Foreground(m.currentTheme.Muted).
		Background(m.currentTheme.Background).
		Italic(true).
		Width(width)
	return "\n" + style.Render(wrapText(text.String(), width)) + "\n"
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentLength := 0

	words := strings.Fields(text)
	for i, word := range words {
		wordLen := len(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordLen > width {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			currentLine.Reset()
			currentLength = 0
		}

		// Add space before word (except at start of line)
		if currentLength > 0 {
			currentLine.WriteString(" ")
			currentLength++
		}

		currentLine.WriteString(word)
		currentLength += wordLen

		// If this is the last word, write it out
		if i == len(words)-1 {
			result.WriteString(currentLine.String())
		}
	}

	return result.String()
}

func wrapTextWithIndent(text string, width int, indent int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentLength := 0
	isFirstLine := true

	words := strings.Fields(text)
	for i, word := range words {
		wordLen := len(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordLen > width {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			currentLine.Reset()
			currentLength = 0
			isFirstLine = false
		}

		// Add indent for continuation lines
		if currentLength == 0 && !isFirstLine {
			currentLine.WriteString(strings.Repeat(" ", indent))
			currentLength = indent
		}

		// Add space before word (except at the very start of a line where currentLength is 0)
		if currentLength > 0 {
			currentLine.WriteString(" ")
			currentLength++
		}

		currentLine.WriteString(word)
		currentLength += wordLen

		// If this is the last word, write it out
		if i == len(words)-1 {
			result.WriteString(currentLine.String())
		}
	}

	return result.String()
}

func (m Model) formatParallelVerses(versesMap map[string][]api.Verse, translations []string, bookName string, chapter int, width int) string {
	if len(translations) == 0 {
		return ""
	}

	bg := m.currentTheme.Background

	headerStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Accent).
		Background(bg).
		Bold(true)
	verseNumStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Warning).
		Background(bg).
		Bold(true)
	textStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Primary).
		Background(bg)
	separatorStyle := lipgloss.NewStyle().
		Foreground(m.currentTheme.Border).
		Background(bg)
	bgPad := lipgloss.NewStyle().Background(bg)

	// Column geometry: split the available width across N translations,
	// leaving a 1-cell gutter between each pair.
	n := len(translations)
	gaps := n - 1
	colWidth := (width - gaps) / n
	if colWidth < 20 {
		colWidth = 20
	}
	// Inner text width allows for a "NN " verse number prefix (4 cells).
	textWidth := colWidth - 4
	if textWidth < 12 {
		textWidth = 12
	}

	// padCol pads a logical column line to colWidth using bg-styled
	// spaces so the rows merge into a clean grid regardless of theme.
	padCol := func(s string) string {
		w := lipgloss.Width(s)
		if w >= colWidth {
			return s
		}
		return s + bgPad.Render(strings.Repeat(" ", colWidth-w))
	}

	// maxVerses across all translations.
	maxVerses := 0
	for _, vs := range versesMap {
		if len(vs) > maxVerses {
			maxVerses = len(vs)
		}
	}

	// Build the header row: one column per translation, padded to colWidth.
	// "▾" hints that the header opens a translation picker on click.
	headerCells := make([]string, n)
	for j, trans := range translations {
		label := trans + " ▾"
		if lipgloss.Width(label) > colWidth {
			label = label[:colWidth]
		}
		headerCells[j] = padCol(headerStyle.Render(label))
	}
	gutter := bgPad.Render(" ")
	header := strings.Join(headerCells, gutter)

	// Separator row under the header.
	sepRow := padCol(separatorStyle.Render(strings.Repeat("─", colWidth)))
	separator := strings.Join(repeatString(sepRow, n), gutter)

	// For each verse number, build a row of columns and JoinHorizontal
	// them. JoinHorizontal automatically pads shorter columns at the
	// bottom so all rows in a verse stay aligned.
	var rows []string
	rows = append(rows, header, separator)

	for i := 1; i <= maxVerses; i++ {
		cells := make([]string, n)
		for j, trans := range translations {
			verses := versesMap[trans]
			var text string
			for _, v := range verses {
				if v.Verse == i {
					text = stripHTMLTags(v.Text)
					break
				}
			}
			if text == "" {
				cells[j] = padCol("")
				continue
			}
			// First line: "N  text…", continuation lines indent under
			// the text so the verse number stays as a visual anchor.
			wrapped := wrapTextWithIndent(text, textWidth, 4)
			lines := strings.Split(wrapped, "\n")
			styled := make([]string, len(lines))
			for k, ln := range lines {
				if k == 0 {
					styled[k] = padCol(verseNumStyle.Render(fmt.Sprintf("%-3d", i)) + bgPad.Render(" ") + textStyle.Render(ln))
				} else {
					styled[k] = padCol(textStyle.Render(ln))
				}
			}
			cells[j] = strings.Join(styled, "\n")
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, intersperse(cells, gutter)...))
		// Blank row between verses, styled in bg so it covers the full
		// width and the grid stays painted.
		blankRow := padCol("")
		rows = append(rows, strings.Join(repeatString(blankRow, n), gutter))
	}

	body := strings.Join(rows, "\n")
	for _, name := range translations {
		if name == esv.ID {
			return body + "\n" + m.esvNoticeBlock(width)
		}
	}
	return body
}

func repeatString(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// intersperse returns ss with sep placed between each consecutive pair.
// "a","b","c" + "|" → "a","|","b","|","c"
func intersperse(ss []string, sep string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := make([]string, 0, 2*len(ss)-1)
	for i, s := range ss {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, s)
	}
	return out
}

func stripHTMLTags(s string) string {
	// Strip HTML tags. The bolls.life API wraps the matched search term
	// in <em>…</em> *inside* words (e.g. "lov<em>e</em>d"), so replacing
	// tags with a space would split such words. Drop them outright and
	// collapse any resulting double spaces at the end.
	s = strongsRe.ReplaceAllString(s, "")
	s = htmlTagRe.ReplaceAllString(s, "")

	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&ldquo;", "\u201C")  // Left double quote
	s = strings.ReplaceAll(s, "&rdquo;", "\u201D")  // Right double quote
	s = strings.ReplaceAll(s, "&lsquo;", "\u2018")  // Left single quote
	s = strings.ReplaceAll(s, "&rsquo;", "\u2019")  // Right single quote
	s = strings.ReplaceAll(s, "&mdash;", "\u2014")  // Em dash
	s = strings.ReplaceAll(s, "&ndash;", "\u2013")  // En dash
	s = strings.ReplaceAll(s, "&hellip;", "\u2026") // Ellipsis

	// Decode numeric HTML entities (e.g., &#8220; for left double quote)
	s = numericEntityRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the numeric code
		numStr := match[2 : len(match)-1]
		if num, err := strconv.Atoi(numStr); err == nil && num < 0x110000 {
			return string(rune(num))
		}
		return match
	})

	// Decode hex HTML entities (e.g., &#x201C; for left double quote)
	s = hexEntityRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the hex code
		hexStr := match[3 : len(match)-1]
		if num, err := strconv.ParseInt(hexStr, 16, 32); err == nil && num < 0x110000 {
			return string(rune(num))
		}
		return match
	})

	// Clean up multiple consecutive spaces
	s = spaceRunRe.ReplaceAllString(s, " ")

	// Trim leading and trailing spaces
	s = strings.TrimSpace(s)

	// Entities decoded above can produce control characters (&#27; is
	// ESC), so sanitize last.
	return api.SanitizeText(s)
}
