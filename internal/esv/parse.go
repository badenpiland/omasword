package esv

import (
	"errors"
	"html"
	"regexp"
	"strconv"
	"strings"

	"omasword/internal/api"
)

// errNoVerses means the response could not be split into verses. Callers
// surface it instead of showing an unstructured blob.
var errNoVerses = errors.New("ESV response had no verse text")

// verseNumRe matches the verse-number marker the HTML endpoint emits,
// including the id attribute from the documented response.
var verseNumRe = regexp.MustCompile(`<(?:b|span)\s+[^>]*class="verse-num"[^>]*>\s*(\d+)\s*(?:&nbsp;)?\s*</(?:b|span)>`)

var tagRe = regexp.MustCompile(`<[^>]*>`)

// ParseHTML splits one passage HTML fragment into verses. notice is the
// copyright text Crossway sent with the passage, or "" when the fragment
// only has the short "(ESV)" marker. Verse wording is kept; tags and
// indentation are not part of the text.
func ParseHTML(fragment string) (verses []api.Verse, notice string, err error) {
	notice = extractNotice(fragment)
	body := fragment
	if i := strings.Index(fragment, "Scripture quotations"); i >= 0 {
		body = fragment[:i]
	}
	matches := verseNumRe.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return nil, notice, errNoVerses
	}
	for i, m := range matches {
		n, convErr := strconv.Atoi(body[m[2]:m[3]])
		if convErr != nil || n <= 0 {
			continue
		}
		end := len(body)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		text := cleanVerse(body[m[1]:end])
		if text == "" {
			continue
		}
		verses = append(verses, api.Verse{Verse: n, Text: text})
	}
	if len(verses) == 0 {
		return nil, notice, errNoVerses
	}
	return verses, notice, nil
}

func cleanVerse(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(api.SanitizeText(s))
}

func extractNotice(fragment string) string {
	const marker = "Scripture quotations"
	if i := strings.Index(fragment, marker); i >= 0 {
		return cleanVerse(fragment[i:])
	}
	return ""
}
