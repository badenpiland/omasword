package api

import (
	"regexp"
	"strings"
)

// Everything the bolls.life API returns ends up either on the terminal or,
// for translation names, in file paths and URLs. These helpers keep a
// misbehaving or compromised server (or a tampered cache file) from
// injecting terminal escape sequences or escaping the cache directory.

// SanitizeText removes characters that a terminal would interpret instead
// of print: C0/C1 control characters (ESC starts every escape sequence,
// OSC 52 among them writes the clipboard) and Unicode bidi overrides,
// which can make displayed text read differently from what it is.
// Tabs and newlines become spaces.
func SanitizeText(s string) string {
	if !needsSanitizing(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case isUnsafeRune(r):
			// drop
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func needsSanitizing(s string) bool {
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' || isUnsafeRune(r) {
			return true
		}
	}
	return false
}

func isUnsafeRune(r rune) bool {
	switch {
	case r < 0x20, r == 0x7f: // C0 controls, DEL
		return true
	case r >= 0x80 && r <= 0x9f: // C1 controls (8-bit CSI, OSC, ...)
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069: // bidi embeddings/overrides/isolates
		return true
	}
	return false
}

var translationNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_+-]{0,31}$`)

// ValidTranslationName reports whether name is safe to use as a URL path
// segment and as a cache file name. Real bolls.life short names look like
// "KJV", "NASB", "UKR_OGI" or "RV1960".
func ValidTranslationName(name string) bool {
	return translationNameRe.MatchString(name)
}
