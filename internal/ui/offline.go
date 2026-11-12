package ui

import (
	"errors"
	"fmt"

	"omasword/internal/esv"

	tea "charm.land/bubbletea/v2"
)

// ESV offline: save ahead (S), pin chapters (P), and the BSB in place of
// an ESV chapter that can't be reached. All ESV storage goes through
// esv.VerseCache, which holds Crossway's 500-verse and half-book caps;
// nothing here stores ESV text itself.

// saveAheadDoneMsg reports a finished save-ahead run.
type saveAheadDoneMsg struct {
	res esv.SaveResult
	err error
}

// showingESV is true when the reading pane holds ESV text, so the
// Crossway notice and ESV paging apply. It is false while the BSB stands
// in for an ESV chapter.
func (m Model) showingESV() bool {
	return m.selectedTranslation == esv.ID && m.fallback == ""
}

// readerTranslation names the translation on screen, which is the BSB
// rather than the selected ESV while the fallback is shown.
func (m Model) readerTranslation() string {
	if m.fallback != "" {
		return m.fallback
	}
	return m.selectedTranslation
}

// fallbackStatus is the short status-bar text while the BSB stands in.
func (m Model) fallbackStatus() string {
	var rate *esv.RateError
	if errors.As(m.fallbackErr, &rate) {
		return "ESV rate limit · " + m.fallback + " shown"
	}
	return "ESV offline · " + m.fallback + " shown"
}

// fallbackNote is the line under a stand-in chapter.
func (m Model) fallbackNote() string {
	return fmt.Sprintf("The ESV for this chapter isn't saved and can't be reached right now, so this is the %s. The ESV comes back on its own once it can be loaded. Press S while online to save chapters ahead.", m.fallback)
}

// togglePin pins or unpins the ESV chapter on screen.
func (m Model) togglePin() Model {
	switch {
	case !m.showingESV():
		m.status = "Pins keep ESV chapters offline; other texts download whole with d"
		return m
	case m.esvClient == nil || !m.esvSaved:
		m.status = "This page isn't saved, so it can't be pinned"
		return m
	}
	pin := !m.esvPinned
	found, err := m.esvClient.SetPinned(m.currentBook, m.currentChapter, pin)
	if err != nil {
		m.status = "Couldn't save the pin: " + err.Error()
		return m
	}
	if !found {
		m.esvSaved, m.esvPinned = false, false
		m.status = "This page isn't saved, so it can't be pinned"
		return m
	}
	m.esvPinned = pin
	s := m.esvClient.Stats()
	verb := "Pinned"
	if !pin {
		verb = "Unpinned"
	}
	m.status = fmt.Sprintf("%s %s %d · %d pinned, %d/%d verses saved",
		verb, m.currentBookName, m.currentChapter, s.PinnedChapters, s.Verses, esv.MaxStoredVerses)
	return m
}

// saveAhead starts saving the chapters after the one on screen.
func (m Model) saveAhead() (Model, tea.Cmd) {
	switch {
	case m.selectedTranslation != esv.ID:
		m.status = "Save ahead is for the ESV; other texts download whole with d"
		return m, nil
	case m.esvClient == nil || !m.esvClient.HasKey():
		m.status = "Save ahead needs your ESV key (t, then a)"
		return m, nil
	case m.fallback != "":
		m.status = "Can't reach the ESV; connect, then press S"
		return m, nil
	case m.savingAhead:
		return m, nil
	}
	m.savingAhead = true
	m.status = "Saving ahead…"
	client, book, chapter := m.esvClient, m.currentBook, m.currentChapter
	return m, func() tea.Msg {
		res, err := client.SaveAhead(book, chapter)
		return saveAheadDoneMsg{res, err}
	}
}

// saveAheadStatus describes a finished run.
func saveAheadStatus(res esv.SaveResult, err error) string {
	var msg string
	if res.ThroughBook > 0 {
		name, _ := esv.BookName(res.ThroughBook)
		msg = fmt.Sprintf("Saved through %s %d", name, res.ThroughChapter)
		if res.Fetched > 0 {
			msg += fmt.Sprintf(" (%d new)", res.Fetched)
		}
	} else {
		msg = "Nothing new saved"
	}
	switch {
	case err != nil:
		msg += " · stopped: " + err.Error()
	case res.Full && res.BookLimit:
		name, _ := esv.BookName(res.FullBook)
		msg += " · Crossway allows under half of " + name
	case res.Full:
		msg += " · offline space (500 verses) is full"
	}
	return msg
}
