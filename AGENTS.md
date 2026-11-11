# AGENTS.md

Hard constraints for people and coding agents changing omasword. The
reader is GPL-2.0-or-later, a fork of [kmf/sword-tui](https://github.com/kmf/sword-tui).
Do not merge, tag, release, or push to that upstream from a task that
did not ask for it.

## Licensing

These rules outrank a feature request, a test fixture, and a "temporary"
shortcut. Do not fetch, cache, log, or commit copyrighted Bible text
except as this section allows.

- Built-in text comes from the bolls.life API and is limited to the
  public-domain allowlist in `internal/translations` (BSB, KJV, WEB, ASV,
  YLT, Geneva, Douay-Rheims). The default is the Berean Standard Bible.
  Do not add ESV, NLT, CSB, NIV, NASB, NKJV, or any other copyrighted
  translation to that list, to `internal/api`, or to the offline
  downloader in `internal/cache`. bolls.life hosts those files; that is
  not permission to redistribute them.
- The ESV is allowed only through `internal/esv`, with the user's own
  key from <https://api.esv.org/>. See [Secrets](#secrets).
- ESV is online-only. Do not bulk-download it. Any local cache stays
  within 500 verses and half of any one book, whichever is less, and
  evicts past that. One screen, including this CLI, shows no more than
  that. A clipped passage is marked with an ellipsis.
- Wherever ESV text is shown (the reader, a copy, search results, and
  CLI output), include Crossway's copyright notice and a link to
  <https://www.esv.org/>.
- Stay inside Crossway's rate limits: 60 requests per minute, 1,000 per
  hour, 5,000 per day. Back off on HTTP 429 instead of retrying in a loop.
- The CSB's terms (at most 1,000 verses and no complete book) are not
  workable for a full reader. Do not add it.
- Darby is public domain but bolls.life does not host it. A public-domain
  text that is not hosted there can be added later from the publisher's
  own files, not from a mirror that also hosts copyrighted Bibles.

## Secrets

No plaintext secrets in files, logs, tests, or commits.

- The ESV API key is stored in the OS keyring (gnome-keyring or KeePassXC
  via the Secret Service). Do not put it back in `config.json`, a cache
  file, a fixture, a log, `--json` output, a panic, or an error string.
- `esv_api_key_cmd` may be stored. It is a command. Its stdout is the key
  and must not be written, logged, or attached to an error.
- `OMASWORD_ESV_API_KEY` exists for one process (CI or a single command).
  Never persist it, and do not tell users to put it in a shell startup
  file. Tests that set it use an obvious placeholder and do not print it.
- An old `esv_api_key` field is migrated into the keyring and deleted from
  the file. If the keyring refuses it, the field is still deleted.

## Build, test, and run

Go 1.26 or newer.

```bash
go build -o omasword ./cmd/omasword
go vet ./...
go test ./...
./omasword --version
./omasword                  # the terminal reader
./omasword --help           # non-interactive mode
```

`gofmt -w` changed Go files before committing. Stage paths explicitly
(`git add <paths>`), never `git add -A`. Commit messages are
[Conventional Commits](https://www.conventionalcommits.org/).

## Architecture

- `cmd/omasword`: process entry. No arguments starts the Bubble Tea
  reader. `--version` prints the version. Anything else in `internal/cli`
  prints a passage or a search and exits.
- `internal/translations`: the public-domain allowlist, picker order,
  one-line flavor text, and the BSB default.
- `internal/api`: bolls.life HTTP client. Responses are untrusted.
  `SanitizeText` strips terminal control characters. `ValidTranslationName`
  gates names used in URLs and paths. Requests are refused unless the
  translation is on the allowlist.
- `internal/cache`: offline downloads of allowlisted translations only,
  size-capped, written via a temp file and rename, under
  `~/.cache/omasword/translations`.
- `internal/esv`: Crossway's passage API. Verse cache and rate-limit
  journal live under `~/.cache/omasword` and do not contain the key.
- `internal/settings`: `~/.config/omasword/config.json` (translation,
  book, chapter, optional `esv_api_key_cmd`), written atomically. The
  API key is not in this file. Migrates a sword-tui config on first run.
- `internal/ui`: the Bubble Tea model. The reading pane is pre-rendered
  into `m.content`; color or width changes call `rerenderContent`. Keys
  reach a text input only when `typingInInput` is true. Book IDs are
  1–66 and are not slice indexes.
- `internal/theme`: colors from the active Omarchy theme, polled once a
  second, with Tokyo Night as the fallback.

## CLI for scripts and agents

```bash
omasword --passage "John 3:16"
omasword "Psalms 23" --translation WEB --json
omasword --search love --json
```

`--translation` defaults to the saved choice when that choice is still
allowed, otherwise BSB. Names look like `John 3:16`, `John 3:16-18`,
`1 John 1:9`, `Song of Solomon 2`, or `Psalm 23`.

`--json` prints one object to stdout. On success the passage object has
`translation`, `reference`, `book`, `chapter`, and `verses`
(`book`, `chapter`, `verse`, `text`). ESV objects also have `notice`,
`link`, and `clipped` when the per-page cap cut the passage. Search
objects have `translation`, `query`, `total`, and `results`. Errors go
to stderr as text, not JSON.

Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Printed the passage or the search result |
| 1 | Usage: missing flag, bad reference, both passage and search |
| 2 | Translation is not allowed, or ESV was requested with no API key |
| 3 | The text could not be fetched (network, rate limit, rejected key) |

A search with zero hits is exit 0. Do not treat stderr text as stable;
branch on the exit code.
