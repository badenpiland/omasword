package ui

import (
	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/translations"
)

// translationWindow is how many translations the picker shows at once.
// translationItemLines is the name line plus the flavor line; the mouse
// hit-testing in overlayClick divides by this.
const (
	translationWindow    = 8
	translationItemLines = 2
)

// leftoverDownloads are downloaded translations that are no longer
// offered, such as an ESV file saved by an older build. They are never
// read, but they stay on disk until removed with X in the cache manager.
func (m Model) leftoverDownloads() []string {
	var out []string
	for _, name := range m.cachedTranslations {
		if !translations.IsBuiltin(name) && api.ValidTranslationName(name) {
			out = append(out, name)
		}
	}
	return out
}

// presentTranslations keeps the public-domain allowlist, in catalog order,
// with the short catalog names. Anything else in the server list (ESV,
// NLT, and the other copyrighted texts bolls.life hosts) is dropped.
func presentTranslations(in []api.Translation) []api.Translation {
	byName := make(map[string]api.Translation, len(in))
	for _, t := range in {
		byName[t.ShortName] = t
	}
	out := make([]api.Translation, 0, len(translations.Builtins))
	for _, info := range translations.Builtins {
		t, ok := byName[info.ShortName]
		if !ok {
			continue
		}
		t.FullName = info.FullName
		out = append(out, t)
	}
	// ESV is not served from bolls.life. It is listed so a reader with
	// their own Crossway key can select it. The download manager refuses
	// to cache it.
	out = append(out, api.Translation{ShortName: esv.ID, FullName: "English Standard Version"})
	return out
}
