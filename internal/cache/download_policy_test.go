package cache

import (
	"strings"
	"testing"
)

func TestDownloadRejectsCopyrightedTranslation(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	c, err := NewCache()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ESV", "NLT", "CSB17", "DARBY"} {
		err := c.DownloadTranslation(name)
		if err == nil || !strings.Contains(err.Error(), "public-domain") {
			t.Fatalf("DownloadTranslation(%s) err = %v", name, err)
		}
		if c.IsCached(name) {
			t.Fatalf("%s was written to the cache", name)
		}
	}
}
