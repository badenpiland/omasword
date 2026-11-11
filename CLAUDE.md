# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

omasword is a terminal Bible reader for Omarchy, written in Go on the charm
v2 stack (Bubble Tea, Bubbles, Lip Gloss). Built-in Bible text comes from
the bolls.life API and is limited to public-domain translations, defaulting
to the Berean Standard Bible (BSB), with optional offline downloads of those
same texts. Copyrighted translations are not fetched or cached. It is a fork of
[kmf/sword-tui](https://github.com/kmf/sword-tui) (GPL-2.0-or-later).

## Build and test

```bash
go build -o omasword ./cmd/omasword
go vet ./...
go test ./...
```

## Layout

- `cmd/omasword/main.go`: flags (`--version`), cache setup, starts the program.
- `internal/translations`: public-domain allowlist (BSB, KJV, WEB, ASV, YLT,
 Geneva, Douay-Rheims). The bolls client and the offline cache both refuse
 anything else.
- `internal/esv`: optional ESV via the user's own Crossway API key, stored
 in the OS keyring (or read from `esv_api_key_cmd`). Never write the key
 into a file, log, error, or commit. `OMASWORD_ESV_API_KEY` is a one-shot
 process environment value and is not persisted. Online only. The local
 cache stays within 500 verses and half of any book, and the copyright
 notice plus an esv.org link are shown with the text. Do not bulk-download
 the ESV.
- `internal/api`: bolls.life HTTP client. Everything it returns is untrusted:
 text goes through `SanitizeText` (strips terminal control characters) and
 translation names through `ValidTranslationName` before use in URLs or paths.
 Requests are limited to the allowlist.
- `internal/cache`: offline translations in `~/.cache/omasword/translations`;
  downloads are size-capped and written via temp file + rename.
- `internal/settings`: `~/.config/omasword/config.json` (translation, book,
  chapter), written atomically on every chapter load and on quit. Migrates
  from the old `sword-tui` directories on first run.
- `internal/theme`: colors come only from the active Omarchy theme
  (`colors.toml` under `$XDG_STATE_HOME/omarchy/current/theme`, or the older
  `~/.config/omarchy/current/theme`), polled once a second; Tokyo Night is the
  fallback without Omarchy.
- `internal/ui/model.go`: the Bubble Tea model (Update/View, panes, overlays).
- `assets/`: launcher icon (`make-icon.py` regenerates it) and social preview.

## Conventions

- Book IDs are 1–66 and are not array indices; look books up by `BookID`.
- The reading pane is pre-rendered into `m.content`; anything that changes
  its colors or width must call `rerenderContent()` (theme changes go through
  `setTheme`).
- Keys only reach text inputs when `typingInInput()` is true; keep global
  shortcuts from firing while typing.
- Conventional commits; stage files explicitly (never `git add -A`).
