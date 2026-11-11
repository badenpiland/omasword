package ui

// The "v" picker: a floating panel with three columns (Books → Chapters →
// Verses) for browsing to a passage without typing a reference.

import (
	"fmt"
	"strconv"
	"strings"

	"omasword/internal/api"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	pickerColBooks = iota
	pickerColChapters
	pickerColVerses
)

// pickerVersesMsg carries the verses of the chapter selected in the
// picker. They are kept apart from the reader's chapter so browsing never
// changes what the reader shows until a verse is opened.
type pickerVersesMsg struct {
	book, chapter int
	verses        []api.Verse
	err           error
}

func loadPickerVerses(client *api.Client, translation string, book, chapter int) tea.Cmd {
	return func() tea.Msg {
		verses, err := client.GetChapter(translation, book, chapter)
		return pickerVersesMsg{book: book, chapter: chapter, verses: verses, err: err}
	}
}

// openPicker shows the picker with the reader's current position selected.
func (m *Model) openPicker() {
	m.mode = modePicker
	m.pickerColumn = pickerColBooks
	m.pickerFiltering = false
	m.pickerFilterInput.SetValue("")
	m.pickerFilterInput.Blur()
	m.pickerBook = m.currentBook
	m.pickerChapter = m.currentChapter
	// The open chapter's verses are already loaded.
	m.pickerVerses = m.currentVerses
	m.pickerVersesBook, m.pickerVersesChapter = m.currentBook, m.currentChapter
	m.pickerVersesLoading = false
	m.pickerVersesErr = nil
	m.pickerVerse = m.highlightedVerseStart
}

func (m Model) pickerFilter() string {
	return strings.ToLower(strings.TrimSpace(m.pickerFilterInput.Value()))
}

func (m Model) pickerSelectedBook() (api.Book, bool) {
	for _, b := range m.books {
		if b.BookID == m.pickerBook {
			return b, true
		}
	}
	return api.Book{}, false
}

// pickerBookList is the Books column, narrowed by the filter while that
// column is active.
func (m Model) pickerBookList() []api.Book {
	f := m.pickerFilter()
	if m.pickerColumn != pickerColBooks || f == "" {
		return m.books
	}
	var out []api.Book
	for _, b := range m.books {
		if strings.Contains(strings.ToLower(b.Name), f) {
			out = append(out, b)
		}
	}
	return out
}

// pickerVersesReady reports whether pickerVerses holds the selected chapter.
func (m Model) pickerVersesReady() bool {
	return m.pickerVerses != nil && m.pickerVersesBook == m.pickerBook && m.pickerVersesChapter == m.pickerChapter
}

// pickerVerseList is the Verses column, narrowed by the filter (verse text
// or number) while that column is active.
func (m Model) pickerVerseList() []api.Verse {
	if !m.pickerVersesReady() {
		return nil
	}
	f := m.pickerFilter()
	if m.pickerColumn != pickerColVerses || f == "" {
		return m.pickerVerses
	}
	var out []api.Verse
	for _, v := range m.pickerVerses {
		if strconv.Itoa(v.Verse) == f || strings.Contains(strings.ToLower(stripHTMLTags(v.Text)), f) {
			out = append(out, v)
		}
	}
	return out
}

// pickerMove moves the selection in the active column by delta, clamped
// to the list.
func (m *Model) pickerMove(delta int) {
	switch m.pickerColumn {
	case pickerColBooks:
		books := m.pickerBookList()
		if len(books) == 0 {
			return
		}
		i := clamp(indexOfBook(books, m.pickerBook)+delta, 0, len(books)-1)
		if books[i].BookID != m.pickerBook {
			m.pickerBook = books[i].BookID
			m.pickerChapter = 1
		}
	case pickerColChapters:
		if b, ok := m.pickerSelectedBook(); ok {
			m.pickerChapter = clamp(m.pickerChapter+delta, 1, max(b.Chapters, 1))
		}
	case pickerColVerses:
		verses := m.pickerVerseList()
		if len(verses) == 0 {
			return
		}
		i := clamp(indexOfVerse(verses, m.pickerVerse)+delta, 0, len(verses)-1)
		m.pickerVerse = verses[i].Verse
	}
}

// pickerFilterChanged keeps the selection on a visible item after the
// filter text changes.
func (m *Model) pickerFilterChanged() {
	switch m.pickerColumn {
	case pickerColBooks:
		books := m.pickerBookList()
		if len(books) > 0 && indexOfBook(books, m.pickerBook) < 0 {
			m.pickerBook = books[0].BookID
			m.pickerChapter = 1
		}
	case pickerColVerses:
		verses := m.pickerVerseList()
		if len(verses) > 0 && indexOfVerse(verses, m.pickerVerse) < 0 {
			m.pickerVerse = verses[0].Verse
		}
	}
}

func (m *Model) pickerClearFilter() {
	m.pickerFiltering = false
	m.pickerFilterInput.SetValue("")
	m.pickerFilterInput.Blur()
}

// pickerRight moves one column right, loading the chapter's verses on
// arrival in the Verses column.
func (m *Model) pickerRight() tea.Cmd {
	if _, ok := m.pickerSelectedBook(); !ok || m.pickerColumn == pickerColVerses {
		return nil
	}
	m.pickerClearFilter()
	m.pickerColumn++
	if m.pickerColumn != pickerColVerses || m.pickerVersesReady() {
		return nil
	}
	m.pickerVerses = nil
	m.pickerVersesErr = nil
	m.pickerVersesLoading = true
	m.pickerVersesBook, m.pickerVersesChapter = m.pickerBook, m.pickerChapter
	return loadPickerVerses(m.client, m.selectedTranslation, m.pickerBook, m.pickerChapter)
}

func (m *Model) pickerVersesLoaded(msg pickerVersesMsg) {
	// Ignore a reply for a chapter that is no longer selected.
	if msg.book != m.pickerVersesBook || msg.chapter != m.pickerVersesChapter {
		return
	}
	m.pickerVersesLoading = false
	m.pickerVersesErr = msg.err
	m.pickerVerses = msg.verses
	if len(msg.verses) > 0 && indexOfVerse(msg.verses, m.pickerVerse) < 0 {
		m.pickerVerse = msg.verses[0].Verse
	}
}

// pickerOpen opens the selected verse in the reader, reusing the verses
// the picker already loaded.
func (m Model) pickerOpen() (Model, tea.Cmd) {
	b, ok := m.pickerSelectedBook()
	verses := m.pickerVerseList()
	if !ok || len(verses) == 0 {
		return m, nil
	}
	verse := verses[clamp(indexOfVerse(verses, m.pickerVerse), 0, len(verses)-1)].Verse
	all := m.pickerVerses

	m.mode = modeReader
	m.pickerClearFilter()
	m.focus = paneContent
	m.currentBook = b.BookID
	m.currentBookName = b.Name
	m.currentChapter = m.pickerChapter
	m.highlightedVerseStart = verse
	m.highlightedVerseEnd = verse
	if i := indexOfBook(m.books, b.BookID); i >= 0 {
		m.sidebarSelected = i
	}
	m.loading = true
	return m, func() tea.Msg { return chapterLoadedMsg{all} }
}

// handlePickerKey owns the keyboard while the picker is open, so reader
// shortcuts (n, p, c, t, …) can't act behind it. handled is false only
// for keys meant for the filter input.
func (m Model) handlePickerKey(msg tea.KeyMsg) (_ Model, _ tea.Cmd, handled bool) {
	key := msg.String()
	if key == "ctrl+c" {
		_ = m.saveSettings()
		return m, tea.Quit, true
	}

	if m.pickerFiltering {
		switch key {
		case "esc":
			m.pickerClearFilter()
		case "enter":
			m.pickerFiltering = false
			m.pickerFilterInput.Blur()
		case "up", "down":
			m.pickerMove(map[string]int{"up": -1, "down": 1}[key])
		default:
			return m, nil, false
		}
		return m, nil, true
	}

	switch key {
	case "q":
		_ = m.saveSettings()
		return m, tea.Quit, true
	case "esc", "v":
		if m.pickerFilter() != "" {
			m.pickerClearFilter()
		} else {
			m.mode = modeReader
		}
	case "up", "k":
		m.pickerMove(-1)
	case "down", "j":
		m.pickerMove(1)
	case "pgup":
		m.pickerMove(-m.pickerRows())
	case "pgdown":
		m.pickerMove(m.pickerRows())
	case "left", "h":
		if m.pickerColumn > pickerColBooks {
			m.pickerClearFilter()
			m.pickerColumn--
		}
	case "right", "l":
		return m, m.pickerRight(), true
	case "enter":
		if m.pickerColumn == pickerColVerses {
			m, cmd := m.pickerOpen()
			return m, cmd, true
		}
		return m, m.pickerRight(), true
	case "/":
		if m.pickerColumn != pickerColChapters {
			m.pickerFiltering = true
			m.pickerFilterInput.SetStyles(m.themedInputStyles())
			m.pickerFilterInput.Focus()
		}
	}
	return m, nil, true
}

// pickerRows is how many list rows each column shows. The panel adds 10
// rows of chrome; the margin keeps the header and status bar visible.
func (m Model) pickerRows() int {
	return clamp(m.height-17, 3, 20)
}

func (m Model) renderPicker() string {
	t := m.currentTheme
	bg := t.Background

	// Three equal columns with two-cell gaps, at most 28 cells each.
	// Fit the right pane when there's room (like the other panels),
	// otherwise the whole window.
	const gap, chrome = 2, 2 + 2 + 4 // gaps; margin, border, padding
	colW := min((m.width-leftPaneOuterWidth-chrome-2*gap)/3, 28)
	if colW < 16 {
		colW = (m.width - chrome - 2*gap) / 3
	}
	innerW := 3*colW + 2*gap
	rows := m.pickerRows()

	base := lipgloss.NewStyle().Background(bg)
	title := base.Foreground(t.Accent).Bold(true).Render("Browse")
	sub := base.Foreground(t.Muted).Render("  " + m.selectedTranslation)

	type item struct {
		label    string
		selected bool
	}
	column := func(col int, name string, items []item, empty string) string {
		active := m.pickerColumn == col
		headStyle := base.Foreground(t.Muted).Bold(true)
		ruleStyle := base.Foreground(t.Border)
		if active {
			headStyle = headStyle.Foreground(t.Accent)
			ruleStyle = ruleStyle.Foreground(t.BorderActive)
		}
		sel := 0
		for i, it := range items {
			if it.selected {
				sel = i
			}
		}
		count := ""
		if len(items) > 0 {
			count = fmt.Sprintf("%d/%d", sel+1, len(items))
		}
		head := name + strings.Repeat(" ", max(colW-len(name)-len(count), 1)) + count

		lines := []string{
			headStyle.Width(colW).Render(ansi.Truncate(head, colW, "")),
			ruleStyle.Render(strings.Repeat("─", colW)),
		}
		start := clamp(sel-rows/2, 0, max(len(items)-rows, 0))
		for i := start; i < start+rows; i++ {
			if i >= len(items) {
				text := ""
				if i == 0 {
					text = empty
				}
				lines = append(lines, base.Foreground(t.Muted).Italic(true).Width(colW).Render(ansi.Truncate(text, colW, "…")))
				continue
			}
			it := items[i]
			style := base.Foreground(t.Primary)
			prefix := "  "
			switch {
			case it.selected && active:
				style = lipgloss.NewStyle().Foreground(bg).Background(t.Accent).Bold(true)
				prefix = "▸ "
			case it.selected:
				style = base.Foreground(t.Accent).Bold(true)
				prefix = "▸ "
			}
			lines = append(lines, style.Width(colW).Render(ansi.Truncate(prefix+it.label, colW, "…")))
		}
		return strings.Join(lines, "\n")
	}

	var books []item
	for _, b := range m.pickerBookList() {
		books = append(books, item{b.Name, b.BookID == m.pickerBook})
	}
	var chapters []item
	if b, ok := m.pickerSelectedBook(); ok {
		for c := 1; c <= b.Chapters; c++ {
			chapters = append(chapters, item{fmt.Sprintf("Chapter %d", c), c == m.pickerChapter})
		}
	}
	var verses []item
	for _, v := range m.pickerVerseList() {
		text := strings.Join(strings.Fields(stripHTMLTags(v.Text)), " ")
		verses = append(verses, item{fmt.Sprintf("%d %s", v.Verse, text), v.Verse == m.pickerVerse})
	}

	booksEmpty := "Loading…"
	if m.books != nil {
		booksEmpty = "No match"
	}
	versesEmpty := "→ to show verses"
	switch {
	case m.pickerVersesLoading && m.pickerVersesBook == m.pickerBook && m.pickerVersesChapter == m.pickerChapter:
		versesEmpty = "Loading…"
	case m.pickerVersesErr != nil && m.pickerVersesBook == m.pickerBook && m.pickerVersesChapter == m.pickerChapter:
		versesEmpty = "Couldn't load verses"
	case m.pickerVersesReady():
		versesEmpty = "No match"
	}

	spacer := base.Width(gap).Height(rows + 2).Render("")
	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		column(pickerColBooks, "BOOKS", books, booksEmpty), spacer,
		column(pickerColChapters, "CHAPTERS", chapters, ""), spacer,
		column(pickerColVerses, "VERSES", verses, versesEmpty),
	)

	var footer string
	switch {
	case m.pickerFiltering:
		footer = base.Foreground(t.Accent).Render("/ ") + m.pickerFilterInput.View()
	case m.pickerFilter() != "":
		footer = base.Foreground(t.Warning).Render("filter: "+m.pickerFilterInput.Value()) +
			base.Foreground(t.Muted).Render("  (esc clears)")
	default:
		footer = base.Foreground(t.Muted).Render("/ filter · ⏎ open · esc close")
	}
	footer = base.Width(innerW).Render(ansi.Truncate(footer, innerW, "…"))

	content := lipgloss.JoinVertical(lipgloss.Left,
		base.Width(innerW).Render(title+sub),
		base.Width(innerW).Render(""),
		columns,
		base.Width(innerW).Render(""),
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		BorderBackground(bg).
		Background(bg).
		Padding(1, 2).
		Render(content)
}

func indexOfBook(books []api.Book, id int) int {
	for i, b := range books {
		if b.BookID == id {
			return i
		}
	}
	return -1
}

func indexOfVerse(verses []api.Verse, n int) int {
	for i, v := range verses {
		if v.Verse == n {
			return i
		}
	}
	return -1
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
