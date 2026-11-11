package ui

import (
	"omasword/internal/api"
	"omasword/internal/theme"
	"time"

	tea "charm.land/bubbletea/v2"
)

func downloadTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return downloadTickMsg{}
	})
}

// omarchyPollInterval is how often we check whether the Omarchy desktop
// theme changed (e.g. via `omarchy theme set`).
const omarchyPollInterval = time.Second

type omarchyTickMsg struct{}

// omarchyTick schedules the next Omarchy theme check. It is a no-op on
// systems without Omarchy.
func omarchyTick() tea.Cmd {
	if theme.OmarchyThemeDir() == "" {
		return nil
	}
	return tea.Tick(omarchyPollInterval, func(time.Time) tea.Msg {
		return omarchyTickMsg{}
	})
}

func loadTranslations(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		translations, err := client.GetTranslations()
		if err != nil {
			return errMsg{err}
		}
		return translationsLoadedMsg{translations}
	}
}

func loadBooks(client *api.Client, translation string) tea.Cmd {
	return func() tea.Msg {
		books, err := client.GetBooks(translation)
		if err != nil {
			return errMsg{err}
		}
		return booksLoadedMsg{books}
	}
}

func loadChapter(client *api.Client, translation string, book, chapter int) tea.Cmd {
	return func() tea.Msg {
		verses, err := client.GetChapter(translation, book, chapter)
		if err != nil {
			return errMsg{err}
		}
		return chapterLoadedMsg{verses}
	}
}

func loadParallelVerses(client *api.Client, translations []string, book, chapter int, verses []int) tea.Cmd {
	return func() tea.Msg {
		req := api.ParallelVerseRequest{
			Translations: translations,
			Verses:       verses,
			Chapter:      chapter,
			Book:         book,
		}
		result, err := client.GetParallelVerses(req)
		if err != nil {
			return errMsg{err}
		}
		return parallelVersesLoadedMsg{result}
	}
}

func loadCachedList(cache CacheInterface) tea.Cmd {
	return func() tea.Msg {
		translations, err := cache.ListCached()
		if err != nil {
			return errMsg{err}
		}
		return cacheListLoadedMsg{translations}
	}
}

func downloadTranslation(cache CacheInterface, translation string) tea.Cmd {
	return func() tea.Msg {
		err := cache.DownloadTranslation(translation)
		if err != nil {
			return downloadErrorMsg{translation, err}
		}
		return downloadCompleteMsg{translation}
	}
}

func loadSearchResults(client *api.Client, translation, query string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.SearchVerses(translation, query)
		if err != nil {
			return errMsg{err}
		}
		return searchResultsLoadedMsg{
			results: resp.Results,
			total:   resp.Total,
			query:   query,
		}
	}
}
