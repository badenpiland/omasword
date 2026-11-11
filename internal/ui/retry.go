package ui

import (
	"errors"
	"time"

	"omasword/internal/esv"

	tea "charm.land/bubbletea/v2"
)

// The book and translation lists load once at startup, and a chapter
// loads each time you move. If bolls.life refuses one (rate limits, an
// outage, no network yet), the app keeps retrying in the background
// instead of showing "Loading…" or a blank page forever.

type (
	booksFailedMsg        struct{ err error }
	translationsFailedMsg struct{ err error }
	retryBooksMsg         struct{}
	retryTranslationsMsg  struct{}
	chapterFailedMsg      struct {
		book, chapter int
		err           error
	}
	retryChapterMsg struct{ book, chapter int }
)

// listRetryDelay backs off retries of a list: 5s, 10s, 20s, then every 30s.
func listRetryDelay(attempt int) time.Duration {
	return min(5*time.Second<<min(attempt, 3), 30*time.Second)
}

func retryAfter(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

// handleListLoad updates list-loading state for msg and returns the
// follow-up command. ok is false for any other message.
func (m *Model) handleListLoad(msg tea.Msg) (cmd tea.Cmd, ok bool) {
	switch msg := msg.(type) {
	case booksFailedMsg:
		m.booksErr = msg.err
		cmd = retryAfter(listRetryDelay(m.booksRetries), retryBooksMsg{})
		m.booksRetries++
	case retryBooksMsg:
		cmd = m.loadReadingBooks()
	case translationsFailedMsg:
		m.translationsErr = msg.err
		cmd = retryAfter(listRetryDelay(m.translationsRetries), retryTranslationsMsg{})
		m.translationsRetries++
	case retryTranslationsMsg:
		cmd = loadTranslations(m.client)
	case chapterFailedMsg:
		m.err = msg.err
		m.loading = false
		// Keep retrying only while that chapter is still the one you're
		// on, and only when trying again can help.
		if msg.book == m.currentBook && msg.chapter == m.currentChapter && retryableChapterErr(msg.err) {
			delay := listRetryDelay(m.chapterRetries)
			var rate *esv.RateError
			if errors.As(msg.err, &rate) {
				delay = max(delay, rate.RetryAfter)
			}
			cmd = retryAfter(delay, retryChapterMsg{msg.book, msg.chapter})
			m.chapterRetries++
		}
	case retryChapterMsg:
		if msg.book == m.currentBook && msg.chapter == m.currentChapter {
			m.loading = true
			cmd = m.loadReadingChapter(msg.book, msg.chapter)
		}
	default:
		return nil, false
	}
	return cmd, true
}

// retryableChapterErr is false for a missing or rejected ESV key: the
// same request would fail again, and a rejected key would keep calling
// Crossway every 30 seconds.
func retryableChapterErr(err error) bool {
	return !errors.Is(err, esv.ErrNoKey) && !errors.Is(err, esv.ErrKeyRejected)
}
