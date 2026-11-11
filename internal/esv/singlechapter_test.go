package esv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSingleChapterQueriesUseVerseRanges(t *testing.T) {
	const key = "test-key-not-real"
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		got = append(got, q)
		// Model Crossway: "Obadiah 1" is verse 1. The range is the chapter.
		if q == "Obadiah 1" || q == "Jude 1" || q == "Philemon 1" || q == "2 John 1" || q == "3 John 1" {
			fmt.Fprintf(w, `{"passages":["[1] verse one only."]}`)
			return
		}
		fmt.Fprintf(w, `{"passages":[%q]}`, obadiahChapter)
	}))
	defer srv.Close()

	c, err := New(key, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	page, err := c.Chapter(31, 1, 1) // Obadiah
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Verses) != MaxVerses(31) {
		t.Fatalf("Obadiah page = %d verses, want the half-book cap %d", len(page.Verses), MaxVerses(31))
	}
	have := map[int]string{}
	for _, v := range page.Verses {
		have[v.Verse] = v.Text
	}
	for _, n := range []int{2, 3, 4} {
		if have[n] == "" {
			t.Fatalf("Obadiah page missing verse %d: %+v", n, page.Verses)
		}
	}
	if strings.Contains(have[1], "verse one only") {
		t.Fatal("client sent the bare chapter query and cached verse 1")
	}

	// The other one-chapter books, plus a normal chapter that must stay
	// "John 3" rather than a verse range.
	for _, id := range []int{57, 63, 64, 65, 43} {
		chapter := 1
		if id == 43 {
			chapter = 3
		}
		if _, err := c.Chapter(id, chapter, 1); err != nil {
			t.Fatal(err)
		}
	}

	want := []string{
		"Obadiah",
		"Philemon",
		"2 John",
		"3 John",
		"Jude",
		"John 3",
	}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Fatalf("queries = %q", got)
	}
}

func TestPartialSingleChapterIsNotCached(t *testing.T) {
	const key = "test-key-not-real"
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Query().Get("q") != "Obadiah" {
			t.Errorf("query = %q", r.URL.Query().Get("q"))
		}
		// Short reply: do not treat this as the whole chapter.
		fmt.Fprintf(w, `{"passages":["[1] The vision of Obadiah."]}`)
	}))
	defer srv.Close()

	c, err := New(key, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	page, err := c.Chapter(31, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Verses) != 1 || page.Verses[0].Verse != 1 {
		t.Fatalf("page = %+v", page.Verses)
	}
	if _, err := c.Chapter(31, 1, 1); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("server hits = %d, want 2; a one-verse reply was cached as the chapter", hits)
	}
}
