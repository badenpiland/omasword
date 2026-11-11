package ui

import (
	"strings"

	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/translations"

	tea "charm.land/bubbletea/v2"
)

// loadReadingChapter loads the chapter on screen. ESV comes from the
// user's Crossway key; everything else still comes from bolls.life.
func (m Model) loadReadingChapter(book, chapter int) tea.Cmd {
	if m.selectedTranslation != esv.ID {
		return loadChapter(m.client, m.selectedTranslation, book, chapter)
	}
	start := 1
	if m.esvPageBook == book && m.esvPageChapter == chapter && m.esvPageStart > 1 {
		start = m.esvPageStart
	}
	client := m.esvClient
	return func() tea.Msg {
		if client == nil || !client.HasKey() {
			return chapterFailedMsg{book: book, chapter: chapter, err: esv.ErrNoKey}
		}
		page, err := client.Chapter(book, chapter, start)
		if err != nil {
			return chapterFailedMsg{book: book, chapter: chapter, err: err}
		}
		return chapterLoadedMsg{
			verses:    page.Verses,
			esvFirst:  page.First,
			esvLast:   page.Last,
			esvTotal:  page.Total,
			esvNotice: page.Notice,
		}
	}
}

func (m Model) loadReadingBooks() tea.Cmd {
	if m.selectedTranslation == esv.ID {
		return func() tea.Msg { return booksLoadedMsg{esv.Books()} }
	}
	return loadBooks(m.client, m.selectedTranslation)
}

func (m Model) loadReadingSearch(query string) tea.Cmd {
	if m.selectedTranslation != esv.ID {
		return loadSearchResults(m.client, m.selectedTranslation, query)
	}
	client := m.esvClient
	return func() tea.Msg {
		if client == nil || !client.HasKey() {
			return errMsg{esv.ErrNoKey}
		}
		resp, err := client.Search(query)
		if err != nil {
			return errMsg{err}
		}
		return searchResultsLoadedMsg{results: resp.Results, total: resp.Total, query: query}
	}
}

// loadReadingParallel uses the bolls parallel endpoint unless a column is
// ESV, which must not be requested from bolls.life. That column is fetched
// from Crossway and clipped to the per-page cap.
func (m Model) loadReadingParallel(names []string, book, chapter int, verses []int) tea.Cmd {
	for _, name := range names {
		if name == esv.ID {
			return m.loadSplitParallel(names, book, chapter, verses)
		}
	}
	return loadParallelVerses(m.client, names, book, chapter, verses)
}

func (m Model) loadSplitParallel(names []string, book, chapter int, verses []int) tea.Cmd {
	client := m.client
	esvClient := m.esvClient
	return func() tea.Msg {
		out := make(map[string][]api.Verse, len(names))
		var first error
		for _, name := range names {
			var got []api.Verse
			var err error
			if name == esv.ID {
				if esvClient == nil || !esvClient.HasKey() {
					err = esv.ErrNoKey
				} else {
					var page esv.Passage
					page, err = esvClient.Chapter(book, chapter, 1)
					got = page.Verses
				}
			} else {
				got, err = client.GetChapter(name, book, chapter)
			}
			if err != nil {
				if first == nil {
					first = err
				}
				continue
			}
			out[name] = filterVerseNumbers(got, verses)
		}
		if len(out) == 0 && first != nil {
			return errMsg{first}
		}
		return parallelVersesLoadedMsg{out}
	}
}

func filterVerseNumbers(verses []api.Verse, want []int) []api.Verse {
	if len(want) == 0 {
		return verses
	}
	allow := make(map[int]bool, len(want))
	for _, n := range want {
		allow[n] = true
	}
	out := make([]api.Verse, 0, len(verses))
	for _, v := range verses {
		if allow[v.Verse] {
			out = append(out, v)
		}
	}
	return out
}

func (m Model) loadPickerChapter(book, chapter int) tea.Cmd {
	if m.selectedTranslation != esv.ID {
		return loadPickerVerses(m.client, m.selectedTranslation, book, chapter)
	}
	client := m.esvClient
	return func() tea.Msg {
		if client == nil || !client.HasKey() {
			return pickerVersesMsg{book: book, chapter: chapter, err: esv.ErrNoKey}
		}
		page, err := client.Chapter(book, chapter, 1)
		return pickerVersesMsg{book: book, chapter: chapter, verses: page.Verses, err: err}
	}
}

// chooseTranslation switches the reader, or a comparison column, to name.
// ESV without a key opens the key wizard instead of calling an API.
func (m Model) chooseTranslation(name string) (Model, tea.Cmd) {
	if name == esv.ID && (m.esvClient == nil || !m.esvClient.HasKey()) {
		m.mode = modeESVKey
		m.esvKeyThenSelect = true
		m.esvKeyInput.SetValue("")
		m.esvKeyInput.Focus()
		return m, nil
	}
	m.esvPageStart = 1
	if m.comparisonPickerColumn >= 0 && m.comparisonPickerColumn < len(m.comparisonTranslations) {
		m.comparisonTranslations[m.comparisonPickerColumn] = name
		m.comparisonPickerColumn = -1
		m.mode = modeComparison
		m.loading = true
		return m, m.loadReadingParallel(m.comparisonTranslations, m.currentBook, m.currentChapter, m.comparisonVerseList())
	}
	m.selectedTranslation = name
	m.mode = modeReader
	m.loading = true
	return m, tea.Batch(
		m.loadReadingBooks(),
		m.loadReadingChapter(m.currentBook, m.currentChapter),
	)
}

// commitESVKey stores the wizard's input. An empty input removes the key
// from config. The environment variable is never written into the file.
func (m Model) commitESVKey() (Model, tea.Cmd) {
	raw := strings.TrimSpace(m.esvKeyInput.Value())
	m.esvConfigKey = raw
	effective := esv.EffectiveKey(raw)
	if m.esvClient == nil {
		m.esvClient, _ = esv.New(effective, esv.DefaultCacheDir())
	} else {
		m.esvClient.SetKey(effective)
	}
	_ = m.saveSettings()
	m.esvKeyInput.SetValue("")
	then := m.esvKeyThenSelect
	m.esvKeyThenSelect = false
	if then && m.esvClient != nil && m.esvClient.HasKey() {
		return m.chooseTranslation(esv.ID)
	}
	if m.selectedTranslation == esv.ID && (m.esvClient == nil || !m.esvClient.HasKey()) {
		m.selectedTranslation = translations.Default
		m.esvPageStart = 1
		m.mode = modeReader
		m.loading = true
		return m, tea.Batch(m.loadReadingBooks(), m.loadReadingChapter(m.currentBook, m.currentChapter))
	}
	m.mode = modeTranslationSelect
	return m, nil
}

func (m Model) translationFlavor(name string) string {
	if name == esv.ID {
		if m.esvClient != nil && m.esvClient.HasKey() {
			return esv.FlavorReady
		}
		return esv.FlavorNeedsKey
	}
	return translations.LookupFlavor(name)
}

func (m Model) displayedESVNotice() string {
	if m.esvNotice != "" {
		return m.esvNotice
	}
	if m.esvClient != nil {
		return m.esvClient.Notice()
	}
	return esv.CopyrightNotice
}

func (m Model) withESVAnchor(book, chapter, verse int) Model {
	if m.selectedTranslation == esv.ID && verse > 1 {
		m.esvPageBook, m.esvPageChapter = book, chapter
		m.esvPageStart = esv.PageStart(book, verse)
		return m
	}
	m.esvPageStart = 1
	return m
}
