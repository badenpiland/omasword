package esv

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"omasword/internal/api"
)

// errNoVerses means the response could not be split into verses. Callers
// surface it instead of showing an unstructured blob.
var errNoVerses = errors.New("ESV response had no verse text")

// verseMarker matches the [N] markers the passage-text endpoint emits,
// including when several verses share one paragraph.
var verseMarker = regexp.MustCompile(`\[(\d+)\]`)

// footerCut matches Crossway's trailing copyright lines. They are not
// verse text. The reader shows its own notice instead.
var footerCut = regexp.MustCompile(`(?i)(?:\(\s*ESV\s*\)|ESV\s*®\s*Bible|ESV®\s*Bible|please visit our permission|Scripture quotations)`)

// ParsePassage splits one passage-text response into verses. notice is
// Crossway's "Scripture quotations..." paragraph when the response
// included one. Footers such as "ESV ® Bible ... permission site" are
// removed from the verse text and are not returned as the notice.
func ParsePassage(text string) (verses []api.Verse, notice string, err error) {
	text, notice = splitNotice(text)
	text = cutFooter(text)
	matches := verseMarker.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, notice, errNoVerses
	}
	for i, m := range matches {
		n, convErr := strconv.Atoi(text[m[2]:m[3]])
		if convErr != nil || n <= 0 {
			continue
		}
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := cleanVerse(cutFooter(text[m[1]:end]))
		if body == "" {
			continue
		}
		verses = append(verses, api.Verse{Verse: n, Text: body})
	}
	if len(verses) == 0 {
		return nil, notice, errNoVerses
	}
	return verses, notice, nil
}

func splitNotice(s string) (body, notice string) {
	const marker = "Scripture quotations"
	if i := strings.Index(s, marker); i >= 0 {
		return s[:i], cleanVerse(s[i:])
	}
	return s, ""
}

func cutFooter(s string) string {
	if loc := footerCut.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	return s
}

func cleanVerse(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(api.SanitizeText(s))
}
