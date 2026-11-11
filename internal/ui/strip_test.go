package ui

import (
	"strings"
	"testing"
)

func TestStripHTMLTagsDropsControlCharacters(t *testing.T) {
	out := stripHTMLTags("In the <em>beginning</em>&#27;]0;owned&#7; \x1b[2J God&#x1b;")
	if strings.ContainsAny(out, "\x1b\x07") {
		t.Fatalf("control characters survived: %q", out)
	}
	if !strings.Contains(out, "In the beginning") {
		t.Fatalf("text was mangled: %q", out)
	}
}

func TestStripHTMLTagsDropsStrongsNumbers(t *testing.T) {
	// Real KJV text from bolls.life, Genesis 1:1 and a search hit.
	cases := map[string]string{
		"In the beginning<S>7225</S> God<S>430</S> created<S>1254</S> <S>853</S> the heaven<S>8064</S> and<S>853</S> the earth<S>776</S>.": "In the beginning God created the heaven and the earth.",
		"he that is <mark>begotten</mark><S>1080</S> of<S>1537</S> God<S>2316</S>":                                                         "he that is begotten of God",
		"<sup>A Psalm of David.</sup> Jehovah<S>3068</S> is my shepherd<S>7462</S>;":                                                       "A Psalm of David. Jehovah is my shepherd;",
	}
	for in, want := range cases {
		if got := stripHTMLTags(in); got != want {
			t.Errorf("stripHTMLTags(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
