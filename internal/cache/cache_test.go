package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTranslationNamesCannotEscapeCacheDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(root, "victim.json")
	if err := os.WriteFile(victim, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Cache{cacheDir: dir}

	if c.IsCached("../victim") {
		t.Error("IsCached must not see files outside the cache dir")
	}
	if err := c.RemoveTranslation("../victim"); err == nil {
		t.Error("RemoveTranslation accepted a path outside the cache dir")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("file outside the cache dir was touched: %v", err)
	}
	if err := c.DownloadTranslation("../../evil"); err == nil {
		t.Error("DownloadTranslation accepted a traversal name")
	}
}

func TestCachedVerseTextIsSanitized(t *testing.T) {
	dir := t.TempDir()
	data := `[{"pk":1,"verse":1,"text":"In the beginning\u001b]0;owned\u0007","book":1,"chapter":1}]`
	if err := os.WriteFile(filepath.Join(dir, "KJV.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Cache{cacheDir: dir}
	verses, err := c.GetChapter("KJV", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := verses[0].Text; got != "In the beginning]0;owned" {
		t.Fatalf("text = %q", got)
	}
}
