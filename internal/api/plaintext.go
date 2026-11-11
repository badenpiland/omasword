package api

import (
	"regexp"
	"strconv"
	"strings"
)

// Compiled once: PlainText runs for every verse on every redraw.
var (
	htmlTagRe = regexp.MustCompile(`<[^>]*>`)
	// strongsRe matches the Strong's concordance numbers bolls.life
	// puts after words in KJV and ASV ("beginning<S>7225</S>"). The
	// number goes with the tag, not just the markup around it.
	strongsRe       = regexp.MustCompile(`(?i)<S>\s*\d*\s*</S>`)
	numericEntityRe = regexp.MustCompile(`&#(\d+);`)
	hexEntityRe     = regexp.MustCompile(`&#[xX]([0-9a-fA-F]+);`)
	spaceRunRe      = regexp.MustCompile(`\s+`)
)

// PlainText turns verse text from bolls.life into plain text: Strong's
// numbers (KJV and ASV tag words with "<S>7225</S>") are dropped with
// their markers, other tags are removed, HTML entities are decoded, and
// terminal control characters are stripped.
func PlainText(s string) string {
	// Strip HTML tags. The bolls.life API wraps the matched search term
	// in <em>…</em> *inside* words (e.g. "lov<em>e</em>d"), so replacing
	// tags with a space would split such words. Drop them outright and
	// collapse any resulting double spaces at the end.
	s = strongsRe.ReplaceAllString(s, "")
	s = htmlTagRe.ReplaceAllString(s, "")

	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&ldquo;", "\u201C")  // Left double quote
	s = strings.ReplaceAll(s, "&rdquo;", "\u201D")  // Right double quote
	s = strings.ReplaceAll(s, "&lsquo;", "\u2018")  // Left single quote
	s = strings.ReplaceAll(s, "&rsquo;", "\u2019")  // Right single quote
	s = strings.ReplaceAll(s, "&mdash;", "\u2014")  // Em dash
	s = strings.ReplaceAll(s, "&ndash;", "\u2013")  // En dash
	s = strings.ReplaceAll(s, "&hellip;", "\u2026") // Ellipsis

	// Decode numeric HTML entities (e.g., &#8220; for left double quote)
	s = numericEntityRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the numeric code
		numStr := match[2 : len(match)-1]
		if num, err := strconv.Atoi(numStr); err == nil && num < 0x110000 {
			return string(rune(num))
		}
		return match
	})

	// Decode hex HTML entities (e.g., &#x201C; for left double quote)
	s = hexEntityRe.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the hex code
		hexStr := match[3 : len(match)-1]
		if num, err := strconv.ParseInt(hexStr, 16, 32); err == nil && num < 0x110000 {
			return string(rune(num))
		}
		return match
	})

	// Clean up multiple consecutive spaces
	s = spaceRunRe.ReplaceAllString(s, " ")

	// Trim leading and trailing spaces
	s = strings.TrimSpace(s)

	// Entities decoded above can produce control characters (&#27; is
	// ESC), so sanitize last.
	return SanitizeText(s)
}
