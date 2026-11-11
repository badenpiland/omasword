package ui

import (
	"fmt"
	"omasword/internal/api"
	"omasword/internal/reference"
)

func fuzzyMatchBook(query string, books []api.Book) (int, string, bool) {
	return reference.MatchBook(query, books)
}

func parseReference(ref string, books []api.Book) (book, chapter, verseStart, verseEnd int, err error) {
	return reference.Parse(ref, books)
}

func validateReference(books []api.Book, bookID, chapter, verseStart, verseEnd int) (api.Book, int, int, int, error) {
	return reference.Validate(books, bookID, chapter, verseStart, verseEnd)
}

func (m Model) getBookName(bookID int) string {
	if m.books != nil {
		for _, book := range m.books {
			if book.BookID == bookID {
				return book.Name
			}
		}
	}
	return fmt.Sprintf("Book %d", bookID)
}
