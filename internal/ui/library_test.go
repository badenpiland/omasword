package ui

import (
	"strings"
	"testing"

	"omasword/internal/api"
	"omasword/internal/settings"
	"omasword/internal/translations"
)

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
	if len(got) != 4 {
		t.Fatalf("got %d translations, want BSB, KJV, WEB, YLT", len(got))
	}
	want := []string{"BSB", "KJV", "WEB", "YLT"}
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
	if strings.Contains(view, "ESV") {
		t.Fatal("picker should not list ESV")
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
