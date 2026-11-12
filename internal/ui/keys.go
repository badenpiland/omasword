package ui

import (
	"errors"
	"fmt"
	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/translations"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// handleKey runs the global keyboard shortcuts. handled is false when the
// key should fall through to the focused text input or the viewport.
func (m Model) handleKey(msg tea.KeyMsg) (_ Model, _ tea.Cmd, handled bool) {
	if m.mode == modePicker {
		return m.handlePickerKey(msg)
	}
	switch msg.String() {
	case "ctrl+c", "q":
		// "q" is a letter like any other while a text input has
		// focus; only ctrl+c quits from there.
		if msg.String() == "q" && m.typingInInput() {
			break
		}
		_ = m.saveSettings()
		return m, tea.Quit, true
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
			return m, nil, true
		}
	case "]":
		if m.mode == modeReader {
			m.focus = paneContent
			return m, nil, true
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
			return m, nil, true
		}
	case "shift+tab":
		if m.mode == modeReader {
			if m.focus == paneBooks {
				m.focus = paneContent
			} else {
				m.focus = paneBooks
			}
			return m, nil, true
		}
	case "v":
		if m.mode == modeReader {
			m.openPicker()
			return m, nil, true
		}
	case "/":
		if m.mode == modeReader {
			// Close sidebar if open when entering search mode
			m.focus = paneContent
			m.mode = modeSearch
			m.searchErr = nil
			m.textInput.Focus()
			return m, nil, true
		}
	case "up", "k":
		if m.mode == modeSaved {
			m.savedSelected = max(m.savedSelected-1, 0)
			return m, nil, true
		}
		if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected > 0 {
			m.wordSearchSelected--
			return m, nil, true
		} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected > 0 {
			m.translationSelected--
			return m, nil, true
		} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected > 0 {
			m.cacheSelected--
			return m, nil, true
		} else if m.focus == paneBooks && m.sidebarSelected > 0 {
			m.sidebarSelected--
			return m, nil, true
		} else if m.mode == modeReader && m.currentVerses != nil {
			// With nothing selected, start at the top visible verse.
			if m.highlightedVerseStart == 0 {
				return m.selectTopVerse(), nil, true
			}
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
			return m, nil, true
		}
	case "down", "j":
		if m.mode == modeSaved {
			m.savedSelected = min(m.savedSelected+1, max(len(m.savedList)-1, 0))
			return m, nil, true
		}
		if m.mode == modeWordSearch && m.wordSearchResults != nil && m.wordSearchSelected < len(m.wordSearchResults)-1 {
			m.wordSearchSelected++
			return m, nil, true
		} else if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations)-1 {
			m.translationSelected++
			return m, nil, true
		} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations)-1 {
			m.cacheSelected++
			return m, nil, true
		} else if m.focus == paneBooks && m.books != nil && m.sidebarSelected < len(m.books)-1 {
			m.sidebarSelected++
			return m, nil, true
		} else if m.mode == modeReader && m.currentVerses != nil {
			if m.highlightedVerseStart == 0 {
				return m.selectTopVerse(), nil, true
			}
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
			return m, nil, true
		}
	case "+", "=", "-":
		// In comparison, + adds a column and - removes the last one.
		// The count is saved and used the next time comparison opens.
		// Only the added column is fetched; the others are already on
		// screen, and a removed column is just not drawn.
		if m.mode == modeComparison {
			n := len(m.comparisonTranslations)
			var cmd tea.Cmd
			switch {
			case msg.String() != "-" && n < translations.MaxComparison:
				name := translations.NextComparison(m.comparisonTranslations)
				m.comparisonTranslations = append(m.comparisonTranslations, name)
				cmd = m.loadComparisonColumn(name)
			case msg.String() == "-" && n > translations.MinComparison:
				m.comparisonTranslations = m.comparisonTranslations[:n-1]
			default:
				return m, nil, true
			}
			_ = m.saveSettings()
			m.rerenderContent()
			return m, cmd, true
		}
	case "c":
		// c toggles comparison: once to open it, again to go back.
		if m.mode == modeComparison {
			m, cmd := m.backToReader()
			return m, cmd, true
		}
		if m.mode == modeReader {
			// Ask for every verse of the loaded chapter; a fixed 1–31
			// cut long chapters such as Psalm 119 short.
			verses := m.comparisonVerseList()
			m.mode = modeComparison
			return m, m.loadReadingParallel(m.comparisonTranslations, m.currentBook, m.currentChapter, verses), true
		}
	case "r":
		// Don't intercept 'r' when typing in search inputs
		if m.mode == modeSearch {
			// Let it pass through to verse reference input
		} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
			// Let it pass through to word search input
		} else if m.mode != modeReader {
			m, cmd := m.backToReader()
			return m, cmd, true
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
			return m, m.checkESVKey(), true
		}
	case "d":
		if m.mode == modeReader {
			m.mode = modeCacheManager
			m.cacheSelected = 0
			if m.cache != nil {
				return m, loadCachedList(m.cache), true
			}
			return m, nil, true
		}
	case "?":
		if m.mode == modeReader {
			m.mode = modeAbout
			return m, nil, true
		}
	case "s":
		if m.mode == modeReader {
			m.mode = modeWordSearch
			m.wordSearchInput.SetValue("")
			m.wordSearchInput.Focus()
			m.wordSearchResults = nil
			m.wordSearchSelected = 0
			m.wordSearchLoading = false
			return m, nil, true
		}
	case "n", "pgdown":
		if m.mode == modeReader {
			return m, m.stepChapter(1), true
		}
	case "p", "pgup":
		if m.mode == modeReader {
			return m, m.stepChapter(-1), true
		}
	case "y":
		// Yank (copy) highlighted verse(s) or current chapter to clipboard
		if m.mode == modeReader && m.currentVerses != nil {
			var textToCopy strings.Builder

			// If verses are highlighted, only copy those
			if m.highlightedVerseStart > 0 {
				if m.highlightedVerseStart == m.highlightedVerseEnd {
					textToCopy.WriteString(fmt.Sprintf("%s %s %d:%d\n\n", m.readerTranslation(), m.currentBookName, m.currentChapter, m.highlightedVerseStart))
				} else {
					textToCopy.WriteString(fmt.Sprintf("%s %s %d:%d-%d\n\n", m.readerTranslation(), m.currentBookName, m.currentChapter, m.highlightedVerseStart, m.highlightedVerseEnd))
				}

				for _, v := range m.currentVerses {
					if v.Verse >= m.highlightedVerseStart && v.Verse <= m.highlightedVerseEnd {
						text := stripHTMLTags(v.Text)
						textToCopy.WriteString(fmt.Sprintf("%d. %s\n\n", v.Verse, text))
					}
				}
			} else {
				// Copy entire chapter
				textToCopy.WriteString(fmt.Sprintf("%s %s %d\n\n", m.readerTranslation(), m.currentBookName, m.currentChapter))

				for _, v := range m.currentVerses {
					text := stripHTMLTags(v.Text)
					textToCopy.WriteString(fmt.Sprintf("%d. %s\n\n", v.Verse, text))
				}
			}

			if m.showingESV() {
				textToCopy.WriteString("\n")
				textToCopy.WriteString(esv.Attribution(m.displayedESVNotice()))
				textToCopy.WriteString("\n")
			}
			clipboard.WriteAll(textToCopy.String())
		}
	case "enter":
		if m.mode == modeSaved {
			m, cmd := m.openSavedSelected()
			return m, cmd, true
		}
		if m.mode == modeESVKey {
			m, cmd := m.commitESVKey()
			return m, cmd, true
		}
		if m.mode == modeTranslationSelect && m.translations != nil && m.translationSelected < len(m.translations) {
			m, cmd := m.chooseTranslation(m.translations[m.translationSelected].ShortName)
			return m, cmd, true
		} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
			// Download selected translation
			translation := m.translations[m.cacheSelected].ShortName
			if translation == esv.ID {
				m.err = errors.New("ESV is online-only; it is not downloaded")
				return m, nil, true
			}
			if !translations.IsBuiltin(translation) {
				m.err = errors.New("offline download is limited to public-domain translations")
				return m, nil, true
			}
			if m.cache != nil && !m.cache.IsCached(translation) {
				m.downloadingTranslation = translation
				m.downloadProgress = 0
				return m, tea.Batch(downloadTranslation(m.cache, translation), downloadTick()), true
			}
			return m, nil, true
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
				return m, m.loadReadingChapter(m.currentBook, m.currentChapter), true
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
				m = m.withESVAnchor(m.currentBook, m.currentChapter, verseStart)
				return m, m.loadReadingChapter(m.currentBook, m.currentChapter), true
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
							m = m.withESVAnchor(m.currentBook, m.currentChapter, vs)
							return m, m.loadReadingChapter(m.currentBook, m.currentChapter), true
						}
					}
					m.wordSearchLoading = true
					m.wordSearchInput.Blur()
					return m, m.loadReadingSearch(query), true
				}
			} else if len(m.wordSearchResults) > 0 {
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
				m = m.withESVAnchor(m.currentBook, m.currentChapter, result.Verse)
				return m, m.loadReadingChapter(m.currentBook, m.currentChapter), true
			}
		} else if m.mode == modeTranslationSelect {
			// The list has not loaded yet. Cycle the public-domain
			// catalog instead of a hardcoded set that included
			// copyrighted translations.
			cycle := translations.ShortNames()
			for i, t := range cycle {
				if t == m.selectedTranslation {
					m.selectedTranslation = cycle[(i+1)%len(cycle)]
					break
				}
			}
			m.mode = modeReader
			m.loading = true
			return m, tea.Batch(
				m.loadReadingBooks(),
				m.loadReadingChapter(m.currentBook, m.currentChapter),
			), true
		}
	case "x", "delete":
		if m.mode == modeSaved {
			return m.removeSavedSelected(), nil, true
		}
		// Delete cached translation
		if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
			translation := m.translations[m.cacheSelected].ShortName
			if m.cache != nil && m.cache.IsCached(translation) {
				if err := m.cache.RemoveTranslation(translation); err == nil {
					return m, loadCachedList(m.cache), true
				}
			}
			return m, nil, true
		}
	case "a":
		// In the translation picker, "a" opens the ESV key wizard.
		// "k" is already "up", so it can't be the key shortcut.
		if m.mode == modeTranslationSelect {
			m.mode = modeESVKey
			m.esvKeyThenSelect = false
			m.esvKeyInput.SetValue("")
			m.esvKeyInput.Focus()
			m.err = nil
			return m, nil, true
		}
	case "S":
		if m.mode == modeReader {
			return m.toggleSave(), nil, true
		}
	case "J", "shift+down":
		if m.mode == modeReader {
			return m.extendSelection(1), nil, true
		}
	case "K", "shift+up":
		if m.mode == modeReader {
			return m.extendSelection(-1), nil, true
		}
	case "o":
		if m.mode == modeReader {
			return m.openSavedList(), nil, true
		}
		if m.mode == modeSaved {
			m.mode = modeReader
			return m, nil, true
		}
	case ",":
		if m.mode == modeReader && m.showingESV() && m.esvFirst > 1 {
			m.esvPageBook, m.esvPageChapter = m.currentBook, m.currentChapter
			m.esvPageStart = esv.PageStart(m.currentBook, m.esvFirst-1)
			m.loading = true
			return m, m.loadReadingChapter(m.currentBook, m.currentChapter), true
		}
	case ".":
		if m.mode == modeReader && m.showingESV() && m.esvTotal > m.esvLast && m.esvLast > 0 {
			m.esvPageBook, m.esvPageChapter = m.currentBook, m.currentChapter
			m.esvPageStart = m.esvLast + 1
			m.loading = true
			return m, m.loadReadingChapter(m.currentBook, m.currentChapter), true
		}
	case "X":
		// Remove every downloaded translation that is no longer offered.
		if m.mode == modeCacheManager && m.cache != nil {
			leftovers := m.leftoverDownloads()
			if len(leftovers) == 0 {
				return m, nil, true
			}
			for _, name := range leftovers {
				if err := m.cache.RemoveTranslation(name); err != nil {
					m.err = err
					break
				}
			}
			return m, loadCachedList(m.cache), true
		}
	case "esc":
		if m.mode == modeESVKey {
			m.esvKeyThenSelect = false
			m.esvKeyInput.SetValue("")
			m.mode = modeTranslationSelect
			return m, nil, true
		}
		if m.mode == modeCacheManager || m.mode == modeSaved {
			m.mode = modeReader
			return m, nil, true
		}
		if m.mode == modeSearch || m.mode == modeTranslationSelect || m.mode == modeAbout || m.mode == modeComparison || m.mode == modeWordSearch || m.mode == modeCacheManager {
			// Picker was opened from a comparison column: dismiss
			// it back into comparison view instead of dropping all
			// the way down to the reader.
			if m.mode == modeTranslationSelect && m.comparisonPickerColumn >= 0 {
				m.comparisonPickerColumn = -1
				m.mode = modeComparison
				return m, nil, true
			}
			m.wordSearchResults = nil
			m.wordSearchInput.SetValue("")
			m, cmd := m.backToReader()
			return m, cmd, true
		}
		if m.focus == paneBooks {
			m.focus = paneContent
			return m, nil, true
		}
		if m.mode == modeReader && m.highlightedVerseStart > 0 {
			return m.clearSelection(), nil, true
		}
	}
	return m, nil, false
}

// backToReader returns to the reader. Showing a comparison replaces the
// reading pane and drops the loaded chapter, so leaving one (or a
// translation picker opened from one of its columns) reloads the chapter.
func (m Model) backToReader() (Model, tea.Cmd) {
	fromComparison := m.mode == modeComparison || m.comparisonPickerColumn >= 0
	m.mode = modeReader
	m.comparisonPickerColumn = -1
	if !fromComparison {
		return m, nil
	}
	m.loading = true
	return m, m.loadReadingChapter(m.currentBook, m.currentChapter)
}
