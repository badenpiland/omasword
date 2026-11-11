package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"omasword/internal/api"
)

// leftoverCache is a cache with a fixed set of downloaded files.
type leftoverCache struct {
	files   []string
	removed []string
}

func (c *leftoverCache) IsCached(t string) bool { return slices.Contains(c.files, t) }
func (c *leftoverCache) GetChapter(string, int, int) ([]api.Verse, error) {
	return nil, nil
}
func (c *leftoverCache) DownloadTranslation(string) error    { return nil }
func (c *leftoverCache) DownloadProgress() (float64, string) { return 0, "" }
func (c *leftoverCache) ListCached() ([]string, error)       { return slices.Clone(c.files), nil }
func (c *leftoverCache) GetCacheSize() (int64, error)        { return 0, nil }
func (c *leftoverCache) RemoveTranslation(t string) error {
	c.removed = append(c.removed, t)
	c.files = slices.DeleteFunc(c.files, func(f string) bool { return f == t })
	return nil
}

func TestLeftoverDownloadsCanBeRemoved(t *testing.T) {
	// NewModel reads and may rewrite the config; keep it off the real one.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := &leftoverCache{files: []string{"BSB", "ESV", "NIV"}}
	m := NewModel()
	m.width, m.height = 120, 40
	m.cache = c
	m.translations = []api.Translation{{ShortName: "BSB", FullName: "Berean Standard Bible"}}
	m.mode = modeCacheManager
	m.cachedTranslations, _ = c.ListCached()

	if view := m.renderCacheManager(); !strings.Contains(view, "No longer offered: ESV, NIV") {
		t.Fatalf("cache manager does not list leftovers:\n%s", view)
	}

	next, cmd, handled := m.handleKey(tea.KeyPressMsg{Code: 'X', Text: "X"})
	if !handled || cmd == nil {
		t.Fatal("X was not handled")
	}
	if !slices.Equal(c.removed, []string{"ESV", "NIV"}) {
		t.Fatalf("removed %v, want ESV and NIV only", c.removed)
	}
	updated, _ := next.Update(cmd())
	if view := updated.(Model).renderCacheManager(); strings.Contains(view, "No longer offered") {
		t.Fatalf("leftovers still listed after removal:\n%s", view)
	}
}
