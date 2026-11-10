package ui

import (
	"omasword/internal/api"
	"omasword/internal/settings"
	"omasword/internal/theme"
	"regexp"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type viewMode int

const (
	modeReader viewMode = iota
	modeSearch
	modeComparison
	modeTranslationSelect
	modeCacheManager
	modeAbout
	modeWordSearch
)

type focusPane int

const (
	paneBooks focusPane = iota
	paneContent
)

type Model struct {
	client    *api.Client
	viewport  viewport.Model
	textInput textinput.Model
	// searchErr explains why the last verse lookup was rejected; shown
	// in the lookup panel until the next attempt.
	searchErr              error
	translations           []api.Translation
	selectedTranslation    string
	currentBook            int
	currentChapter         int
	currentBookName        string
	books                  []api.Book
	content                string
	mode                   viewMode
	width                  int
	height                 int
	ready                  bool
	err                    error
	loading                bool
	comparisonTranslations []string
	sidebarSelected        int
	currentVerses          []api.Verse
	currentParallelVerses  map[string][]api.Verse
	highlightedVerseStart  int // Start of highlighted verse range
	highlightedVerseEnd    int // End of highlighted verse range
	// Miller columns state
	millerColumn         int // 0=books, 1=chapters, 2=verses
	millerBookIdx        int
	millerChapterIdx     int
	millerVerseIdx       int
	showMillerColumns    bool
	millerFilterInput    textinput.Model
	millerFilter         string
	millerFilteredBooks  []api.Book
	millerFilteredVerses []api.Verse
	millerFilterMode     bool // When true, all keys go to filter input
	// Cache management state
	cache                  CacheInterface
	cachedTranslations     []string
	cacheSelected          int
	downloadingTranslation string
	// Translation selection state
	translationSelected int
	// Theme state
	currentTheme theme.Theme
	// Word search state
	wordSearchInput    textinput.Model
	wordSearchQuery    string
	wordSearchResults  []api.Verse
	wordSearchTotal    int
	wordSearchSelected int
	wordSearchLoading  bool
	// Pane focus (book list vs content)
	focus focusPane
	// topVisibleVerse mirrors the verse number currently at the top of
	// the viewport. The right pane title surfaces it as a sticky scroll
	// indicator so the reader always knows where they are.
	topVisibleVerse int
	// Last known mouse position. Updated on every MouseClickMsg /
	// MouseMotionMsg / MouseWheelMsg. The render functions read these
	// to surface hover state (book row hover in the left pane, verse
	// number hover in the right pane title).
	mouseX, mouseY int
	// dragAnchorVerse holds the verse number where the user pressed
	// the left mouse button in the reader. While the button is held
	// and the mouse moves, the highlighted range is extended from this
	// anchor to whichever verse the cursor is now over. 0 means no
	// drag in progress.
	dragAnchorVerse int
	// comparisonPickerColumn is the column index in comparisonTranslations
	// whose translation is currently being swapped via the translation
	// picker. -1 means the picker is not scoped to a column (i.e. the
	// normal "set the active translation" flow).
	comparisonPickerColumn int
	// Translation download progress in [0, 1]. Polled from the cache
	// every ~120ms while a download is running.
	downloadProgress float64
	progressBar      progress.Model
}

type CacheInterface interface {
	IsCached(translation string) bool
	GetChapter(translation string, book, chapter int) ([]api.Verse, error)
	DownloadTranslation(translation string) error
	// DownloadProgress reports the byte-level progress of the currently
	// running download as a value in [0, 1] and the translation
	// short-name being downloaded ("" if idle). Safe to call from any
	// goroutine.
	DownloadProgress() (float64, string)
	ListCached() ([]string, error)
	GetCacheSize() (int64, error)
	RemoveTranslation(translation string) error
}

type (
	errMsg                  struct{ err error }
	translationsLoadedMsg   struct{ translations []api.Translation }
	booksLoadedMsg          struct{ books []api.Book }
	chapterLoadedMsg        struct{ verses []api.Verse }
	parallelVersesLoadedMsg struct{ verses map[string][]api.Verse }
	cacheListLoadedMsg      struct{ translations []string }
	downloadCompleteMsg     struct{ translation string }
	downloadErrorMsg        struct {
		translation string
		err         error
	}
)

type searchResultsLoadedMsg struct {
	results []api.Verse
	total   int
	query   string
}

// downloadTickMsg fires roughly every 120ms while a translation download
// is running so the UI can poll the cache for byte-level progress.
type downloadTickMsg struct{}

func (e errMsg) Error() string { return e.err.Error() }

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter verse reference (e.g., 1 1:1 or Gen 1:1)"
	ti.Focus()
	ti.CharLimit = 50
	ti.SetWidth(50)

	millerFilter := textinput.New()
	millerFilter.Placeholder = "Type to filter..."
	millerFilter.CharLimit = 50
	millerFilter.SetWidth(25)

	wordSearch := textinput.New()
	wordSearch.Placeholder = "Search the Bible..."
	wordSearch.CharLimit = 100
	wordSearch.SetWidth(50)

	// --- Load persisted settings (if any) ---
	cfg, err := settings.Load()

	selectedTranslation := "ESV"
	currentBook := 1
	currentChapter := 1

	// Colors always follow the active Omarchy desktop theme.
	theme.RefreshOmarchy()
	currentTheme := theme.Current()

	if err == nil {
		if api.ValidTranslationName(cfg.SelectedTranslation) {
			selectedTranslation = cfg.SelectedTranslation
		}
		if cfg.CurrentBook > 0 {
			currentBook = cfg.CurrentBook
		}
		if cfg.CurrentChapter > 0 {
			currentChapter = cfg.CurrentChapter
		}
	}

	return Model{
		client:                 api.NewClient(),
		textInput:              ti,
		millerFilterInput:      millerFilter,
		wordSearchInput:        wordSearch,
		selectedTranslation:    selectedTranslation,
		currentBook:            currentBook,
		currentChapter:         currentChapter,
		currentBookName:        "Genesis", // corrected after books load
		mode:                   modeReader,
		comparisonTranslations: []string{"ESV", "KJV", "NLT"},
		currentTheme:           currentTheme,
		focus:                  paneContent,
		progressBar:            progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage()),
		comparisonPickerColumn: -1,
	}
}

func (m *Model) SetCache(cache CacheInterface) {
	m.cache = cache
	if cache != nil {
		// Set cache on API client too
		m.client.SetCache(cache)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadTranslations(m.client),
		loadBooks(m.client, m.selectedTranslation),
		loadChapter(m.client, m.selectedTranslation, m.currentBook, m.currentChapter),
		omarchyTick(),
	)
}

// Layout constants for the two-pane shell.
const (
	leftPaneOuterWidth = 30 // books pane outer width incl. rounded border
	headerOuterHeight  = 3  // header rounded box: 1 content + 2 border lines
	statusOuterHeight  = 3  // status bar rounded box: same
)

// Compiled once: stripHTMLTags runs for every verse on every redraw.
var (
	htmlTagRe       = regexp.MustCompile(`<[^>]*>`)
	numericEntityRe = regexp.MustCompile(`&#(\d+);`)
	hexEntityRe     = regexp.MustCompile(`&#[xX]([0-9a-fA-F]+);`)
	spaceRunRe      = regexp.MustCompile(`\s+`)

	// Book identifier alternatives, in order of specificity:
	// digit + letters ("1 John"), letters ("Song of Solomon"), digits ("1").
	referenceRe = regexp.MustCompile(`(?i)^(\d+\s*[a-z]+(?:\s+[a-z]+)*|[a-z]+(?:\s+[a-z]+)*|\d+)\s*(.*)$`)
	digitsRe    = regexp.MustCompile(`\d+`)
)
