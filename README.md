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
- **Multiple Translations**: Switch between public-domain English translations (Berean Standard Bible by default)
- **Side-by-Side Comparison**: Per-column translation pickers for parallel reading
- **Verse Lookup**: Jump directly to any book, chapter, and verse
- **Offline Cache**: Download public-domain translations, with a byte-level progress bar
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

Tagged releases also attach prebuilt Linux binaries (amd64 and arm64) on the
[Releases page](https://github.com/badenpiland/omasword/releases).

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
- `v` - Browse to a passage: Books → Chapters → Verses columns (`/` filters a column, `Enter` opens the verse)
- `c` - Comparison view (side-by-side translations)
- `t` - Translation picker (`a` in that picker sets or clears an ESV API key)
- `,` / `.` - Previous / next ESV page, when a chapter is longer than Crossway's per-page limit
- `d` - Cache manager (`x` deletes a cached translation here)
- `r` - Return to reader from any overlay
- `y` - Yank/copy selected verse
- `?` - About
- `Enter` - Select item
- `esc` - Close overlay / cancel
- `q`, `Ctrl-C` - Quit

## Translations and licensing

The default translation is the **Berean Standard Bible (BSB)**. [bolls.life](https://bolls.life/api/) serves it, and it has been public domain since 2023: modern, word-for-word English, and the closest built-in text to the reading level of recent evangelical translations. The picker also offers the other public-domain texts that API hosts — KJV, WEB, ASV, YLT, the Geneva Bible, and the Douay-Rheims — each with a one-line description of how it reads. Darby is public domain, but bolls.life does not host it, so it is not listed. A public-domain text that is not hosted there can be added later from the publisher's own files.

Copyrighted translations are not fetched in bulk or cached as a whole Bible. That includes the NLT (Tyndale) and the CSB (Holman). A saved config that still names one of them opens the BSB instead of failing.

The ESV (Crossway) is the exception, and only with your own key. Crossway allows non-commercial use through [api.esv.org](https://api.esv.org/), not through a third-party download. In the translation picker, choose ESV and paste the key when asked, or press `a` there to replace it. Submitting an empty key deletes it. The key is stored in the system keyring (gnome-keyring or KeePassXC on Omarchy), not in a file. If no keyring is available, omasword refuses to save it and tells you how to unlock one.

To use your own password manager instead, set `esv_api_key_cmd` in `~/.config/omasword/config.json` to a command whose first line of stdout is the key, for example `"pass show esv"`. That output is not written to disk.

`OMASWORD_ESV_API_KEY` is only for one process, such as CI or a single command. omasword never writes it anywhere. Do not put it in a shell startup file; that would keep the key in plain text. Use the keyring or `esv_api_key_cmd` for anything that should stick around.

ESV text is fetched from Crossway as you read. omasword stores at most 500 verses, and at most half of any book, and shows the same limit on one screen. A longer passage (Obadiah, for example) is split across pages with `,` and `.`. The Crossway copyright notice and a link to [esv.org](https://www.esv.org/) appear wherever ESV text is shown, including copied verses. Requests stay inside Crossway's rate limits: 60 per minute, 1,000 per hour, and 5,000 per day.

## API

Built-in texts come from the [bolls.life API](https://bolls.life/api/).

## License

GPL-2.0-or-later. omasword is a modified version of
[sword-tui](https://github.com/kmf/sword-tui) by Karl Fischer, which is
licensed the same way. Changes since the fork are recorded in this
repository's git history.
