package esv

import (
	"fmt"

	"omasword/internal/api"
)

// book is one Protestant canon book. verses is the traditional verse
// total (the Bible's 31,102). ESV 3 John has 15 verses rather than 14;
// using the smaller total makes that book's cap stricter, never looser.
// chapters is the chapter count used for the book list.
type book struct {
	name     string
	chapters int
	verses   int
}

// canon is indexed by book ID minus one, matching bolls.life book IDs.
var canon = [...]book{
	{"Genesis", 50, 1533},
	{"Exodus", 40, 1213},
	{"Leviticus", 27, 859},
	{"Numbers", 36, 1288},
	{"Deuteronomy", 34, 959},
	{"Joshua", 24, 658},
	{"Judges", 21, 618},
	{"Ruth", 4, 85},
	{"1 Samuel", 31, 810},
	{"2 Samuel", 24, 695},
	{"1 Kings", 22, 816},
	{"2 Kings", 25, 719},
	{"1 Chronicles", 29, 942},
	{"2 Chronicles", 36, 822},
	{"Ezra", 10, 280},
	{"Nehemiah", 13, 406},
	{"Esther", 10, 167},
	{"Job", 42, 1070},
	{"Psalms", 150, 2461},
	{"Proverbs", 31, 915},
	{"Ecclesiastes", 12, 222},
	{"Song of Solomon", 8, 117},
	{"Isaiah", 66, 1292},
	{"Jeremiah", 52, 1364},
	{"Lamentations", 5, 154},
	{"Ezekiel", 48, 1273},
	{"Daniel", 12, 357},
	{"Hosea", 14, 197},
	{"Joel", 3, 73},
	{"Amos", 9, 146},
	{"Obadiah", 1, 21},
	{"Jonah", 4, 48},
	{"Micah", 7, 105},
	{"Nahum", 3, 47},
	{"Habakkuk", 3, 56},
	{"Zephaniah", 3, 53},
	{"Haggai", 2, 38},
	{"Zechariah", 14, 211},
	{"Malachi", 4, 55},
	{"Matthew", 28, 1071},
	{"Mark", 16, 678},
	{"Luke", 24, 1151},
	{"John", 21, 879},
	{"Acts", 28, 1007},
	{"Romans", 16, 433},
	{"1 Corinthians", 16, 437},
	{"2 Corinthians", 13, 257},
	{"Galatians", 6, 149},
	{"Ephesians", 6, 155},
	{"Philippians", 4, 104},
	{"Colossians", 4, 95},
	{"1 Thessalonians", 5, 89},
	{"2 Thessalonians", 3, 47},
	{"1 Timothy", 6, 113},
	{"2 Timothy", 4, 83},
	{"Titus", 3, 46},
	{"Philemon", 1, 25},
	{"Hebrews", 13, 303},
	{"James", 5, 108},
	{"1 Peter", 5, 105},
	{"2 Peter", 3, 61},
	{"1 John", 5, 105},
	{"2 John", 1, 13},
	{"3 John", 1, 14},
	{"Jude", 1, 25},
	{"Revelation", 22, 404},
}

// MaxStoredVerses is Crossway's cap on locally stored ESV text.
const MaxStoredVerses = 500

// MaxVerses is the most verses of one book that may be stored or shown
// at once: half the book or 500, whichever is less. Half is kept strictly
// under 50 percent, matching Crossway's rule that a quotation may not
// amount to 50 percent of a book.
func MaxVerses(bookID int) int {
	n := verseCount(bookID)
	if n <= 1 {
		return 1
	}
	half := (n - 1) / 2
	if half > MaxStoredVerses {
		return MaxStoredVerses
	}
	if half < 1 {
		return 1
	}
	return half
}

// PageStart is the first verse of the legal page that contains verse.
// Pages are MaxVerses long and start at verse 1.
func PageStart(bookID, verse int) int {
	if verse <= 1 {
		return 1
	}
	size := MaxVerses(bookID)
	if size < 1 {
		return 1
	}
	return ((verse-1)/size)*size + 1
}

func verseCount(bookID int) int {
	if bookID < 1 || bookID > len(canon) {
		return 0
	}
	return canon[bookID-1].verses
}

func chapterCount(bookID int) int {
	if bookID < 1 || bookID > len(canon) {
		return 0
	}
	return canon[bookID-1].chapters
}

// passageQuery is the api.esv.org reference for one chapter.
// Single-chapter books are requested by name alone: Crossway reads
// "Obadiah 1" as verse 1, and a verse range from our table would miss
// 3 John 15, which the ESV has and the traditional count does not.
func passageQuery(bookID, chapter int) (string, bool) {
	name, ok := BookName(bookID)
	if !ok || chapter < 1 || chapter > chapterCount(bookID) {
		return "", false
	}
	if chapterCount(bookID) == 1 {
		return name, true
	}
	return fmt.Sprintf("%s %d", name, chapter), true
}

// BookName is the English name used in an api.esv.org passage query.
func BookName(bookID int) (string, bool) {
	if bookID < 1 || bookID > len(canon) {
		return "", false
	}
	return canon[bookID-1].name, true
}

// Books is the 66-book list for the ESV reader. Chapter counts are facts
// about the canon, not a copy of the ESV text.
func Books() []api.Book {
	out := make([]api.Book, len(canon))
	for i, b := range canon {
		out[i] = api.Book{BookID: i + 1, ChronOrder: i + 1, Name: b.name, Chapters: b.chapters}
	}
	return out
}

// LookupBook matches the longest canon name at the start of ref, so
// "1 John" wins over a future shorter prefix. The remainder is returned
// unchanged aside from a leading space being fine for the caller to parse.
func LookupBook(ref string) (id int, rest string, ok bool) {
	best := 0
	for i, b := range canon {
		name := b.name
		if len(name) < best {
			continue
		}
		if len(ref) < len(name) || !equalFold(ref[:len(name)], name) {
			continue
		}
		if len(ref) > len(name) {
			switch ref[len(name)] {
			case ' ', '\t', ':':
			default:
				continue
			}
		}
		best = len(name)
		id = i + 1
		rest = ref[len(name):]
		ok = true
	}
	return id, rest, ok
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
