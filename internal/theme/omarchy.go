package theme

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

// OmarchyName is the display name of the theme that follows the active
// Omarchy desktop theme. It is also the value persisted in settings.
const OmarchyName = "Omarchy"

var (
	omarchyMu      sync.RWMutex
	omarchyTheme   *Theme
	omarchyRaw     []byte
	omarchyDirFunc = defaultOmarchyThemeDir
)

// fallbackColors is Omarchy's default theme (Tokyo Night), used when the
// Omarchy theme can't be read (Omarchy not installed, or mid-switch on
// first launch) so the app always looks like stock Omarchy.
const fallbackColors = `
accent = "#7aa2f7"
selection = "#292e42"
muted = "#414868"
background = "#1a1b26"
dark_background = "#13141c"
darker_background = "#0e0e14"
lighter_background = "#24283b"
foreground = "#a9b1d6"
dark_foreground = "#565f89"
light_foreground = "#b4bee6"
red = "#f7768e"
yellow = "#e0af68"
green = "#9ece6a"
blue = "#7aa2f7"
`

var fallbackTheme = func() Theme {
	th, err := parseOmarchyColors([]byte(fallbackColors))
	if err != nil {
		panic("theme: invalid fallback colors: " + err.Error())
	}
	return th
}()

// Current returns the active Omarchy theme, or Omarchy's default theme
// if none has been loaded.
func Current() Theme {
	if th, ok := Omarchy(); ok {
		return th
	}
	return fallbackTheme
}

// OmarchyThemeDir returns the directory holding the active Omarchy theme,
// or "" when Omarchy isn't installed.
//
// Current Omarchy keeps it in $XDG_STATE_HOME/omarchy/current/theme;
// older releases used ~/.config/omarchy/current/theme (a symlink).
func OmarchyThemeDir() string {
	return omarchyDirFunc()
}

func defaultOmarchyThemeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	for _, dir := range []string{
		filepath.Join(stateHome, "omarchy", "current", "theme"),
		filepath.Join(configHome, "omarchy", "current", "theme"),
	} {
		if _, err := os.Stat(filepath.Join(dir, "colors.toml")); err == nil {
			return dir
		}
	}
	return ""
}

// OmarchyAvailable reports whether an Omarchy theme has been loaded.
func OmarchyAvailable() bool {
	omarchyMu.RLock()
	defer omarchyMu.RUnlock()
	return omarchyTheme != nil
}

// Omarchy returns the most recently loaded Omarchy theme.
func Omarchy() (Theme, bool) {
	omarchyMu.RLock()
	defer omarchyMu.RUnlock()
	if omarchyTheme == nil {
		return Theme{}, false
	}
	return *omarchyTheme, true
}

// RefreshOmarchy re-reads the active Omarchy theme and reports whether its
// colors changed since the last call. It is cheap enough to poll: the file
// is a few hundred bytes and is only re-parsed when its content differs.
func RefreshOmarchy() bool {
	dir := OmarchyThemeDir()
	if dir == "" {
		omarchyMu.Lock()
		defer omarchyMu.Unlock()
		changed := omarchyTheme != nil
		omarchyTheme, omarchyRaw = nil, nil
		return changed
	}

	raw, err := os.ReadFile(filepath.Join(dir, "colors.toml"))
	if err != nil {
		// Omarchy swaps the theme directory while switching; keep the
		// last good theme instead of flickering back to a default.
		return false
	}

	omarchyMu.RLock()
	same := omarchyTheme != nil && bytes.Equal(raw, omarchyRaw)
	omarchyMu.RUnlock()
	if same {
		return false
	}

	th, err := parseOmarchyColors(raw)
	if err != nil {
		return false
	}

	omarchyMu.Lock()
	defer omarchyMu.Unlock()
	omarchyTheme, omarchyRaw = &th, raw
	return true
}

// LoadOmarchyFrom parses the colors.toml in dir into a Theme.
func LoadOmarchyFrom(dir string) (Theme, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "colors.toml"))
	if err != nil {
		return Theme{}, err
	}
	return parseOmarchyColors(raw)
}

// parseOmarchyColors maps an Omarchy colors.toml onto a Theme. It accepts
// both the current named-color format (foreground, red, accent, ...) and
// the older terminal palette format (color0..color15, selection_background).
func parseOmarchyColors(raw []byte) (Theme, error) {
	kv := parseFlatTOML(raw)

	pick := func(keys ...string) (color.Color, bool) {
		for _, k := range keys {
			if v, ok := kv[k]; ok {
				if c, err := parseHex(v); err == nil {
					return c, true
				}
			}
		}
		return nil, false
	}

	bg, ok := pick("background", "color0")
	if !ok {
		return Theme{}, errors.New("omarchy colors.toml has no background color")
	}
	fg, ok := pick("foreground", "color7", "color15")
	if !ok {
		return Theme{}, errors.New("omarchy colors.toml has no foreground color")
	}

	or := func(fallback color.Color, keys ...string) color.Color {
		if c, ok := pick(keys...); ok {
			return c
		}
		return fallback
	}

	accent := or(fg, "accent", "blue", "color4")
	muted := or(fg, "muted", "dark_foreground", "color8")
	highlight := or(shift(bg, 0.12), "selection", "selection_background", "lighter_background")

	return Theme{
		Name:         OmarchyName,
		Primary:      fg,
		Secondary:    or(fg, "dark_foreground", "light_foreground", "color7"),
		Accent:       accent,
		Muted:        muted,
		Error:        or(fg, "red", "color1"),
		Success:      or(fg, "green", "color2"),
		Warning:      or(fg, "yellow", "color3"),
		Border:       or(highlight, "lighter_background", "selection", "selection_background", "color8"),
		BorderActive: accent,
		Background:   bg,
		Highlight:    highlight,
		Shadow:       or(shift(bg, -0.4), "darker_background", "dark_background"),
	}, nil
}

// parseFlatTOML reads `key = "value"` pairs, which is all colors.toml uses.
// Tables, comments and non-string values are ignored.
func parseFlatTOML(raw []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		val = strings.Trim(val, `"'`)
		out[key] = val
	}
	return out
}

func parseHex(s string) (color.Color, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	s = strings.TrimPrefix(s, "0x")
	if len(s) == 8 { // drop alpha
		s = s[:6]
	}
	if len(s) != 6 {
		return nil, fmt.Errorf("invalid hex color %q", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return nil, fmt.Errorf("invalid hex color %q: %w", s, err)
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b)), nil
}

// shift lightens (amount > 0) or darkens (amount < 0) a color.
func shift(c color.Color, amount float64) color.Color {
	r, g, b, _ := c.RGBA()
	adj := func(v uint32) uint8 {
		f := float64(v >> 8)
		if amount >= 0 {
			f += (255 - f) * amount
		} else {
			f *= 1 + amount
		}
		return uint8(f)
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", adj(r), adj(g), adj(b)))
}
