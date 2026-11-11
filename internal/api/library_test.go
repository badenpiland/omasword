package api

import (
	"strings"
	"testing"
)

func TestEnglishBuiltinsDropsCopyrightedAndKeepsOrder(t *testing.T) {
	groups := []LanguageGroup{
		{
			Language: "German",
			Translations: []Translation{
				{ShortName: "LUTH", FullName: "Luther"},
			},
		},
		{
			Language: "English",
			Translations: []Translation{
				{ShortName: "YLT", FullName: "Young's"},
				{ShortName: "ESV", FullName: "English Standard Version"},
				{ShortName: "NLT", FullName: "New Living Translation"},
				{ShortName: "NKJV", FullName: "New King James"},
				{ShortName: "BSB", FullName: "Berean Standard Bible"},
				{ShortName: "KJV", FullName: "King James"},
				{ShortName: "CSB17", FullName: "Christian Standard Bible"},
				{ShortName: "WEB", FullName: "World English Bible"},
				{ShortName: "../KJV", FullName: "nope"},
			},
		},
	}
	got := englishBuiltins(groups)
	var names []string
	for _, t := range got {
		names = append(names, t.ShortName)
	}
	if strings.Join(names, ",") != "BSB,KJV,WEB,YLT" {
		t.Fatalf("englishBuiltins = %s", strings.Join(names, ","))
	}
}

type stubCache struct {
	calls int
}

func (s *stubCache) IsCached(string) bool { return true }
func (s *stubCache) GetChapter(string, int, int) ([]Verse, error) {
	s.calls++
	return []Verse{{Verse: 1, Text: "from the bulk cache"}}, nil
}

func TestCopyrightedChapterIsNotServedFromCacheOrNetwork(t *testing.T) {
	c := NewClient()
	cache := &stubCache{}
	c.SetCache(cache)
	for _, name := range []string{"ESV", "NLT", "CSB17"} {
		if _, err := c.GetChapter(name, 43, 3); err == nil || !strings.Contains(err.Error(), "public-domain") {
			t.Fatalf("GetChapter(%s) err = %v", name, err)
		}
		if _, err := c.GetBooks(name); err == nil {
			t.Fatalf("GetBooks(%s) was allowed", name)
		}
		if _, err := c.SearchVerses(name, "love"); err == nil {
			t.Fatalf("SearchVerses(%s) was allowed", name)
		}
	}
	if _, err := c.GetParallelVerses(ParallelVerseRequest{Translations: []string{"KJV", "ESV"}, Book: 43, Chapter: 3, Verses: []int{16}}); err == nil {
		t.Fatal("parallel request including ESV was allowed")
	}
	if cache.calls != 0 {
		t.Fatalf("bulk cache was read %d times", cache.calls)
	}
}

func TestBuiltinChapterStillReadsTheCache(t *testing.T) {
	c := NewClient()
	cache := &stubCache{}
	c.SetCache(cache)
	verses, err := c.GetChapter("KJV", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cache.calls != 1 || len(verses) != 1 || verses[0].Text != "from the bulk cache" {
		t.Fatalf("verses = %+v calls %d", verses, cache.calls)
	}
}
