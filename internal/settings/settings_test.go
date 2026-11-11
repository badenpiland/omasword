package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"omasword/internal/esv"
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

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func TestLegacyKeyMovesToKeyringAndLeavesTheFile(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "omasword")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\n  \"selected_translation\": \"ESV\",\n  \"current_book\": 43,\n  \"current_chapter\": 16,\n  \"esv_api_key\": \"test-key-not-real\"\n}\n")
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyNotice != esv.MigrationOKNotice {
		t.Fatalf("notice = %q", got.KeyNotice)
	}
	if got.LegacyESVAPIKey != "" {
		t.Fatal("legacy key still on the loaded settings")
	}
	if esv.Resolve("") != "test-key-not-real" {
		t.Fatal("key was not stored in the keyring")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "test-key-not-real") || strings.Contains(string(data), "esv_api_key") {
		t.Fatalf("config still has a plaintext key: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("config mode = %o, want 644", info.Mode().Perm())
	}
}

func TestLegacyKeyIsScrubbedWhenTheKeyringRefusesIt(t *testing.T) {
	keyring.MockInitWithError(os.ErrPermission)
	t.Cleanup(func() { keyring.MockInit() })
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "omasword")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{"selected_translation":"ESV","esv_api_key":"test-key-not-real"}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyNotice != esv.MigrationFailedNotice {
		t.Fatalf("notice = %q", got.KeyNotice)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "test-key-not-real") || strings.Contains(string(data), "esv_api_key") {
		t.Fatalf("refused keyring still left the key in config: %s", data)
	}
	if esv.Resolve("") != "" {
		t.Fatal("a refused keyring still provided a key")
	}
}

func TestSaveDropsALegacyKeyWithoutWritingIt(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	err := Save(Settings{
		SelectedTranslation: "BSB",
		ESVAPIKeyCmd:        "pass show esv",
		LegacyESVAPIKey:     "test-key-not-real",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "test-key-not-real") || strings.Contains(string(data), "esv_api_key\"") {
		t.Fatalf("Save wrote a key: %s", data)
	}
	if !strings.Contains(string(data), "pass show esv") {
		t.Fatalf("key command was dropped: %s", data)
	}
}
