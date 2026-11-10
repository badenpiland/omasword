package ui

import (
	"fmt"
	"omasword/internal/api"
	"strconv"
	"strings"
)

// fuzzyMatchBook attempts to match a book name or abbreviation to a book ID
func fuzzyMatchBook(query string, books []api.Book) (int, string, bool) {
	query = strings.ToLower(strings.TrimSpace(query))

	// Book name abbreviations mapping
	bookAbbrevs := map[string][]string{
		"genesis":         {"gen", "ge", "gn"},
		"exodus":          {"exo", "ex", "exod"},
		"leviticus":       {"lev", "le", "lv"},
		"numbers":         {"num", "nu", "nm", "nb"},
		"deuteronomy":     {"deut", "de", "dt"},
		"joshua":          {"josh", "jos", "jsh"},
		"judges":          {"judg", "jdg", "jg", "jdgs"},
		"ruth":            {"rut", "ru", "rth"},
		"1 samuel":        {"1sam", "1sa", "1samuel", "1 sam", "1 sa", "1s"},
		"2 samuel":        {"2sam", "2sa", "2samuel", "2 sam", "2 sa", "2s"},
		"1 kings":         {"1king", "1kgs", "1ki", "1k", "1 kings", "1 kgs"},
		"2 kings":         {"2king", "2kgs", "2ki", "2k", "2 kings", "2 kgs"},
		"1 chronicles":    {"1chron", "1chr", "1ch", "1 chronicles", "1 chr"},
		"2 chronicles":    {"2chron", "2chr", "2ch", "2 chronicles", "2 chr"},
		"ezra":            {"ezr", "ez"},
		"nehemiah":        {"neh", "ne"},
		"esther":          {"est", "es"},
		"job":             {"jb"},
		"psalms":          {"psalm", "psa", "ps", "pss"},
		"proverbs":        {"prov", "pro", "pr", "prv"},
		"ecclesiastes":    {"eccl", "ecc", "ec", "qoh"},
		"song of solomon": {"song", "sos", "so", "canticle", "canticles", "song of songs"},
		"isaiah":          {"isa", "is"},
		"jeremiah":        {"jer", "je", "jr"},
		"lamentations":    {"lam", "la"},
		"ezekiel":         {"ezek", "eze", "ezk"},
		"daniel":          {"dan", "da", "dn"},
		"hosea":           {"hos", "ho"},
		"joel":            {"joe", "jl"},
		"amos":            {"amo", "am"},
		"obadiah":         {"obad", "ob"},
		"jonah":           {"jon", "jnh"},
		"micah":           {"mic", "mi"},
		"nahum":           {"nah", "na"},
		"habakkuk":        {"hab", "hb"},
		"zephaniah":       {"zeph", "zep", "zp"},
		"haggai":          {"hag", "hg"},
		"zechariah":       {"zech", "zec", "zc"},
		"malachi":         {"mal", "ml"},
		"matthew":         {"matt", "mat", "mt"},
		"mark":            {"mar", "mrk", "mk", "mr"},
		"luke":            {"luk", "lk"},
		"john":            {"joh", "jhn", "jn"},
		"acts":            {"act", "ac"},
		"romans":          {"rom", "ro", "rm"},
		"1 corinthians":   {"1cor", "1co", "1 corinthians", "1 cor"},
		"2 corinthians":   {"2cor", "2co", "2 corinthians", "2 cor"},
		"galatians":       {"gal", "ga"},
		"ephesians":       {"eph", "ephes"},
		"philippians":     {"phil", "php", "pp"},
		"colossians":      {"col", "co"},
		"1 thessalonians": {"1thess", "1th", "1 thessalonians", "1 thess"},
		"2 thessalonians": {"2thess", "2th", "2 thessalonians", "2 thess"},
		"1 timothy":       {"1tim", "1ti", "1 timothy", "1 tim"},
		"2 timothy":       {"2tim", "2ti", "2 timothy", "2 tim"},
		"titus":           {"tit", "ti"},
		"philemon":        {"philem", "phm", "pm"},
		"hebrews":         {"heb", "he"},
		"james":           {"jam", "jas", "jm"},
		"1 peter":         {"1pet", "1pe", "1pt", "1p", "1 peter", "1 pet"},
		"2 peter":         {"2pet", "2pe", "2pt", "2p", "2 peter", "2 pet"},
		"1 john":          {"1john", "1jn", "1jo", "1j", "1 john"},
		"2 john":          {"2john", "2jn", "2jo", "2j", "2 john"},
		"3 john":          {"3john", "3jn", "3jo", "3j", "3 john"},
		"jude":            {"jud", "jd"},
		"revelation":      {"rev", "re", "rv"},
	}

	// Try exact match first
	for _, book := range books {
		if strings.ToLower(book.Name) == query {
			return book.BookID, book.Name, true
		}
	}

	// Try abbreviation match
	for _, book := range books {
		bookNameLower := strings.ToLower(book.Name)
		if abbrevs, ok := bookAbbrevs[bookNameLower]; ok {
			for _, abbrev := range abbrevs {
				if query == abbrev {
					return book.BookID, book.Name, true
				}
			}
		}
	}

	// Try prefix match
	for _, book := range books {
		if strings.HasPrefix(strings.ToLower(book.Name), query) {
			return book.BookID, book.Name, true
		}
	}

	return 0, "", false
}

// parseReference accepts a wide variety of reference formats:
//   - "John 3:16"       canonical
//   - "john3:16"        no spaces
//   - "john 3 16"       spaces instead of colon
//   - "john 3:16-17"    range
//   - "john 3 16-17"    range with spaces
//   - "1 John 3:16"     book name starting with a digit
//   - "1john3:16"       no spaces, book starts with digit
//   - "1 1:1"           book by numeric id + chapter:verse
//   - "gen 1"           book + chapter only
//   - "gen"             book only (defaults to chapter 1)
//
// The split into book and the rest happens at the boundary between the
// (optional digit-prefixed) word that names the book and the numeric
// chapter/verse data that follows. Then any digit-runs in the rest are
// pulled out in order as chapter / verse-start / verse-end, with a
// hyphen interpreted as the range separator.
func parseReference(ref string, books []api.Book) (book, chapter, verseStart, verseEnd int, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, 0, 0, 0, fmt.Errorf("empty reference")
	}

	// Book identifier alternatives, in order of specificity. Each
	// letter run may be followed by ` letter-run` repeats so multi-word
	// book names like "Song of Solomon" or "1 Samuel" stay intact.
	//   1. digit + optional whitespace + letters (+ more words)  →  "1 John", "1john", "1 Samuel"
	//   2. letters (+ more words)                                 →  "John", "rom", "Song of Solomon"
	//   3. digit                                                  →  "1" (book id)
	m := referenceRe.FindStringSubmatch(ref)
	if m == nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid reference: %s", ref)
	}
	bookPart := strings.TrimSpace(m[1])
	rest := m[2]

	// Resolve the book identifier.
	if id, perr := strconv.Atoi(bookPart); perr == nil {
		book = id
	} else if len(books) > 0 {
		id, _, found := fuzzyMatchBook(bookPart, books)
		if !found {
			return 0, 0, 0, 0, fmt.Errorf("book not found: %s", bookPart)
		}
		book = id
	} else {
		return 0, 0, 0, 0, fmt.Errorf("no books loaded")
	}

	// Default chapter when none was supplied.
	chapter = 1

	if rest == "" {
		return book, chapter, 0, 0, nil
	}

	// Split into before/after a hyphen so verse-range parsing is robust
	// even with arbitrary separators around it ("3:16-17", "3 16 - 17",
	// "3 16-17").
	beforeDash, afterDash := rest, ""
	if i := strings.IndexByte(rest, '-'); i >= 0 {
		beforeDash = rest[:i]
		afterDash = rest[i+1:]
	}

	beforeNums := digitsRe.FindAllString(beforeDash, -1)
	afterNums := digitsRe.FindAllString(afterDash, -1)

	if len(beforeNums) >= 1 {
		chapter, _ = strconv.Atoi(beforeNums[0])
	}
	if len(beforeNums) >= 2 {
		verseStart, _ = strconv.Atoi(beforeNums[1])
		verseEnd = verseStart
	}
	if len(afterNums) >= 1 {
		verseEnd, _ = strconv.Atoi(afterNums[0])
		if verseStart == 0 && len(beforeNums) >= 2 {
			verseStart, _ = strconv.Atoi(beforeNums[1])
		}
	}

	return book, chapter, verseStart, verseEnd, nil
}

// validateReference checks a parsed reference against the loaded books so
// out-of-range lookups ("gen 99", "999 1") are rejected instead of opening
// an empty chapter and saving it as the reading position. A reversed verse
// range ("1:5-2") is swapped.
func validateReference(books []api.Book, bookID, chapter, verseStart, verseEnd int) (api.Book, int, int, int, error) {
	var book api.Book
	found := false
	for _, b := range books {
		if b.BookID == bookID {
			book, found = b, true
			break
		}
	}
	if !found {
		return api.Book{}, 0, 0, 0, fmt.Errorf("no book %d in this translation", bookID)
	}
	if chapter < 1 || chapter > book.Chapters {
		return api.Book{}, 0, 0, 0, fmt.Errorf("%s has %d chapters", book.Name, book.Chapters)
	}
	if verseEnd < verseStart {
		verseStart, verseEnd = verseEnd, verseStart
	}
	if verseStart == 0 && verseEnd > 0 {
		verseStart = verseEnd
	}
	return book, chapter, verseStart, verseEnd, nil
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
