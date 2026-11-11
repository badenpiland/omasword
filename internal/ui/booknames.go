package ui

// bookNames are the 66 book names by bolls.life book ID. They stand in
// until the translation's own book list loads, so the title never shows
// the wrong book.
var bookNames = [...]string{
	"Genesis", "Exodus", "Leviticus", "Numbers", "Deuteronomy", "Joshua",
	"Judges", "Ruth", "1 Samuel", "2 Samuel", "1 Kings", "2 Kings",
	"1 Chronicles", "2 Chronicles", "Ezra", "Nehemiah", "Esther", "Job",
	"Psalms", "Proverbs", "Ecclesiastes", "Song of Solomon", "Isaiah",
	"Jeremiah", "Lamentations", "Ezekiel", "Daniel", "Hosea", "Joel", "Amos",
	"Obadiah", "Jonah", "Micah", "Nahum", "Habakkuk", "Zephaniah", "Haggai",
	"Zechariah", "Malachi", "Matthew", "Mark", "Luke", "John", "Acts",
	"Romans", "1 Corinthians", "2 Corinthians", "Galatians", "Ephesians",
	"Philippians", "Colossians", "1 Thessalonians", "2 Thessalonians",
	"1 Timothy", "2 Timothy", "Titus", "Philemon", "Hebrews", "James",
	"1 Peter", "2 Peter", "1 John", "2 John", "3 John", "Jude", "Revelation",
}

// fallbackBookName returns the standard name for a book ID, or "" if the
// ID is out of range.
func fallbackBookName(id int) string {
	if id < 1 || id > len(bookNames) {
		return ""
	}
	return bookNames[id-1]
}
