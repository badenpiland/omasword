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

// A config.json that isn't valid JSON (here, a hand edit gone wrong) is
// reported, and the next Save keeps it beside the new file instead of
// overwriting it, so esv_api_key_cmd can be copied back.
func TestSaveKeepsAnUnreadableConfig(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "omasword")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := []byte("{\n  \"current_chapter\":3 1,\n  \"esv_api_key_cmd\": \"example-key-cmd\"\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), broken, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); !IsUnreadable(err) {
		t.Fatalf("Load() error = %v, want an UnreadableError", err)
	}
	if err := Save(Settings{CurrentBook: 1, CurrentChapter: 1}); err != nil {
		t.Fatal(err)
	}

	kept, _ := filepath.Glob(filepath.Join(dir, "config.json.*.bad"))
	if len(kept) != 1 {
		t.Fatalf("kept %v, want one .bad file", kept)
	}
	if got, _ := os.ReadFile(kept[0]); string(got) != string(broken) {
		t.Fatalf("kept file = %q", got)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("the new config doesn't load: %v", err)
	}

	// A valid config is replaced as usual, with no new copy.
	if err := Save(Settings{CurrentBook: 2, CurrentChapter: 1}); err != nil {
		t.Fatal(err)
	}
	if again, _ := filepath.Glob(filepath.Join(dir, "config.json.*.bad")); len(again) != 1 {
		t.Fatalf("a valid config was kept too: %v", again)
	}
}
