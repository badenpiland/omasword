package api

import "testing"

func TestSanitizeText(t *testing.T) {
	tests := map[string]string{
		"In the beginning":                  "In the beginning",
		"Jesus wept.\x1b]52;c;cHduZWQ=\x07": "Jesus wept.]52;c;cHduZWQ=",
		"clear\x1b[2Jscreen":                "clear[2Jscreen",
		"8-bit CSI\u009b31m":                "8-bit CSI31m",
		"line\none\ttab":                    "line one tab",
		"bidi \u202eoverride\u202c":         "bidi override",
		"“quotes” — em dash, ünïcödé":       "“quotes” — em dash, ünïcödé",
	}
	for in, want := range tests {
		if got := SanitizeText(in); got != want {
			t.Errorf("SanitizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidTranslationName(t *testing.T) {
	for _, ok := range []string{"KJV", "NLT", "UKR_OGI", "RV1960", "NASB", "SYNOD+"} {
		if !ValidTranslationName(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "..", "../KJV", "KJV/../../x", "a/b", `a\b`, ".hidden", "KJV.json", "x\x00", " KJV"} {
		if ValidTranslationName(bad) {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
