package ui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"omasword/internal/update"
)

// realFindUpdate keeps the shipped check so its opt-out can be tested.
var realFindUpdate = findUpdate

// No ui test may reach GitHub: the daily check is stubbed for the package.
func init() {
	findUpdate = func() string { return "" }
}

func TestUpdateNoticeShowsInStatusBar(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	findUpdate = func() string { return "v0.4.0" }
	t.Cleanup(func() { findUpdate = func() string { return "" } })

	msg := checkForUpdate()()
	if msg != (updateAvailableMsg{tag: "v0.4.0"}) {
		t.Fatalf("msg = %#v", msg)
	}
	m := NewModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	next, _ = next.(Model).Update(msg)
	m = next.(Model)
	if bar := m.renderStatusBar(); !strings.Contains(bar, "v0.4.0 available") {
		t.Fatalf("status bar has no notice:\n%s", bar)
	}

	findUpdate = func() string { return "" }
	if msg := checkForUpdate()(); msg != nil {
		t.Fatalf("no update should send nothing, got %#v", msg)
	}
}

func TestUpdateCheckOptOut(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// The environment variable turns it off for one run.
	t.Setenv(update.OptOutEnv, "1")
	if got := realFindUpdate(); got != "" {
		t.Fatalf("opted out by env, got %q", got)
	}

	// update_check: false in config.json turns it off for good.
	t.Setenv(update.OptOutEnv, "")
	dir := filepath.Join(cfgHome, "omasword")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"update_check": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := realFindUpdate(); got != "" {
		t.Fatalf("opted out in config, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "omasword", "update-check.json")); !os.IsNotExist(err) {
		t.Fatal("an opted-out reader recorded a check")
	}
}

func TestSavingSettingsKeepsUpdateCheckOptOut(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path := filepath.Join(cfgHome, "omasword", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"selected_translation":"KJV","update_check":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.currentChapter = 3
	if err := m.saveSettings(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		UpdateCheck *bool `json:"update_check"`
	}
	if err := json.Unmarshal(data, &saved); err != nil || saved.UpdateCheck == nil || *saved.UpdateCheck {
		t.Fatalf("opt-out lost on save: %s", data)
	}
}

// The notice shows over the saved-verse count, but never hides the label
// that says the BSB is standing in for the ESV.
func TestUpdateNoticePriority(t *testing.T) {
	m := esvReader(t)
	m, _ = press(t, m, "S")
	m.status = ""
	m.updateNotice = "v0.4.0"
	if bar := plain(m.renderStatusBar()); !strings.Contains(bar, "v0.4.0 available") {
		t.Fatalf("saved count hid the notice: %q", bar)
	}
	next, _ := m.Update(chapterLoadedMsg{
		verses: m.currentVerses, book: 2, chapter: 1,
		fallback: "BSB", fallbackErr: errors.New("dial tcp: network is unreachable"),
	})
	m = next.(Model)
	if bar := plain(m.renderStatusBar()); !strings.Contains(bar, "BSB shown") {
		t.Fatalf("notice hid the BSB label: %q", bar)
	}
}
