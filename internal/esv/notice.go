package esv

// ID is the translation short name for the English Standard Version.
// Another licensed translation can follow this package's shape (its own
// package, config field, and storage cap). The UI routes on the short
// name; there is no provider registry.
const ID = "ESV"

// Flavor lines for the translation picker. Both stay on one row.
const (
	FlavorNeedsKey = "Formal word-for-word, 2001; paste a key from api.esv.org"
	FlavorReady    = "Formal word-for-word, 2001; online via your Crossway key"
)

// Link is shown wherever ESV text is shown. Crossway requires a link to
// www.esv.org on every page that displays the text.
const Link = "https://www.esv.org/"

// CopyrightNotice is Crossway's notice for digital quotations, from
// https://www.crossway.org/permissions/. It is the fallback when a
// response does not include its own notice. API terms also require
// showing whatever notice came back with the text; callers prefer that
// string when it is non-empty.
const CopyrightNotice = "Scripture quotations are from the ESV® Bible (The Holy Bible, English Standard Version®), © 2001 by Crossway, a publishing ministry of Good News Publishers. ESV Text Edition: 2025. The ESV text may not be quoted in any publication made available to the public by a Creative Commons license. The ESV may not be translated in whole or in part into any other language. Used by permission. All rights reserved."

// Attribution is the notice plus the required esv.org link.
func Attribution(notice string) string {
	if notice == "" {
		notice = CopyrightNotice
	}
	return notice + "\n" + Link
}
