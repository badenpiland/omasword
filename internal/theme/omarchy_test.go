package theme

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return string([]byte{
		"0123456789abcdef"[r>>12], "0123456789abcdef"[(r>>8)&0xf],
		"0123456789abcdef"[g>>12], "0123456789abcdef"[(g>>8)&0xf],
		"0123456789abcdef"[b>>12], "0123456789abcdef"[(b>>8)&0xf],
	})
}

func TestParseOmarchyNamedFormat(t *testing.T) {
	raw := []byte(`mode = "dark"

accent = "#89b4fa"
selection = "#45475a"
muted = "#585b70"
background = "#1e1e2e"
darker_background = "#101019"
lighter_background = "#313244"
foreground = "#cdd6f4"
dark_foreground = "#6c7086"
red = "#f38ba8"
yellow = "#f9e2af"
green = "#a6e3a1"
`)
	th, err := parseOmarchyColors(raw)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]struct {
		got  color.Color
		want string
	}{
		"Primary":      {th.Primary, "cdd6f4"},
		"Secondary":    {th.Secondary, "6c7086"},
		"Accent":       {th.Accent, "89b4fa"},
		"BorderActive": {th.BorderActive, "89b4fa"},
		"Muted":        {th.Muted, "585b70"},
		"Error":        {th.Error, "f38ba8"},
		"Success":      {th.Success, "a6e3a1"},
		"Warning":      {th.Warning, "f9e2af"},
		"Background":   {th.Background, "1e1e2e"},
		"Highlight":    {th.Highlight, "45475a"},
		"Border":       {th.Border, "313244"},
		"Shadow":       {th.Shadow, "101019"},
	}
	for field, c := range checks {
		if c.got == nil {
			continue
		}
		if got := hexOf(c.got); got != c.want {
			t.Errorf("%s = %s, want %s", field, got, c.want)
		}
	}
	if th.Name != OmarchyName {
		t.Errorf("Name = %q, want %q", th.Name, OmarchyName)
	}
}

func TestParseOmarchyPaletteFormat(t *testing.T) {
	raw := []byte(`accent = "#7aa2f7"
foreground = "#a9b1d6"
background = "#1a1b26"
selection_background = "#7aa2f7"
color0 = "#32344a"
color1 = "#f7768e"
color2 = "#9ece6a"
color3 = "#e0af68"
color8 = "#444b6a"
`)
	th, err := parseOmarchyColors(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := hexOf(th.Error); got != "f7768e" {
		t.Errorf("Error = %s", got)
	}
	if got := hexOf(th.Success); got != "9ece6a" {
		t.Errorf("Success = %s", got)
	}
	if got := hexOf(th.Muted); got != "444b6a" {
		t.Errorf("Muted = %s", got)
	}
	if got := hexOf(th.Highlight); got != "7aa2f7" {
		t.Errorf("Highlight = %s", got)
	}
}

func TestParseOmarchyRequiresBackground(t *testing.T) {
	if _, err := parseOmarchyColors([]byte(`foreground = "#ffffff"`)); err == nil {
		t.Fatal("expected error without background")
	}
}

func TestRefreshOmarchyDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	orig := omarchyDirFunc
	omarchyDirFunc = func() string {
		if _, err := os.Stat(filepath.Join(dir, "colors.toml")); err != nil {
			return ""
		}
		return dir
	}
	t.Cleanup(func() {
		omarchyDirFunc = orig
		omarchyTheme, omarchyRaw = nil, nil
	})

	if RefreshOmarchy() || OmarchyAvailable() {
		t.Fatal("no theme should be loaded without colors.toml")
	}

	write := func(bg string) {
		data := `foreground = "#ffffff"` + "\n" + `background = "` + bg + `"` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "colors.toml"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("#000000")
	if !RefreshOmarchy() {
		t.Fatal("first load should report a change")
	}
	if RefreshOmarchy() {
		t.Fatal("unchanged file should not report a change")
	}
	if got := hexOf(Current().Background); got != "000000" {
		t.Fatalf("Current() should be the loaded theme, background = %s", got)
	}

	write("#112233")
	if !RefreshOmarchy() {
		t.Fatal("new colors should report a change")
	}
	th, _ := Omarchy()
	if got := hexOf(th.Background); got != "112233" {
		t.Fatalf("Background = %s, want 112233", got)
	}

	if err := os.Remove(filepath.Join(dir, "colors.toml")); err != nil {
		t.Fatal(err)
	}
	if !RefreshOmarchy() || OmarchyAvailable() {
		t.Fatal("removing Omarchy should drop the theme")
	}
	if got := hexOf(Current().Background); got != "1a1b26" {
		t.Fatalf("without Omarchy, Current() should fall back to Tokyo Night, background = %s", got)
	}
}
