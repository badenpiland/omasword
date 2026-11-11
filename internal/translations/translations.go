// Package translations is the allowlist of texts omasword will fetch or
// cache from bolls.life. Copyrighted translations are absent on purpose:
// a third party hosting them is not a license to redistribute them.
package translations

// Default is the built-in translation opened when there is no saved
// choice, or when the saved choice is no longer allowed. BSB is a modern
// word-for-word text, public domain since 2023, and bolls.life serves it.
const Default = "BSB"

// Info is one public-domain translation. Flavor is the one line shown in
// the picker: philosophy, era or reading level, and a notable trait.
type Info struct {
	ShortName string
	FullName  string
	Flavor    string
}

// Builtins is every translation the bolls.life client may request and the
// only set the cache may download. Order is the picker order.
//
// Darby is public domain, but bolls.life does not host it, so it is not
// listed. Do not add ESV, NLT, CSB, NIV, NASB, or any other copyrighted
// text here, even when bolls.life has a file for it.
var Builtins = []Info{
	{ShortName: "BSB", FullName: "Berean Standard Bible", Flavor: "Modern word-for-word; public domain since 2023"},
	{ShortName: "KJV", FullName: "King James Version", Flavor: "Formal word-for-word, 1611; Early Modern English"},
	{ShortName: "WEB", FullName: "World English Bible", Flavor: "Modern word-for-word; uses Yahweh for God's name"},
	{ShortName: "ASV", FullName: "American Standard Version", Flavor: "Formal word-for-word, 1901; uses Jehovah"},
	{ShortName: "YLT", FullName: "Young's Literal Translation", Flavor: "Strictly literal, 1898; Hebrew and Greek word order"},
	{ShortName: "GNV", FullName: "Geneva Bible", Flavor: "Formal word-for-word, 1599; Reformation-era English"},
	{ShortName: "DRB", FullName: "Douay-Rheims Bible", Flavor: "Formal word-for-word from the Latin Vulgate"},
}

// Comparison is the side-by-side set opened by the comparison view:
// the default, a classic formal text, and a modern one that reads differently.
func Comparison() []string {
	return []string{Default, "KJV", "WEB"}
}

// Lookup returns the catalog entry for a short name.
func Lookup(shortName string) (Info, bool) {
	for _, info := range Builtins {
		if info.ShortName == shortName {
			return info, true
		}
	}
	return Info{}, false
}

// IsBuiltin reports whether shortName may be fetched or cached from bolls.life.
func IsBuiltin(shortName string) bool {
	_, ok := Lookup(shortName)
	return ok
}

// LookupFlavor is the picker's one-line description, or "" when shortName
// is not a built-in translation.
func LookupFlavor(shortName string) string {
	info, ok := Lookup(shortName)
	if !ok {
		return ""
	}
	return info.Flavor
}

// ShortNames returns the built-in short names in picker order.
func ShortNames() []string {
	names := make([]string, len(Builtins))
	for i, info := range Builtins {
		names[i] = info.ShortName
	}
	return names
}
