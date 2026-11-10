# TODO

Open ideas, roughly ordered by impact. Completed items and the old
multi-theme backlog were dropped when theming moved to Omarchy only.

## Distribution

- [ ] **Omarchy package** — tag a first omasword release, then add a recipe
  to `omacom/omarchy-pkgs` so it installs and updates through the Omarchy
  menu (`omarchy-pkg-add omasword`).

## Reading

- [ ] **Cross-reference hyperlinks** — render refs like `John 3:16` as ANSI
  hyperlinks (lipgloss v2) that jump to the verse in supporting terminals.
- [ ] **Inline footnote popups** — show embedded footnotes in an overlay
  instead of stripping them.
- [ ] **Bookmarks** — a `b` list persisted in `config.json`.
- [ ] **`gg` / `G`** — jump to the top / end of a chapter.

## Input and feedback

- [ ] **Real terminal cursor in inputs** — use the `tea.View` cursor so the
  search boxes show the terminal's cursor.
- [ ] **Auto-growing word-search input** — bubbles v2 `textarea` with
  dynamic height for long phrases.
- [ ] **Toasts** — brief status messages such as "Copied" or "Bookmark
  added".
