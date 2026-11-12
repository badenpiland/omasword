# TODO

Open ideas, roughly ordered by impact. Completed items and the old
multi-theme backlog were dropped when theming moved to Omarchy only.

## Distribution

- [ ] **Omarchy package** — add a recipe to `omacom/omarchy-pkgs` so it
  installs and updates through the Omarchy menu
  (`omarchy-pkg-add omasword`).

## ESV offline (within Crossway's 500-verse cap)

- [x] **Save ahead** — a key that saves the next chapters from where you
  are, up to the cap, before going offline.
- [x] **Pin chapters** — keep chosen chapters when older ones are dropped;
  they still count toward the cap.
- [x] **BSB fallback** — when offline and an ESV chapter isn't saved, show
  the BSB for that chapter, clearly labeled, and switch back once online.

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
