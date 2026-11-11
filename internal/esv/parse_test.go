package esv

import "testing"

func TestParseHTMLVersesAndCopyright(t *testing.T) {
	const fragment = `<h2>Obadiah 1</h2>
<p><b class="verse-num" id="v31001001-1">1&nbsp;</b>The vision of Obadiah.</p>
<p><b class="verse-num" id="v31001002-1">2&nbsp;</b>Behold, I will make you small among the nations; you shall be utterly despised.</p>
<p>Scripture quotations are from the ESV® Bible (The Holy Bible, English Standard Version®), © 2001 by Crossway, a publishing ministry of Good News Publishers. Used by permission. All rights reserved.</p>`

	verses, notice, err := ParseHTML(fragment)
	if err != nil {
		t.Fatal(err)
	}
	if len(verses) != 2 || verses[0].Verse != 1 || verses[0].Text != "The vision of Obadiah." {
		t.Fatalf("verses = %+v", verses)
	}
	if verses[1].Text != "Behold, I will make you small among the nations; you shall be utterly despised." {
		t.Fatalf("verse 2 = %q", verses[1].Text)
	}
	if notice == "" || notice == "ESV" {
		t.Fatalf("notice = %q", notice)
	}
	if !contains(notice, "Crossway") {
		t.Fatalf("notice = %q", notice)
	}
}

func TestParseHTMLUnescapesAndDropsTags(t *testing.T) {
	const fragment = `<p><b class="verse-num">16&nbsp;</b>For God so loved the world, that he gave his only Son, that whoever believes in him should not perish but have eternal life. <span class="footnote">&ldquo;only&rdquo;</span></p>`
	verses, _, err := ParseHTML(fragment)
	if err != nil {
		t.Fatal(err)
	}
	if len(verses) != 1 || verses[0].Verse != 16 {
		t.Fatalf("verses = %+v", verses)
	}
	if contains(verses[0].Text, "<") || contains(verses[0].Text, "ldquo") {
		t.Fatalf("markup survived: %q", verses[0].Text)
	}
	if !contains(verses[0].Text, "For God so loved") || !contains(verses[0].Text, "“only”") && !contains(verses[0].Text, "only") {
		t.Fatalf("text = %q", verses[0].Text)
	}
}

func TestParseHTMLRejectsABlobWithoutVerseNumbers(t *testing.T) {
	if _, _, err := ParseHTML("<p>Jesus wept.</p>"); err == nil {
		t.Fatal("expected an error when no verse numbers are marked")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
