package translations

import "testing"

func TestDefaultIsBSBAndServed(t *testing.T) {
	info, ok := Lookup(Default)
	if !ok || info.ShortName != "BSB" {
		t.Fatalf("default = %+v, %v; want BSB in the catalog", info, ok)
	}
	if info.Flavor == "" || info.FullName == "" {
		t.Fatal("BSB needs a full name and a flavor line")
	}
}

func TestCopyrightedTextsAreExcluded(t *testing.T) {
	for _, name := range []string{"ESV", "NLT", "CSB", "CSB17", "NIV", "NASB", "NKJV", "DARBY"} {
		if IsBuiltin(name) {
			t.Errorf("%s must not be fetched or cached from bolls.life", name)
		}
	}
}

func TestNamedPublicDomainTextsStay(t *testing.T) {
	for _, name := range []string{"BSB", "KJV", "ASV", "WEB", "YLT", "GNV", "DRB"} {
		info, ok := Lookup(name)
		if !ok {
			t.Errorf("%s missing from the catalog", name)
			continue
		}
		if info.Flavor == "" {
			t.Errorf("%s has no flavor line", name)
		}
	}
}

func TestComparisonIsPublicDomain(t *testing.T) {
	got := Comparison()
	if len(got) != 3 || got[0] != Default {
		t.Fatalf("Comparison() = %v, want BSB first", got)
	}
	for _, name := range got {
		if !IsBuiltin(name) {
			t.Errorf("comparison translation %s is not public domain", name)
		}
	}
}

func TestEveryFlavorIsOneShortLine(t *testing.T) {
	for _, info := range Builtins {
		if info.Flavor == "" {
			t.Errorf("%s flavor is empty", info.ShortName)
		}
		if len(info.Flavor) > 58 {
			t.Errorf("%s flavor is %d bytes; keep it to one picker line", info.ShortName, len(info.Flavor))
		}
	}
}
