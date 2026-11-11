// Package cli is the non-interactive mode used by scripts and agents.
// With no CLI flags, cmd/omasword starts the terminal reader instead.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"omasword/internal/api"
	"omasword/internal/esv"
	"omasword/internal/reference"
	"omasword/internal/settings"
	"omasword/internal/translations"
)

// Exit codes are part of the CLI contract. Agents should branch on these
// and not on error text.
const (
	ExitOK         = 0
	ExitUsage      = 1
	ExitNotAllowed = 2
	ExitFetch      = 3
)

// WantsCLI reports whether args ask for the non-interactive mode.
// --version stays with the TUI binary's existing flag.
func WantsCLI(args []string) bool {
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h" || a == "--json":
			return true
		case strings.HasPrefix(a, "--passage") || strings.HasPrefix(a, "--search") || strings.HasPrefix(a, "--translation"):
			return true
		case a != "" && !strings.HasPrefix(a, "-"):
			return true
		}
	}
	return false
}

// Run prints a passage or search result. It never starts the TUI.
func Run(args []string) int {
	return RunWith(os.Stdout, os.Stderr, args)
}

// RunWith is Run with explicit writers, for tests.
func RunWith(stdout, stderr io.Writer, args []string) int {
	opt, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stderr, "Run omasword --help for usage.")
		return ExitUsage
	}
	if opt.help {
		fmt.Fprint(stdout, helpText)
		return ExitOK
	}
	bolls := api.NewClient()
	esvClient, err := esv.New(opt.key, esv.DefaultCacheDir())
	if err != nil {
		fmt.Fprintln(stderr, redactErr(err, opt.key))
		return ExitFetch
	}
	return execute(stdout, stderr, opt, bolls, esvClient)
}

type options struct {
	help        bool
	json        bool
	passage     string
	search      string
	translation string
	keyCmd      string
	key         string // this process only; never printed or written
	explicit    bool   // --translation was set
}

func parseArgs(args []string) (options, error) {
	var opt options
	fs := flag.NewFlagSet("omasword", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opt.help, "help", false, "")
	fs.BoolVar(&opt.help, "h", false, "")
	fs.BoolVar(&opt.json, "json", false, "")
	fs.StringVar(&opt.passage, "passage", "", "")
	fs.StringVar(&opt.search, "search", "", "")
	fs.StringVar(&opt.translation, "translation", "", "")
	// Flag parsing stops at the first non-flag. Move flags in front so
	// `omasword "Psalm 23:1-3" --translation ESV --json` works.
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return options{}, fmt.Errorf("usage: %s", err)
	}
	opt.explicit = opt.translation != ""
	if opt.passage != "" && fs.NArg() > 0 {
		return options{}, fmt.Errorf("give the reference once, either as the argument or as --passage")
	}
	if opt.passage == "" && opt.search == "" && fs.NArg() > 0 {
		opt.passage = strings.Join(fs.Args(), " ")
	}
	if !opt.help && opt.passage == "" && opt.search == "" {
		return options{}, fmt.Errorf("give --passage or --search")
	}
	if opt.passage != "" && opt.search != "" {
		return options{}, fmt.Errorf("use either --passage or --search, not both")
	}
	if opt.help {
		return opt, nil
	}
	cfg, err := settings.Load()
	if err != nil && !os.IsNotExist(err) {
		return options{}, err
	}
	opt.keyCmd = cfg.ESVAPIKeyCmd
	opt.translation = strings.ToUpper(strings.TrimSpace(opt.translation))
	// Only an ESV request needs the key. Looking it up for BSB would touch
	// the keyring, or run esv_api_key_cmd, for nothing.
	if opt.translation == esv.ID || (!opt.explicit && cfg.SelectedTranslation == esv.ID) {
		opt.key = esv.Resolve(cfg.ESVAPIKeyCmd)
	}
	if !opt.explicit {
		opt.translation = resolveDefault(cfg.SelectedTranslation, opt.key)
	}
	return opt, nil
}

// reorderArgs puts flags and their values before positional arguments
// so a reference can come before or after the flags.
func reorderArgs(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !isFlag(a) {
			pos = append(pos, a)
			continue
		}
		name, hasValue := splitFlag(a)
		flags = append(flags, a)
		if hasValue || !flagTakesValue(name) {
			continue
		}
		if i+1 < len(args) && !isFlag(args[i+1]) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, pos...)
}

func isFlag(a string) bool {
	return len(a) > 1 && a[0] == '-'
}

func splitFlag(a string) (name string, hasValue bool) {
	a = strings.TrimLeft(a, "-")
	if i := strings.IndexByte(a, '='); i >= 0 {
		return a[:i], true
	}
	return a, false
}

func flagTakesValue(name string) bool {
	switch name {
	case "passage", "search", "translation":
		return true
	default:
		return false
	}
}

func resolveDefault(saved, key string) string {
	if translations.IsBuiltin(saved) {
		return saved
	}
	if saved == esv.ID && key != "" {
		return esv.ID
	}
	return translations.Default
}

type bollsAPI interface {
	GetChapter(translation string, book, chapter int) ([]api.Verse, error)
	SearchVerses(translation, query string) (*api.SearchResponse, error)
}

type esvAPI interface {
	HasKey() bool
	Chapter(book, chapter, startVerse int) (esv.Passage, error)
	Search(query string) (*api.SearchResponse, error)
	Notice() string
}

func execute(stdout, stderr io.Writer, opt options, bolls bollsAPI, esvClient esvAPI) int {
	name := opt.translation
	if err := checkTranslation(name, esvClient); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitNotAllowed
	}
	if opt.search != "" {
		return runSearch(stdout, stderr, opt, bolls, esvClient)
	}
	return runPassage(stdout, stderr, opt, bolls, esvClient)
}

func checkTranslation(name string, esvClient esvAPI) error {
	if translations.IsBuiltin(name) {
		return nil
	}
	if name == esv.ID {
		if esvClient == nil || !esvClient.HasKey() {
			return fmt.Errorf("ESV needs your API key from https://api.esv.org/. Store it in the system keyring (t, then a in the reader), set esv_api_key_cmd, or set OMASWORD_ESV_API_KEY for this command only")
		}
		return nil
	}
	return fmt.Errorf("%s is not available; built-in translations are public domain (ESV needs your own Crossway key)", name)
}

type ref struct {
	book, chapter int
	start, end    int // 0,0 means the whole chapter
	whole         bool
	label         string
}

func parsePassage(q string) (ref, error) {
	// The reader's parser: "John 3:16", "rom 8:28", "1jn 1:9",
	// "ps 23 1-3" and "Song of Solomon 2" all work.
	books := esv.Books()
	id, chapter, start, end, err := reference.Parse(q, books)
	if err != nil {
		return ref{}, fmt.Errorf("unknown reference %q (try \"John 3:16\", \"rom 8:28\" or \"1 John 1:9\")", q)
	}
	book, chapter, start, end, err := reference.Validate(books, id, chapter, start, end)
	if err != nil {
		return ref{}, err
	}
	out := ref{book: book.BookID, chapter: chapter, label: fmt.Sprintf("%s %d", book.Name, chapter), whole: true}
	if start == 0 {
		return out, nil
	}
	out.start, out.end, out.whole = start, end, false
	if start == end {
		out.label = fmt.Sprintf("%s %d:%d", book.Name, chapter, start)
	} else {
		out.label = fmt.Sprintf("%s %d:%d-%d", book.Name, chapter, start, end)
	}
	return out, nil
}

func runPassage(stdout, stderr io.Writer, opt options, bolls bollsAPI, esvClient esvAPI) int {
	wanted, err := parsePassage(opt.passage)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}
	var verses []api.Verse
	var notice string
	clipped := false
	if opt.translation == esv.ID {
		// Start the page at the first verse asked for, so a short range
		// such as Jude 1:20-25 isn't cut at a fixed page boundary. The
		// page is still capped at half the book or 500 verses.
		start := 1
		if !wanted.whole {
			start = wanted.start
		}
		page, err := esvClient.Chapter(wanted.book, wanted.chapter, start)
		if err != nil {
			fmt.Fprintln(stderr, redactErr(err, opt.key))
			return ExitFetch
		}
		notice = page.Notice
		verses = filterVerses(page.Verses, wanted)
		if wanted.whole && page.MoreAfter {
			clipped = true
		}
		if !wanted.whole && wanted.end > page.Last && page.MoreAfter {
			clipped = true
		}
	} else {
		got, err := bolls.GetChapter(opt.translation, wanted.book, wanted.chapter)
		if err != nil {
			fmt.Fprintln(stderr, redactErr(err, opt.key))
			return ExitFetch
		}
		verses = filterVerses(got, wanted)
	}
	if !hasVerseText(verses) {
		fmt.Fprintf(stderr, "no text for %s\n", wanted.label)
		return ExitFetch
	}
	// A second clip, in case a caller handed back more ESV text than
	// Crossway allows on one page.
	if opt.translation == esv.ID && len(verses) > esv.MaxVerses(wanted.book) {
		verses = verses[:esv.MaxVerses(wanted.book)]
		clipped = true
	}
	for i := range verses {
		if verses[i].Book == 0 {
			verses[i].Book = wanted.book
		}
		if verses[i].Chapter == 0 {
			verses[i].Chapter = wanted.chapter
		}
	}
	if opt.json {
		return writeJSON(stdout, stderr, passageDoc{
			Translation: opt.translation,
			Reference:   wanted.label,
			Book:        wanted.book,
			Chapter:     wanted.chapter,
			Verses:      verseDocs(verses),
			Clipped:     clipped,
			Notice:      esvNotice(opt.translation, notice),
			Link:        esvLink(opt.translation),
		})
	}
	fmt.Fprintf(stdout, "%s %s\n\n", opt.translation, wanted.label)
	for _, v := range verses {
		fmt.Fprintf(stdout, "%d %s\n", v.Verse, plainText(v.Text))
	}
	if clipped {
		fmt.Fprintln(stdout, "\nShowing part of this passage. Crossway allows at most 500 verses or half a book on one page. …")
	}
	if opt.translation == esv.ID {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, esv.Attribution(esvNotice(opt.translation, notice)))
	}
	return ExitOK
}

func runSearch(stdout, stderr io.Writer, opt options, bolls bollsAPI, esvClient esvAPI) int {
	var resp *api.SearchResponse
	var err error
	if opt.translation == esv.ID {
		resp, err = esvClient.Search(opt.search)
	} else {
		resp, err = bolls.SearchVerses(opt.translation, opt.search)
	}
	if err != nil {
		fmt.Fprintln(stderr, redactErr(err, opt.key))
		return ExitFetch
	}
	if resp == nil {
		resp = &api.SearchResponse{}
	}
	if opt.translation == esv.ID {
		resp.Results = clipSearch(resp.Results)
	}
	if opt.json {
		return writeJSON(stdout, stderr, searchDoc{
			Translation: opt.translation,
			Query:       opt.search,
			Total:       resp.Total,
			Results:     verseDocs(resp.Results),
			Notice:      esvNotice(opt.translation, ""),
			Link:        esvLink(opt.translation),
		})
	}
	fmt.Fprintf(stdout, "%s search: %s (%d shown", opt.translation, opt.search, len(resp.Results))
	if resp.Total > 0 {
		fmt.Fprintf(stdout, ", %d matches", resp.Total)
	}
	fmt.Fprintln(stdout, ")")
	for _, v := range resp.Results {
		name, _ := esv.BookName(v.Book)
		if name == "" {
			name = fmt.Sprintf("Book %d", v.Book)
		}
		fmt.Fprintf(stdout, "%s %d:%d %s\n", name, v.Chapter, v.Verse, plainText(v.Text))
	}
	if opt.translation == esv.ID {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, esv.Attribution(""))
	}
	return ExitOK
}

func hasVerseText(verses []api.Verse) bool {
	for _, v := range verses {
		if strings.TrimSpace(v.Text) != "" {
			return true
		}
	}
	return false
}

func filterVerses(verses []api.Verse, wanted ref) []api.Verse {
	if wanted.whole {
		return verses
	}
	out := make([]api.Verse, 0, wanted.end-wanted.start+1)
	for _, v := range verses {
		if v.Verse >= wanted.start && v.Verse <= wanted.end {
			out = append(out, v)
		}
	}
	return out
}

// clipSearch enforces the per-book and 500-verse display caps on ESV hits.
func clipSearch(in []api.Verse) []api.Verse {
	per := map[int]int{}
	out := make([]api.Verse, 0, len(in))
	for _, v := range in {
		if len(out) >= esv.MaxStoredVerses {
			break
		}
		if per[v.Book] >= esv.MaxVerses(v.Book) {
			continue
		}
		out = append(out, v)
		per[v.Book]++
	}
	return out
}

func esvNotice(translation, notice string) string {
	if translation != esv.ID {
		return ""
	}
	if notice == "" {
		return esv.CopyrightNotice
	}
	return notice
}

func esvLink(translation string) string {
	if translation != esv.ID {
		return ""
	}
	return esv.Link
}

type verseDoc struct {
	Book    int    `json:"book"`
	Chapter int    `json:"chapter"`
	Verse   int    `json:"verse"`
	Text    string `json:"text"`
}

type passageDoc struct {
	Translation string     `json:"translation"`
	Reference   string     `json:"reference"`
	Book        int        `json:"book"`
	Chapter     int        `json:"chapter"`
	Verses      []verseDoc `json:"verses"`
	Clipped     bool       `json:"clipped,omitempty"`
	Notice      string     `json:"notice,omitempty"`
	Link        string     `json:"link,omitempty"`
}

type searchDoc struct {
	Translation string     `json:"translation"`
	Query       string     `json:"query"`
	Total       int        `json:"total"`
	Results     []verseDoc `json:"results"`
	Notice      string     `json:"notice,omitempty"`
	Link        string     `json:"link,omitempty"`
}

func verseDocs(verses []api.Verse) []verseDoc {
	if verses == nil {
		return []verseDoc{}
	}
	out := make([]verseDoc, len(verses))
	for i, v := range verses {
		out[i] = verseDoc{Book: v.Book, Chapter: v.Chapter, Verse: v.Verse, Text: plainText(v.Text)}
	}
	return out
}

// plainText is the reader's cleanup (Strong's numbers, <mark> around
// search hits, entities, control characters) on one line.
func plainText(s string) string {
	return strings.Join(strings.Fields(api.PlainText(s)), " ")
}

func redactErr(err error, key string) error {
	if err == nil || key == "" || !strings.Contains(err.Error(), key) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), key, "[redacted]"))
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitFetch
	}
	return ExitOK
}

const helpText = `omasword — terminal Bible reader

With no arguments, omasword opens the reader.

Non-interactive mode prints a passage or a search and exits. Public-domain
translations (BSB by default) come from bolls.life. The ESV is online-only
and needs your own key from https://api.esv.org/. The key is stored in the
system keyring, not in a file. esv_api_key_cmd in config.json may name a
command that prints it (for example "pass show esv"); that output is not
saved. OMASWORD_ESV_API_KEY is for one command only and is never written.
ESV output stays within 500 verses or half a book, whichever is less, and
includes Crossway's copyright notice and https://www.esv.org/.

Usage:
  omasword --passage "John 3:16" [--translation BSB] [--json]
  omasword "1 John 1:9"
  omasword --search love [--translation WEB] [--json]

Book names are the English names (Genesis, Psalms, Song of Solomon,
1 John). A chapter alone prints that chapter. "John 3:16" and
"John 3:16-18" print those verses. "Psalm 23" is accepted for Psalms.

Flags:
  --passage TEXT     passage to print
  --search TEXT      word or phrase to search
  --translation ID   BSB (default), KJV, WEB, ASV, YLT, GNV, DRB, or ESV
  --json             print JSON instead of plain text
  --help, -h         show this help

Exit codes:
  0  printed the passage or search result
  1  usage error (bad reference, missing flags)
  2  translation is not allowed, or ESV has no API key
  3  the text could not be fetched (network, rate limit, rejected key)
`
