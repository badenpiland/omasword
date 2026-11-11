package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTripLeavesNoTempFiles(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	want := Settings{SelectedTranslation: "ESV", CurrentBook: 48, CurrentChapter: 4}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
	entries, _ := os.ReadDir(filepath.Join(cfg, "omasword"))
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("config dir should hold only config.json, got %v", entries)
	}
}
