package ui

import (
	"fmt"
	"omasword/internal/api"
	"omasword/internal/settings"
	"omasword/internal/theme"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// setTheme applies th and redraws the reading pane, whose text is
// pre-rendered with the theme's colors baked in.
func (m *Model) setTheme(th theme.Theme) {
	m.currentTheme = th
	m.rerenderContent()
}

// rerenderContent rebuilds the viewport content for the current chapter
// or comparison at the current width and theme, keeping the scroll
// position.
func (m *Model) rerenderContent() {
	if !m.ready {
		return
	}
	w := m.viewport.Width()
	switch {
	case m.mode == modeComparison && m.currentParallelVerses != nil:
		m.content = m.formatParallelVerses(m.currentParallelVerses, m.comparisonTranslations, m.currentBookName, m.currentChapter, w)
	case m.currentVerses != nil:
		m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, w, m.highlightedVerseStart, m.highlightedVerseEnd)
	case m.currentParallelVerses != nil:
		m.content = m.formatParallelVerses(m.currentParallelVerses, m.comparisonTranslations, m.currentBookName, m.currentChapter, w)
	default:
		return
	}
	y := m.viewport.YOffset()
	m.viewport.SetContent(m.content)
	m.viewport.SetYOffset(y)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// "q" is a letter like any other while a text input has
			// focus; only ctrl+c quits from there.
			if msg.String() == "q" && m.typingInInput() {
				break
			}
			_ = m.saveSettings()
			return m, tea.Quit
		case "[":
			if m.mode == modeReader {
				m.focus = paneBooks
				if m.books != nil {
					for i, book := range m.books {
						if book.BookID == m.currentBook {
							m.sidebarSelected = i
							break
						}
					}
				}
				return m, nil
			}
		case "]":
			if m.mode == modeReader {
				m.focus = paneContent
				return m, nil
			}
		case "tab":
			if m.mode == modeReader {
				if m.focus == paneBooks {
					m.focus = paneContent
				} else {
					m.focus = paneBooks
					if m.books != nil && m.sidebarSelected == 0 {
						for i, book := range m.books {
							if book.BookID == m.currentBook {
								m.sidebarSelected = i
								break
							}
						}
					}
				}
				return m, nil
			}
		case "shift+tab":
			if m.mode == modeReader {
				if m.focus == paneBooks {
					m.focus = paneContent
				} else {
					m.focus = paneBooks
				}
				return m, nil
			}
		case "v":
			if m.mode == modeReader {
				m.showMillerColumns = !m.showMillerColumns
				if m.showMillerColumns && m.books != nil {
					// Initialize Miller columns with current position
					for i, book := range m.books {
						if book.BookID == m.currentBook {
							m.millerBookIdx = i
							break
						}
					}
					m.millerChapterIdx = m.currentChapter - 1
					m.millerVerseIdx = 0
					m.millerColumn = 0
					// Reset filter
					m.millerFilterInput.SetValue("")
					m.millerFilter = ""
					m.millerFilteredBooks = nil
					m.millerFilteredVerses = nil
					m.millerFilterMode = false
				}
				return m, nil
			}
		case "/":
			if m.showMillerColumns {
				// Toggle filter mode in Miller columns
				m.millerFilterMode = !m.millerFilterMode
				if m.millerFilterMode {
					m.millerFilterInput.Focus()
				} else {
					m.millerFilterInput.Blur()
				}
				return m, nil
			} else if m.mode == modeReader {
				// Close sidebar if open when entering search mode
				m.focus = paneContent
				m.mode = modeSearch
				m.searchErr = nil
				m.textInput.Focus()
				return m, nil
			}
		case "up", "k":
			if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected > 0 {
				m.wordSearchSelected--
				return m, nil
			} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected > 0 {
				m.translationSelected--
				return m, nil
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected > 0 {
				m.cacheSelected--
				return m, nil
			} else if m.showMillerColumns && !m.millerFilterMode {
				switch m.millerColumn {
				case 0: // Books column
					if m.millerBookIdx > 0 {
						m.millerBookIdx--
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1: // Chapters column
					if m.millerChapterIdx > 0 {
						m.millerChapterIdx--
						m.millerVerseIdx = 0
					}
				case 2: // Verses column
					if m.millerVerseIdx > 0 {
						m.millerVerseIdx--
					}
				}
				return m, nil
			} else if m.focus == paneBooks && m.sidebarSelected > 0 {
				m.sidebarSelected--
				return m, nil
			} else if m.mode == modeReader && m.currentVerses != nil {
				// Navigate to previous verse
				currentIdx := -1
				for i, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentIdx = i
						break
					}
				}
				if currentIdx > 0 {
					m.highlightedVerseStart = m.currentVerses[currentIdx-1].Verse
					m.highlightedVerseEnd = m.highlightedVerseStart
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
					m.scrollToHighlightedVerse()
				}
				return m, nil
			}
		case "down", "j":
			if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected < len(m.wordSearchResults)-1 {
				m.wordSearchSelected++
				return m, nil
			} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations)-1 {
				m.translationSelected++
				return m, nil
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations)-1 {
				m.cacheSelected++
				return m, nil
			} else if m.showMillerColumns && !m.millerFilterMode && m.books != nil {
				switch m.millerColumn {
				case 0: // Books column
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if m.millerBookIdx < len(booksToUse)-1 {
						m.millerBookIdx++
						m.millerChapterIdx = 0
						m.millerVerseIdx = 0
					}
				case 1: // Chapters column
					booksToUse := m.books
					if m.millerFilter != "" && m.millerFilteredBooks != nil {
						booksToUse = m.millerFilteredBooks
					}
					if m.millerBookIdx < len(booksToUse) {
						selectedBook := booksToUse[m.millerBookIdx]
						if m.millerChapterIdx < selectedBook.Chapters-1 {
							m.millerChapterIdx++
							m.millerVerseIdx = 0
						}
					}
				case 2: // Verses column
					versesToUse := m.currentVerses
					if m.millerFilter != "" && m.millerFilteredVerses != nil {
						versesToUse = m.millerFilteredVerses
					}
					if versesToUse != nil && m.millerVerseIdx < len(versesToUse)-1 {
						m.millerVerseIdx++
					}
				}
				return m, nil
			} else if m.focus == paneBooks && m.books != nil && m.sidebarSelected < len(m.books)-1 {
				m.sidebarSelected++
				return m, nil
			} else if m.mode == modeReader && m.currentVerses != nil {
				// Navigate to next verse
				currentIdx := -1
				for i, v := range m.currentVerses {
					if v.Verse == m.highlightedVerseStart {
						currentIdx = i
						break
					}
				}
				if currentIdx >= 0 && currentIdx < len(m.currentVerses)-1 {
					m.highlightedVerseStart = m.currentVerses[currentIdx+1].Verse
					m.highlightedVerseEnd = m.highlightedVerseStart
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
					m.scrollToHighlightedVerse()
				}
				return m, nil
			}
		case "left", "h":
			if m.showMillerColumns && !m.millerFilterMode && m.millerColumn > 0 {
				m.millerColumn--
				return m, nil
			}
		case "right", "l":
			if m.showMillerColumns && !m.millerFilterMode {
				if m.millerColumn < 2 {
					m.millerColumn++
					// When moving to verses column, load the chapter if not already loaded
					if m.millerColumn == 2 {
						booksToUse := m.books
						if m.millerFilter != "" && m.millerFilteredBooks != nil {
							booksToUse = m.millerFilteredBooks
						}
						if m.millerBookIdx < len(booksToUse) {
							selectedBook := booksToUse[m.millerBookIdx]
							selectedChapter := m.millerChapterIdx + 1
							// Only load if different from current
							if selectedBook.BookID != m.currentBook || selectedChapter != m.currentChapter {
								return m, loadChapter(m.client, m.selectedTranslation, selectedBook.BookID, selectedChapter)
							}
						}
					}
				}
				return m, nil
			}
		case "c":
			if m.mode == modeReader {
				m.mode = modeComparison
				verses := []int{}
				for i := 1; i <= 31; i++ {
					verses = append(verses, i)
				}
				return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, verses)
			}
		case "r":
			// Don't intercept 'r' when typing in search inputs
			if m.mode == modeSearch {
				// Let it pass through to verse reference input
			} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
				// Let it pass through to word search input
			} else if m.mode != modeReader {
				m.mode = modeReader
				return m, nil
			}
		case "t":
			if m.mode == modeReader {
				m.mode = modeTranslationSelect
				m.translationSelected = 0
				// Find current translation in list
				if m.translations != nil {
					for i, trans := range m.translations {
						if trans.ShortName == m.selectedTranslation {
							m.translationSelected = i
							break
						}
					}
				}
				return m, nil
			}
		case "d":
			if m.mode == modeReader {
				m.mode = modeCacheManager
				m.cacheSelected = 0
				if m.cache != nil {
					return m, loadCachedList(m.cache)
				}
				return m, nil
			}
		case "?":
			if m.mode == modeReader {
				m.mode = modeAbout
				return m, nil
			}
		case "s":
			if m.mode == modeReader {
				m.mode = modeWordSearch
				m.wordSearchInput.SetValue("")
				m.wordSearchInput.Focus()
				m.wordSearchResults = nil
				m.wordSearchSelected = 0
				m.wordSearchLoading = false
				return m, nil
			}
		case "n", "pgdown":
			if m.mode == modeReader {
				return m, m.stepChapter(1)
			}
		case "p", "pgup":
			if m.mode == modeReader {
				return m, m.stepChapter(-1)
			}
		case "y":
			// Yank (copy) highlighted verse(s) or current chapter to clipboard
			if m.mode == modeReader && m.currentVerses != nil {
				var textToCopy strings.Builder

				// If verses are highlighted, only copy those
				if m.highlightedVerseStart > 0 {
					if m.highlightedVerseStart == m.highlightedVerseEnd {
						textToCopy.WriteString(fmt.Sprintf("%s %s %d:%d\n\n", m.selectedTranslation, m.currentBookName, m.currentChapter, m.highlightedVerseStart))
					} else {
						textToCopy.WriteString(fmt.Sprintf("%s %s %d:%d-%d\n\n", m.selectedTranslation, m.currentBookName, m.currentChapter, m.highlightedVerseStart, m.highlightedVerseEnd))
					}

					for _, v := range m.currentVerses {
						if v.Verse >= m.highlightedVerseStart && v.Verse <= m.highlightedVerseEnd {
							text := stripHTMLTags(v.Text)
							textToCopy.WriteString(fmt.Sprintf("%d. %s\n\n", v.Verse, text))
						}
					}
				} else {
					// Copy entire chapter
					textToCopy.WriteString(fmt.Sprintf("%s %s %d\n\n", m.selectedTranslation, m.currentBookName, m.currentChapter))

					for _, v := range m.currentVerses {
						text := stripHTMLTags(v.Text)
						textToCopy.WriteString(fmt.Sprintf("%d. %s\n\n", v.Verse, text))
					}
				}

				clipboard.WriteAll(textToCopy.String())
			}
		case "enter":
			if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations) {
				newTrans := m.translations[m.translationSelected].ShortName
				// Picker was opened from a comparison column header:
				// swap that column instead of changing the main reader.
				if m.comparisonPickerColumn >= 0 && m.comparisonPickerColumn < len(m.comparisonTranslations) {
					m.comparisonTranslations[m.comparisonPickerColumn] = newTrans
					m.comparisonPickerColumn = -1
					m.mode = modeComparison
					m.loading = true
					return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, m.comparisonVerseList())
				}
				m.selectedTranslation = newTrans
				m.mode = modeReader
				m.loading = true
				return m, tea.Batch(
					loadBooks(m.client, m.selectedTranslation),
					loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
				)
			} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
				// Download selected translation
				translation := m.translations[m.cacheSelected].ShortName
				if m.cache != nil && !m.cache.IsCached(translation) {
					m.downloadingTranslation = translation
					m.downloadProgress = 0
					return m, tea.Batch(downloadTranslation(m.cache, translation), downloadTick())
				}
				return m, nil
			} else if m.showMillerColumns && m.millerFilterMode {
				// Exit filter mode on enter
				m.millerFilterMode = false
				m.millerFilterInput.Blur()
				return m, nil
			} else if m.showMillerColumns && m.books != nil && m.currentVerses != nil {
				// Navigate to the selected verse
				booksToUse := m.books
				if m.millerFilter != "" && m.millerFilteredBooks != nil {
					booksToUse = m.millerFilteredBooks
				}
				if m.millerBookIdx < len(booksToUse) {
					selectedBook := booksToUse[m.millerBookIdx]
					selectedChapter := m.millerChapterIdx + 1
					m.currentBook = selectedBook.BookID
					m.currentBookName = selectedBook.Name
					m.currentChapter = selectedChapter
					m.showMillerColumns = false
					m.loading = true
					m.highlightedVerseStart = 0
					m.highlightedVerseEnd = 0
					// Scroll viewport to the selected verse
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.focus == paneBooks && m.books != nil {
				// Select book from sidebar
				if m.sidebarSelected < len(m.books) {
					m.currentBook = m.books[m.sidebarSelected].BookID
					m.currentBookName = m.books[m.sidebarSelected].Name
					m.currentChapter = 1
					m.focus = paneContent
					m.loading = true
					m.highlightedVerseStart = 0
					m.highlightedVerseEnd = 0
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.mode == modeSearch {
				input := m.textInput.Value()
				book, chapter, verseStart, verseEnd, err := parseReference(input, m.books)
				var target api.Book
				if err == nil {
					target, chapter, verseStart, verseEnd, err = validateReference(m.books, book, chapter, verseStart, verseEnd)
				}
				m.searchErr = err
				if err == nil {
					m.currentBook = target.BookID
					m.currentChapter = chapter
					m.highlightedVerseStart = verseStart
					m.highlightedVerseEnd = verseEnd
					m.currentBookName = target.Name

					m.mode = modeReader
					m.loading = true
					m.textInput.SetValue("")
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.mode == modeWordSearch {
				if m.wordSearchResults == nil && !m.wordSearchLoading {
					query := m.wordSearchInput.Value()
					if query != "" {
						// If the query contains digits AND parses as a verse
						// reference (e.g. "rom8", "rom 8:8", "john 3:16"),
						// jump there instead of doing a full-text search.
						// Plain words like "love" or "rom" fall through to
						// the full-text path.
						if strings.ContainsAny(query, "0123456789") {
							book, chapter, vs, ve, refErr := parseReference(query, m.books)
							var target api.Book
							if refErr == nil {
								target, chapter, vs, ve, refErr = validateReference(m.books, book, chapter, vs, ve)
							}
							if refErr == nil {
								m.currentBook = target.BookID
								m.currentChapter = chapter
								m.highlightedVerseStart = vs
								m.highlightedVerseEnd = ve
								m.currentBookName = target.Name
								m.mode = modeReader
								m.loading = true
								m.wordSearchInput.SetValue("")
								m.wordSearchInput.Blur()
								return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
							}
						}
						m.wordSearchLoading = true
						m.wordSearchInput.Blur()
						return m, loadSearchResults(m.client, m.selectedTranslation, query)
					}
				} else if m.wordSearchResults != nil && len(m.wordSearchResults) > 0 {
					// Navigate to selected result
					result := m.wordSearchResults[m.wordSearchSelected]
					m.currentBook = result.Book
					m.currentChapter = result.Chapter
					m.highlightedVerseStart = result.Verse
					m.highlightedVerseEnd = result.Verse

					// Look up the book name
					for _, b := range m.books {
						if b.BookID == result.Book {
							m.currentBookName = b.Name
							break
						}
					}

					m.mode = modeReader
					m.loading = true
					return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
				}
			} else if m.mode == modeTranslationSelect {
				// Simple translation selection (cycle through common ones)
				translations := []string{"ESV", "NLT", "KJV", "ASV", "WEB", "YLT", "DARBY"}
				for i, t := range translations {
					if t == m.selectedTranslation {
						m.selectedTranslation = translations[(i+1)%len(translations)]
						break
					}
				}
				m.mode = modeReader
				m.loading = true
				return m, tea.Batch(
					loadBooks(m.client, m.selectedTranslation),
					loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
				)
			}
		case "x":
			// Delete cached translation
			if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
				translation := m.translations[m.cacheSelected].ShortName
				if m.cache != nil && m.cache.IsCached(translation) {
					if err := m.cache.RemoveTranslation(translation); err == nil {
						return m, loadCachedList(m.cache)
					}
				}
				return m, nil
			}
		case "esc":
			if m.mode == modeCacheManager {
				m.mode = modeReader
				return m, nil
			}
			if m.showMillerColumns && m.millerFilterMode {
				// Exit filter mode on esc
				m.millerFilterMode = false
				m.millerFilterInput.Blur()
				return m, nil
			} else if m.showMillerColumns {
				m.showMillerColumns = false
				return m, nil
			}
			if m.mode == modeSearch || m.mode == modeTranslationSelect || m.mode == modeAbout || m.mode == modeComparison || m.mode == modeWordSearch || m.mode == modeCacheManager {
				// Picker was opened from a comparison column: dismiss
				// it back into comparison view instead of dropping all
				// the way down to the reader.
				if m.mode == modeTranslationSelect && m.comparisonPickerColumn >= 0 {
					m.comparisonPickerColumn = -1
					m.mode = modeComparison
					return m, nil
				}
				m.mode = modeReader
				m.wordSearchResults = nil
				m.wordSearchInput.SetValue("")
				return m, nil
			}
			if m.focus == paneBooks {
				m.focus = paneContent
				return m, nil
			}
		}

	case tea.MouseClickMsg:
		m.mouseX, m.mouseY = msg.X, msg.Y
		if msg.Button != tea.MouseLeft {
			break
		}

		// Overlay is active: clicks inside its panel select items;
		// clicks outside close the overlay.
		if m.overlayActive() {
			px, py, pw, ph := m.overlayPanelBounds()
			inside := msg.X >= px && msg.X < px+pw && msg.Y >= py && msg.Y < py+ph
			if !inside {
				// Picker scoped to a comparison column drops back to
				// comparison view, not all the way to the reader.
				if m.mode == modeTranslationSelect && m.comparisonPickerColumn >= 0 {
					m.comparisonPickerColumn = -1
					m.mode = modeComparison
					return m, nil
				}
				m.mode = modeReader
				m.wordSearchResults = nil
				m.wordSearchInput.SetValue("")
				return m, nil
			}
			// Inner content area sits one row in from the top border and
			// one row down for the box's top padding, plus the panel's
			// title line and its trailing blank line. Item rows then start.
			rowInPanel := msg.Y - py - 1 - 1 - 1 - 1
			cmd := m.overlayClick(rowInPanel)
			return m, cmd
		}

		// Click in the left (books) pane — select & load that book.
		if msg.X >= 0 && msg.X < leftPaneOuterWidth && msg.Y >= headerOuterHeight+2 {
			m.focus = paneBooks
			if i, ok := m.bookAtRow(msg.Y); ok {
				m.sidebarSelected = i
				m.currentBook = m.books[i].BookID
				m.currentBookName = m.books[i].Name
				m.currentChapter = 1
				m.focus = paneContent
				m.loading = true
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
			}
			return m, nil
		}

		// Comparison view: click on a column header opens the
		// translation picker scoped to that column.
		if m.mode == modeComparison && msg.X >= leftPaneOuterWidth {
			viewportTopY := headerOuterHeight + 4 // app header (3) + pane border (1) + top pad (1) + title (1) + spacer (1) - 2
			// Header occupies the first 2 lines of the viewport
			// (translation labels + ─ separator). Only trigger when
			// the viewport is at the top so the click coordinates match.
			if m.viewport.YOffset() == 0 && (msg.Y == viewportTopY || msg.Y == viewportTopY+1) {
				if col := m.comparisonColumnAtX(msg.X); col >= 0 {
					m.comparisonPickerColumn = col
					m.mode = modeTranslationSelect
					m.translationSelected = 0
					if m.translations != nil {
						for i, t := range m.translations {
							if t.ShortName == m.comparisonTranslations[col] {
								m.translationSelected = i
								break
							}
						}
					}
					return m, nil
				}
			}
		}

		// Click in the right (content) pane — focus it, and if the click
		// landed on a verse line, treat it as a selection.
		//
		//   plain left-click       → highlight just that verse, record
		//                            it as the drag anchor so a
		//                            subsequent drag extends a range
		//   shift+left-click       → extend the existing highlight to
		//                            span from the previous anchor to
		//                            the clicked verse
		//   left-click and drag    → range grows live as the mouse moves
		//                            (handled in MouseMotionMsg)
		//
		// The range is always normalized so start ≤ end.
		if msg.X >= leftPaneOuterWidth {
			m.focus = paneContent
			if v := m.verseAtMouseY(msg.Y); v > 0 && m.mode == modeReader && m.currentVerses != nil {
				if msg.Mod&tea.ModShift != 0 && m.highlightedVerseStart > 0 {
					anchor := m.highlightedVerseStart
					if m.highlightedVerseStart > m.highlightedVerseEnd {
						anchor = m.highlightedVerseEnd
					}
					start, end := anchor, v
					if start > end {
						start, end = end, start
					}
					m.highlightedVerseStart = start
					m.highlightedVerseEnd = end
				} else {
					m.highlightedVerseStart = v
					m.highlightedVerseEnd = v
					m.dragAnchorVerse = v
				}
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
			}
		}

	case tea.MouseReleaseMsg:
		m.mouseX, m.mouseY = msg.X, msg.Y
		// End of a click-and-drag selection.
		m.dragAnchorVerse = 0

	case tea.MouseMotionMsg:
		// Skip the model update when the cursor didn't actually change
		// cells so we avoid a re-render on every micro-movement.
		if msg.X == m.mouseX && msg.Y == m.mouseY {
			return m, nil
		}
		m.mouseX, m.mouseY = msg.X, msg.Y

		// If the left button is held AND we recorded a drag anchor on
		// the original click, extend the highlight range live to span
		// from the anchor to whichever verse is now under the cursor.
		// This is the modifier-less alternative to shift+click — any
		// terminal that passes mouse motion through (the same ones
		// that pass clicks) will let it work.
		if msg.Button == tea.MouseLeft && m.dragAnchorVerse > 0 && m.mode == modeReader && m.currentVerses != nil {
			if v := m.verseAtMouseY(msg.Y); v > 0 {
				start, end := m.dragAnchorVerse, v
				if start > end {
					start, end = end, start
				}
				if start != m.highlightedVerseStart || end != m.highlightedVerseEnd {
					m.highlightedVerseStart = start
					m.highlightedVerseEnd = end
					m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
					m.viewport.SetContent(m.content)
				}
			}
		}
		return m, nil

	case tea.MouseWheelMsg:
		m.mouseX, m.mouseY = msg.X, msg.Y
		// Inside an active overlay, the wheel navigates its selection.
		if m.overlayActive() {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.overlayNudge(-1)
			case tea.MouseWheelDown:
				m.overlayNudge(+1)
			}
			return m, nil
		}
		// Inside the books pane, the wheel moves the highlighted book.
		if msg.X >= 0 && msg.X < leftPaneOuterWidth && m.books != nil {
			switch msg.Button {
			case tea.MouseWheelUp:
				if m.sidebarSelected > 0 {
					m.sidebarSelected--
				}
			case tea.MouseWheelDown:
				if m.sidebarSelected < len(m.books)-1 {
					m.sidebarSelected++
				}
			}
			m.focus = paneBooks
			return m, nil
		}
		// Otherwise forward to the viewport in the content pane.
		if m.focus == paneContent || msg.X >= leftPaneOuterWidth {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	case omarchyTickMsg:
		// Follow the Omarchy desktop theme live.
		if theme.RefreshOmarchy() {
			m.setTheme(theme.Current())
		}
		cmds = append(cmds, omarchyTick())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpW, vpH := m.viewportSize()

		if !m.ready {
			m.viewport = viewport.New(viewport.WithWidth(vpW), viewport.WithHeight(vpH))
			m.viewport.YPosition = 4
			m.ready = true
		} else {
			m.viewport.SetWidth(vpW)
			m.viewport.SetHeight(vpH)
		}

		// Reformat content with new width
		m.rerenderContent()

	case translationsLoadedMsg:
		m.translations = msg.translations

	case booksLoadedMsg:
		// A successful load means the earlier error no longer applies.
		m.err = nil
		m.books = msg.books
		for _, book := range m.books {
			if book.BookID == m.currentBook {
				m.currentBookName = book.Name
				break
			}
		}

	case chapterLoadedMsg:
		// A successful load means the earlier error no longer applies.
		m.err = nil
		m.loading = false
		m.currentVerses = msg.verses
		m.currentParallelVerses = nil
		// Track if we came from a search (highlighted verse was set)
		cameFromSearch := m.highlightedVerseStart > 1
		// Initialize highlighted verse to first verse or use the range from search
		if m.highlightedVerseStart == 0 {
			if len(msg.verses) > 0 {
				m.highlightedVerseStart = msg.verses[0].Verse
				m.highlightedVerseEnd = m.highlightedVerseStart
			} else {
				m.highlightedVerseStart = 1
				m.highlightedVerseEnd = 1
			}
		}
		m.content = m.formatChapter(msg.verses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
		m.viewport.SetContent(m.content)

		// Remember the position now rather than only on "q": closing the
		// window (Super+W from the Omarchy launcher) ends the process
		// without a chance to save.
		if len(msg.verses) > 0 {
			_ = m.saveSettings()
		}

		// If we came from a search, scroll to the highlighted verse
		if cameFromSearch {
			m.scrollToHighlightedVerse()
			m.topVisibleVerse = m.highlightedVerseStart
		} else {
			m.viewport.GotoTop()
			m.topVisibleVerse = 0
		}

	case parallelVersesLoadedMsg:
		// A successful load means the earlier error no longer applies.
		m.err = nil
		m.loading = false
		m.currentParallelVerses = msg.verses
		m.currentVerses = nil
		m.content = m.formatParallelVerses(msg.verses, m.comparisonTranslations, m.currentBookName, m.currentChapter, m.viewport.Width())
		m.viewport.SetContent(m.content)
		m.viewport.GotoTop()

	case cacheListLoadedMsg:
		m.cachedTranslations = msg.translations

	case downloadCompleteMsg:
		m.downloadingTranslation = ""
		m.downloadProgress = 0
		if m.cache != nil {
			return m, loadCachedList(m.cache)
		}

	case downloadErrorMsg:
		m.downloadingTranslation = ""
		m.downloadProgress = 0
		m.err = msg.err

	case downloadTickMsg:
		// Poll the cache for current byte-level progress and reschedule
		// the next tick if a download is still running.
		if m.downloadingTranslation != "" && m.cache != nil {
			p, _ := m.cache.DownloadProgress()
			m.downloadProgress = p
			return m, downloadTick()
		}

	case searchResultsLoadedMsg:
		// A successful load means the earlier error no longer applies.
		m.err = nil
		m.wordSearchLoading = false
		m.wordSearchResults = msg.results
		m.wordSearchTotal = msg.total
		m.wordSearchQuery = msg.query
		m.wordSearchSelected = 0
		// Sort results by book order
		sort.Slice(m.wordSearchResults, func(i, j int) bool {
			if m.wordSearchResults[i].Book != m.wordSearchResults[j].Book {
				return m.wordSearchResults[i].Book < m.wordSearchResults[j].Book
			}
			if m.wordSearchResults[i].Chapter != m.wordSearchResults[j].Chapter {
				return m.wordSearchResults[i].Chapter < m.wordSearchResults[j].Chapter
			}
			return m.wordSearchResults[i].Verse < m.wordSearchResults[j].Verse
		})

	case errMsg:
		m.err = msg.err
		m.loading = false
		m.wordSearchLoading = false
	}

	if m.mode == modeSearch {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
		// Update word search input when typing query
		m.wordSearchInput, cmd = m.wordSearchInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.showMillerColumns && m.millerFilterMode {
		// Update Miller filter input when in filter mode
		m.millerFilterInput, cmd = m.millerFilterInput.Update(msg)
		cmds = append(cmds, cmd)

		// Apply filter when input changes
		newFilter := m.millerFilterInput.Value()
		if newFilter != m.millerFilter {
			m.millerFilter = newFilter
			m.applyMillerFilter()
			m.millerBookIdx = 0
			m.millerVerseIdx = 0
		}
	} else {
		oldYOffset := m.viewport.YOffset()
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		// Update highlighted verse + the sticky-header top-visible verse
		// based on viewport position.
		if m.currentVerses != nil && oldYOffset != m.viewport.YOffset() {
			newTopVerse := m.calculateHighlightedVerse()
			m.topVisibleVerse = newTopVerse
			if m.viewport.YOffset() == 0 {
				// Back at the top: drop the sticky indicator.
				m.topVisibleVerse = 0
			}
			if newTopVerse != m.highlightedVerseStart {
				m.highlightedVerseStart = newTopVerse
				m.highlightedVerseEnd = newTopVerse
				m.content = m.formatChapter(m.currentVerses, m.currentBookName, m.currentChapter, m.viewport.Width(), m.highlightedVerseStart, m.highlightedVerseEnd)
				m.viewport.SetContent(m.content)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// stepChapter moves delta chapters within the current book and loads it.
// It returns nil at either end of the book.
func (m *Model) stepChapter(delta int) tea.Cmd {
	next := m.currentChapter + delta
	if next < 1 {
		return nil
	}
	if delta > 0 {
		last := 0
		for _, b := range m.books {
			if b.BookID == m.currentBook {
				last = b.Chapters
				break
			}
		}
		if next > last {
			return nil
		}
	}
	m.currentChapter = next
	m.loading = true
	m.highlightedVerseStart = 0
	m.highlightedVerseEnd = 0
	return loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter)
}

// saveSettings persists the reading position and translation.
func (m Model) saveSettings() error {
	return settings.Save(settings.Settings{
		SelectedTranslation: m.selectedTranslation,
		CurrentBook:         m.currentBook,
		CurrentChapter:      m.currentChapter,
	})
}

// typingInInput reports whether key presses are currently going to a text
// input (verse lookup, word search, or the Miller-columns filter). It
// mirrors the input routing at the end of Update.
func (m Model) typingInInput() bool {
	switch {
	case m.mode == modeSearch:
		return true
	case m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading:
		return true
	case m.showMillerColumns && m.millerFilterMode:
		return true
	}
	return false
}
