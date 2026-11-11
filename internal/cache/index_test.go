package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetChapterUsesIndexAndForgetsOnRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KJV.json")
	data := `[{"verse":1,"text":"In the beginning","book":1,"chapter":1},
	          {"verse":2,"text":"And the earth","book":1,"chapter":1},
	          {"verse":1,"text":"Thus the heavens","book":1,"chapter":2}]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Cache{cacheDir: dir}

	ch1, err := c.GetChapter("KJV", 1, 1)
	if err != nil || len(ch1) != 2 {
		t.Fatalf("chapter 1 = %v, %v", ch1, err)
	}
	ch1[0].Text = "mutated"

	// Change the file underneath: the indexed copy should still be served,
	// proving the file isn't re-read per chapter, and unaffected by the
	// caller's mutation.
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := c.GetChapter("KJV", 1, 1)
	if err != nil || len(again) != 2 || again[0].Text != "In the beginning" {
		t.Fatalf("indexed chapter 1 = %v, %v", again, err)
	}
	if ch2, _ := c.GetChapter("KJV", 1, 2); len(ch2) != 1 {
		t.Fatalf("chapter 2 = %v", ch2)
	}

	if err := c.RemoveTranslation("KJV"); err != nil {
		t.Fatal(err)
	}
	if c.loadedName != "" {
		t.Fatal("removing a translation should drop its in-memory index")
	}
}
