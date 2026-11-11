package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/settings"
	"omasword/internal/translations"

	tea "charm.land/bubbletea/v2"
	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func TestPresentTranslationsDropsCopyrighted(t *testing.T) {
	in := []api.Translation{
		{ShortName: "YLT", FullName: "Young's Literal Translation (1898)"},
		{ShortName: "ESV", FullName: "English Standard Version"},
		{ShortName: "NLT", FullName: "New Living Translation"},
		{ShortName: "BSB", FullName: "The Holy Bible, Berean Standard Bible"},
		{ShortName: "KJV", FullName: "King James Version 1769"},
		{ShortName: "CSB17", FullName: "Christian Standard Bible"},
		{ShortName: "WEB", FullName: "World English Bible"},
	}
	got := presentTranslations(in)
	if len(got) != 5 {
		t.Fatalf("got %d translations, want BSB, KJV, WEB, YLT, ESV", len(got))
	}
	want := []string{"BSB", "KJV", "WEB", "YLT", "ESV"}
	for i, name := range want {
		if got[i].ShortName != name {
			t.Fatalf("position %d = %s, want %s (catalog order)", i, got[i].ShortName, name)
		}
	}
	if got[0].FullName != "Berean Standard Bible" {
		t.Fatalf("BSB display name = %q", got[0].FullName)
	}
}

func TestSavedCopyrightedTranslationFallsBackToBSB(t *testing.T) {
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	for _, saved := range []string{"ESV", "NLT", "CSB17", "NASB"} {
		t.Run(saved, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			if err := settings.Save(settings.Settings{SelectedTranslation: saved, CurrentBook: 43, CurrentChapter: 16}); err != nil {
				t.Fatal(err)
			}
			m := NewModel()
			if m.selectedTranslation != translations.Default {
				t.Fatalf("selected %s, want %s", m.selectedTranslation, translations.Default)
			}
			if m.currentBook != 43 || m.currentChapter != 16 {
				t.Fatalf("reading position = %d:%d, want 43:16", m.currentBook, m.currentChapter)
			}
			got, err := settings.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got.SelectedTranslation != translations.Default {
				t.Fatalf("rewritten config translation = %s", got.SelectedTranslation)
			}
		})
	}
}

func TestSavedPublicDomainTranslationIsKept(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := settings.Save(settings.Settings{SelectedTranslation: "WEB", CurrentBook: 19, CurrentChapter: 23}); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	if m.selectedTranslation != "WEB" || m.currentBook != 19 || m.currentChapter != 23 {
		t.Fatalf("model = %s %d:%d, want WEB 19:23", m.selectedTranslation, m.currentBook, m.currentChapter)
	}
}

func TestNewModelComparisonIsPublicDomain(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	if strings.Join(m.comparisonTranslations, ",") != "BSB,KJV,WEB" {
		t.Fatalf("comparison = %v", m.comparisonTranslations)
	}
}

func TestTranslationPickerShowsFlavor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	m.mode = modeTranslationSelect
	m.translations = presentTranslations([]api.Translation{
		{ShortName: "BSB"},
		{ShortName: "WEB"},
		{ShortName: "ESV"},
	})
	m.width = 100
	view := m.renderTranslationSelect()
	if !strings.Contains(view, "uses Yahweh") {
		t.Fatal("picker should describe WEB as using Yahweh")
	}
	if !strings.Contains(view, "api.esv.org") {
		t.Fatal("picker should offer ESV with a pointer to api.esv.org")
	}
	if strings.Contains(view, "NLT") {
		t.Fatal("picker should not list NLT")
	}
	if !strings.Contains(view, "Berean Standard Bible") {
		t.Fatal("picker should name the BSB")
	}
}

func TestFlavorLineClickSelectsThatTranslation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewModel()
	m.mode = modeTranslationSelect
	m.translations = []api.Translation{
		{ShortName: "BSB", FullName: "Berean Standard Bible"},
		{ShortName: "KJV", FullName: "King James Version"},
	}
	m.selectedTranslation = "BSB"
	// Item 1 starts on row 2; its flavor line is row 3.
	m.overlayClick(3)
	if m.selectedTranslation != "KJV" || m.mode != modeReader {
		t.Fatalf("after flavor click: translation %s mode %d", m.selectedTranslation, m.mode)
	}
}

func TestSavedESVStaysWhenAKeyIsConfigured(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := esv.SaveKey("test-key-not-real"); err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(settings.Settings{SelectedTranslation: "ESV", CurrentBook: 43, CurrentChapter: 16}); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	if m.selectedTranslation != esv.ID || !m.esvClient.HasKey() {
		t.Fatalf("translation %s key %v", m.selectedTranslation, m.esvClient != nil && m.esvClient.HasKey())
	}
	if m.currentBook != 43 || m.currentChapter != 16 {
		t.Fatalf("position %d:%d", m.currentBook, m.currentChapter)
	}
}

func TestEnvESVKeyIsNotWrittenToConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("OMASWORD_ESV_API_KEY", "from-env-not-a-real-key")
	if err := settings.Save(settings.Settings{SelectedTranslation: "ESV", CurrentBook: 43, CurrentChapter: 16}); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	if m.selectedTranslation != "ESV" || !m.esvClient.HasKey() {
		t.Fatalf("translation %s hasKey %v", m.selectedTranslation, m.esvClient != nil && m.esvClient.HasKey())
	}
	m.Update(chapterLoadedMsg{verses: []api.Verse{{Verse: 16, Text: "For God so loved"}}})
	data, err := os.ReadFile(filepath.Join(dir, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "from-env") {
		t.Fatalf("environment key was written to config: %s", data)
	}
}

func TestESVWizardStoresAndRemovesTheKey(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewModel()
	m.translations = presentTranslations(nil)
	for i, tr := range m.translations {
		if tr.ShortName == esv.ID {
			m.translationSelected = i
		}
	}
	m.mode = modeTranslationSelect
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil {
		t.Fatal("selecting ESV without a key should not fetch")
	}
	if m.mode != modeESVKey {
		t.Fatalf("mode %d, want the key wizard", m.mode)
	}
	m.esvKeyInput.SetValue("test-key-not-real")
	next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("saving a key and selecting ESV should load the chapter")
	}
	if !m.esvClient.HasKey() || m.selectedTranslation != esv.ID {
		t.Fatalf("translation %s hasKey %v", m.selectedTranslation, m.esvClient.HasKey())
	}
	info, err := os.Stat(filepath.Join(dir, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("config mode = %o, want 644 (the key is not in this file)", info.Mode().Perm())
	}
	saved, err := os.ReadFile(filepath.Join(dir, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "test-key-not-real") || strings.Contains(string(saved), "esv_api_key\"") {
		t.Fatalf("config contains the key: %s", saved)
	}
	if esv.Resolve("") != "test-key-not-real" {
		t.Fatal("wizard did not store the key in the keyring")
	}

	m.mode = modeESVKey
	m.esvKeyThenSelect = false
	m.esvKeyInput.SetValue("  ")
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if m.esvClient.HasKey() || m.selectedTranslation != translations.Default {
		t.Fatalf("after removal: translation %s hasKey %v", m.selectedTranslation, m.esvClient.HasKey())
	}
	info, err = os.Stat(filepath.Join(dir, "omasword", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("config mode = %o, want 644 after removal", info.Mode().Perm())
	}
	if esv.Resolve("") != "" {
		t.Fatal("removed key is still stored")
	}
}

func TestWizardRefusesToSaveWithoutAKeyring(t *testing.T) {
	keyring.MockInitWithError(os.ErrPermission)
	t.Cleanup(func() { keyring.MockInit() })
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewModel()
	m.mode = modeESVKey
	m.esvKeyThenSelect = true
	m.esvKeyInput.SetValue("test-key-not-real")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil || m.mode != modeESVKey {
		t.Fatalf("mode %d cmd %v; a missing keyring should keep the wizard open", m.mode, cmd != nil)
	}
	if m.err == nil || !strings.Contains(m.err.Error(), "keyring") {
		t.Fatalf("err = %v", m.err)
	}
	if strings.Contains(m.err.Error(), "test-key-not-real") {
		t.Fatal("the refusal includes the key")
	}
	if m.esvClient != nil && m.esvClient.HasKey() {
		t.Fatal("the unsaved key was kept")
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "omasword")); len(entries) > 0 {
		data, _ := os.ReadFile(filepath.Join(dir, "omasword", "config.json"))
		if strings.Contains(string(data), "test-key-not-real") {
			t.Fatalf("refused key was written to config: %s", data)
		}
	}
}

func TestESVChapterShowsCopyrightAndLink(t *testing.T) {
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewModel()
	m.selectedTranslation = esv.ID
	m.esvNotice = esv.CopyrightNotice
	m.esvFirst, m.esvLast, m.esvTotal = 1, 10, 21
	out := m.formatChapter([]api.Verse{{Verse: 1, Text: "The vision of Obadiah."}}, "Obadiah", 1, 72, 0, 0)
	if !strings.Contains(out, "The vision of Obadiah.") {
		t.Fatal("verse text missing")
	}
	if !strings.Contains(out, "Crossway") || !strings.Contains(out, "esv.org") {
		t.Fatalf("copyright notice missing:\n%s", out)
	}
	if !strings.Contains(out, "Showing verses 1") {
		t.Fatal("clipped chapter should say which verses are on the page")
	}
}

func TestKeyIsNotLookedUpUntilAPickerOpens(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OMASWORD_ESV_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := esv.SaveKey("test-key-not-real"); err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(settings.Settings{SelectedTranslation: "BSB", CurrentBook: 43, CurrentChapter: 3}); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	if m.esvKeyChecked || m.esvClient.HasKey() {
		t.Fatal("key was looked up at launch while reading BSB")
	}
	next, cmd, _ := m.handleKey(tea.KeyPressMsg{Code: 't', Text: "t"})
	if cmd == nil {
		t.Fatal("opening the picker did not look up the key")
	}
	updated, _ := next.Update(cmd())
	if !updated.(Model).esvClient.HasKey() {
		t.Fatal("key from the keyring was not picked up")
	}
	reopened := updated.(Model)
	reopened.mode = modeReader
	if _, again, _ := reopened.handleKey(tea.KeyPressMsg{Code: 't', Text: "t"}); again != nil {
		t.Fatal("key was looked up a second time")
	}
}
