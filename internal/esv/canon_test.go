package esv

import "testing"

func TestCanonTotals(t *testing.T) {
	if len(canon) != 66 {
		t.Fatalf("canon has %d books", len(canon))
	}
	verses, chapters := 0, 0
	for _, b := range canon {
		verses += b.verses
		chapters += b.chapters
	}
	if verses != 31102 {
		t.Fatalf("verse total = %d, want 31102", verses)
	}
	if chapters != 1189 {
		t.Fatalf("chapter total = %d, want 1189", chapters)
	}
}

func TestMaxVersesIsHalfOr500(t *testing.T) {
	// Psalms is far past 500, so the cap is the global 500.
	if got := MaxVerses(19); got != 500 {
		t.Fatalf("Psalms cap = %d, want 500", got)
	}
	// Obadiah has 21 verses; strictly under half is 10.
	if got := MaxVerses(31); got != 10 {
		t.Fatalf("Obadiah cap = %d, want 10", got)
	}
	// Jonah has 48 verses; 24 would be exactly half, which is not allowed.
	if got := MaxVerses(32); got != 23 {
		t.Fatalf("Jonah cap = %d, want 23", got)
	}
	if got := MaxVerses(63); got != 6 { // 2 John, 13 verses
		t.Fatalf("2 John cap = %d, want 6", got)
	}
}

func TestPageStartKeepsShortChaptersWhole(t *testing.T) {
	// John is far larger than one chapter, so verse 16 stays on page 1.
	if got := PageStart(43, 16); got != 1 {
		t.Fatalf("John page start = %d, want 1", got)
	}
	// Obadiah shows 10 verses per page.
	if got := PageStart(31, 15); got != 11 {
		t.Fatalf("Obadiah 15 page start = %d, want 11", got)
	}
	if got := PageStart(31, 21); got != 21 {
		t.Fatalf("Obadiah 21 page start = %d, want 21", got)
	}
}

func TestParseReference(t *testing.T) {
	book, chapter, verse, ok := ParseReference("Song of Solomon 2:4")
	if !ok || book != 22 || chapter != 2 || verse != 4 {
		t.Fatalf("song = %d %d:%d ok=%v", book, chapter, verse, ok)
	}
	book, chapter, verse, ok = ParseReference("1 John 1:9")
	if !ok || book != 62 || chapter != 1 || verse != 9 {
		t.Fatalf("1 john = %d %d:%d ok=%v", book, chapter, verse, ok)
	}
	if _, _, _, ok := ParseReference("Not A Book 1:1"); ok {
		t.Fatal("accepted an unknown book")
	}
}
