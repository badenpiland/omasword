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

func TestESVKeyFileIsOwnerReadOnly(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	withKey := Settings{SelectedTranslation: "ESV", CurrentBook: 43, CurrentChapter: 16, ESVAPIKey: "test-key-not-real"}
	if err := Save(withKey); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(cfg, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600 while a key is stored", info.Mode().Perm())
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ESVAPIKey != "test-key-not-real" {
		t.Fatalf("loaded key = %q", got.ESVAPIKey)
	}

	withKey.ESVAPIKey = ""
	if err := Save(withKey); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(filepath.Join(cfg, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("config mode = %o, want 644 after the key is removed", info.Mode().Perm())
	}
	got, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ESVAPIKey != "" {
		t.Fatalf("key was not removed: %q", got.ESVAPIKey)
	}
}
