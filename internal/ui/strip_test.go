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
