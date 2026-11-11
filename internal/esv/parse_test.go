package esv

import (
	"os"
	"testing"
)

func TestParseJohn3SplitsMidParagraphVerses(t *testing.T) {
	text := readFixture(t, "testdata/john3.txt")
	verses, _, err := ParsePassage(text)
	if err != nil {
		t.Fatal(err)
	}
	byNum := map[int]string{}
	for _, v := range verses {
		byNum[v.Verse] = v.Text
	}
	if byNum[16] == "" {
		t.Fatal("John 3:16 was empty")
	}
	if contains(byNum[16], "did not send") || contains(byNum[16], "[17]") {
		t.Fatalf("verse 16 contains the next verse: %q", byNum[16])
	}
	if contains(byNum[5], "That which is born") || contains(byNum[5], "Do not marvel") || contains(byNum[5], "The wind blows") {
		t.Fatalf("verse 5 swallowed the rest of the paragraph: %q", byNum[5])
	}
	if !contains(byNum[6], "That which is born") {
		t.Fatalf("verse 6 = %q", byNum[6])
	}
	if !contains(byNum[7], "Do not marvel") || !contains(byNum[8], "The wind blows") {
		t.Fatalf("verses 7 and 8 = %q / %q", byNum[7], byNum[8])
	}
	for _, v := range verses {
		if contains(v.Text, "permission site") || contains(v.Text, "ESV") {
			t.Fatalf("verse %d kept the copyright footer: %q", v.Verse, v.Text)
		}
	}
}

func TestParseObadiahSplitsAll21VersesAndDropsFooter(t *testing.T) {
	text := readFixture(t, "testdata/obadiah1.txt")
	verses, _, err := ParsePassage(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(verses) != 21 {
		t.Fatalf("got %d verses, want 21", len(verses))
	}
	if verses[0].Verse != 1 || verses[0].Text != "Obadiah fixture verse 1." {
		t.Fatalf("verse 1 = %+v", verses[0])
	}
	if contains(verses[0].Text, "verse 2") {
		t.Fatalf("verse 1 swallowed the chapter: %q", verses[0].Text)
	}
	last := verses[len(verses)-1]
	if last.Verse != 21 || last.Text != "Obadiah fixture verse 21." {
		t.Fatalf("verse 21 = %+v", last)
	}
	for _, v := range verses {
		if contains(v.Text, "permission site") || contains(v.Text, "ESV") {
			t.Fatalf("verse %d kept the copyright footer: %q", v.Verse, v.Text)
		}
	}
}

func TestParsePassageRejectsABlobWithoutMarkers(t *testing.T) {
	if _, _, err := ParsePassage("Jesus wept."); err == nil {
		t.Fatal("expected an error when no [N] markers are present")
	}
}

func TestParsePassageKeepsScriptureQuotationsAsNoticeOnly(t *testing.T) {
	const text = "[35] Jesus wept.\nScripture quotations are from the ESV® Bible, © 2001 by Crossway."
	verses, notice, err := ParsePassage(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(verses) != 1 || verses[0].Text != "Jesus wept." {
		t.Fatalf("verses = %+v", verses)
	}
	if !contains(notice, "Scripture quotations") || contains(verses[0].Text, "Scripture") {
		t.Fatalf("notice = %q verse = %q", notice, verses[0].Text)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
