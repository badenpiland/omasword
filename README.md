<p align="center">
  <img src="assets/omasword.png" width="128" height="128" alt="omasword icon: a pixel sword on a Bible">
</p>

<h1 align="center">omasword</h1>

<p align="center">A terminal Bible reader for <a href="https://omarchy.org">Omarchy</a>, built with Go, Bubble Tea and Lip Gloss.</p>

omasword is a fork of [sword-tui](https://github.com/kmf/sword-tui) by Karl
Fischer, renamed and adapted for [Omarchy](https://omarchy.org). It is
distributed under the same GPL-2.0-or-later license; see [License](#license).

## Features

### Navigation
- **Multi-Pane Shell**: Permanent two-pane layout with rounded borders and drop-shadow overlays
- **Books & Translations Sidebars**: Independent pickers on `[` and `]`
- **Smart Verse References**: Parses `rom8:8`, `rom 8 8`, `1 john 3 16`, and similar
- **Mouse Support**: Click, drag, hover, and wheel for navigation and verse selection
- **Keyboard-Driven**: Full keyboard navigation with vim-like bindings

### Bible Access
- **Multiple Translations**: Switch between English translations on the fly
- **Side-by-Side Comparison**: Per-column translation pickers for parallel reading
- **Verse Lookup**: Jump directly to any book, chapter, and verse
- **Offline Cache**: Automatic caching with a real byte-level progress bar for downloads
- **Persistent State**: Translation and last-read position survive restarts

### User Interface
- **Modern Terminal UI**: Built on the charm v2 stack (bubbletea, lipgloss)
- **Omarchy Theme Sync**: Colors come from your active [Omarchy](https://omarchy.org) theme and follow `omarchy theme set` live. Without Omarchy, it uses Omarchy's default Tokyo Night colors
- **Sticky Chapter Header**: Morphs into a scroll indicator as you read
- **Viewport-Based Text Wrapping**: Prevents text from rendering off-screen
- **Visual Depth Effects**: Dimming and shadow effects for focused elements
- **Status Bar**: Displays current version and build information

### Productivity
- **Copy/Yank**: Copy selected verse(s) to clipboard
- **Click-and-Drag Range Selection**: Select multi-verse ranges with the mouse
- **Search/Filter**: Filter verses with `/`
- **Quick Navigation**: `n`/`p` step between chapters; sidebars jump between books and translations

## Installation

### From Source

You need `git` and Go 1.26 or newer. On Omarchy (Arch):

```bash
sudo pacman -S --needed git go
```

Then clone, build, and install to `~/.local/bin` (already on your `PATH` on
Omarchy):

```bash
git clone https://github.com/badenpiland/omasword.git
cd omasword
go build -o omasword ./cmd/omasword
install -Dm755 omasword ~/.local/bin/omasword
```

To update later, run `git pull` in the `omasword` folder and repeat the last
two commands.

### Add it to the Omarchy app launcher

From the same `omasword` folder, register it as a TUI app with its icon:

```bash
omarchy-tui-install omasword omasword tile "$PWD/assets/omasword.png"
```

It then shows up in the app launcher (`Super + Space`) and opens in its own
tiled terminal window. To remove the launcher entry, run
`omarchy-tui-remove omasword`.

Packaging recipes (`PKGBUILD`, `omasword.spec`, `Formula/omasword.rb`) are
carried over from sword-tui but no omasword packages, tap or COPR repo have
been published yet.

### Coming from sword-tui

On first run omasword copies your sword-tui settings
(`~/.config/sword-tui/config.json`) and downloaded translations
(`~/.cache/sword-tui/translations`) into its own directories. The originals
are left in place, so both programs can be installed side by side.

## Usage

```bash
omasword
```

### Keyboard Shortcuts

- `[` / `]` - Focus books pane / content pane
- `n` / `p` - Next / previous chapter
- `j` / `k`, `↓` / `↑` - Navigate down / up
- `h` / `l`, `←` / `→` - Navigate left / right between panes
- `tab` / `shift+tab` - Cycle focus between panes
- `PgUp` / `PgDn` - Previous / next chapter (same as `p` / `n`)
- `/` - Search by verse reference
- `s` - Word search
- `v` - Toggle Miller-columns picker (Books → Chapters → Verses)
- `c` - Comparison view (side-by-side translations)
- `t` - Translation picker
- `d` - Cache manager (`x` deletes a cached translation here)
- `r` - Return to reader from any overlay
- `y` - Yank/copy selected verse
- `?` - About
- `Enter` - Select item
- `esc` - Close overlay / cancel
- `q`, `Ctrl-C` - Quit

## API

Uses the [bolls.life API](https://bolls.life/api/) for Bible data.

## License

GPL-2.0-or-later. omasword is a modified version of
[sword-tui](https://github.com/kmf/sword-tui) by Karl Fischer, which is
licensed the same way. Changes since the fork are recorded in this
repository's git history.
