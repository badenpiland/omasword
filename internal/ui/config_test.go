package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAnUnreadableConfigIsReported(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := filepath.Join(cfg, "omasword")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 36})
	m = next.(Model)
	if !strings.Contains(plain(m.renderStatusBar()), "config.json is unreadable") {
		t.Fatalf("status bar: %q", plain(m.renderStatusBar()))
	}
	m, _ = press(t, m, "j")
	if m.configNote != "" {
		t.Fatal("the warning outlasted a key press")
	}
}
