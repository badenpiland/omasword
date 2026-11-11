package ui

import (
	"fmt"
	"omasword/internal/api"
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
	case "c":
		if m.mode == modeReader {
			m.mode = modeComparison
			verses := []int{}
			for i := 1; i <= 31; i++ {
				verses = append(verses, i)
			}
			return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, verses), true
		}
	case "r":
		// Don't intercept 'r' when typing in search inputs
		if m.mode == modeSearch {
			// Let it pass through to verse reference input
		} else if m.mode == modeWordSearch && m.wordSearchResults == nil && !m.wordSearchLoading {
			// Let it pass through to word search input
		} else if m.mode != modeReader {
			m.mode = modeReader
			return m, nil, true
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
			return m, nil, true
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
				return m, loadParallelVerses(m.client, m.comparisonTranslations, m.currentBook, m.currentChapter, m.comparisonVerseList()), true
			}
			m.selectedTranslation = newTrans
			m.mode = modeReader
			m.loading = true
			return m, tea.Batch(
				loadBooks(m.client, m.selectedTranslation),
				loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
			), true
		} else if m.mode == modeCacheManager && m.translations != nil && m.cacheSelected < len(m.translations) {
			// Download selected translation
			translation := m.translations[m.cacheSelected].ShortName
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
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter), true
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
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter), true
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
							return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter), true
						}
					}
					m.wordSearchLoading = true
					m.wordSearchInput.Blur()
					return m, loadSearchResults(m.client, m.selectedTranslation, query), true
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
				return m, loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter), true
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
			), true
		}
	case "x":
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
	case "esc":
		if m.mode == modeCacheManager {
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
			m.mode = modeReader
			m.wordSearchResults = nil
			m.wordSearchInput.SetValue("")
			return m, nil, true
		}
		if m.focus == paneBooks {
			m.focus = paneContent
			return m, nil, true
		}
	}
	return m, nil, false
}
