package editor

// CursorPos is a logical cursor position.
//
// Line is a 0-based logical line index.
// Col is a UTF-8 byte offset into that line (0..len(line)), always snapped to
// a code-point boundary by editor setters. It is NOT a grapheme index and NOT
// a terminal cell column.
type CursorPos struct {
	Line int
	Col  int
}

// Selection is a half-open range in logical positions.
// When Active is false the range is ignored. Anchor is where the selection
// began; Cursor tracks the moving end (same as the caret).
type Selection struct {
	Active bool
	Anchor CursorPos
	Cursor CursorPos
}

// HistoryCursorAnchor places the caret after loading a history entry.
type HistoryCursorAnchor string

const (
	// HistoryAnchorStart puts the caret at the start of the loaded entry.
	HistoryAnchorStart HistoryCursorAnchor = "start"
	// HistoryAnchorEnd puts the caret at the end of the loaded entry.
	HistoryAnchorEnd HistoryCursorAnchor = "end"
)

// AutocompleteItem is one dropdown candidate.
type AutocompleteItem struct {
	Value       string
	Label       string
	Description string
	// Hint is dim ghost text shown after the caret when this item is selected.
	Hint string
}

// Suggestions is a provider response.
type Suggestions struct {
	Items  []AutocompleteItem
	Prefix string
}

// CompletionResult is the buffer state after applying a completion.
type CompletionResult struct {
	Lines      []string
	CursorLine int
	// CursorCol is a UTF-8 byte offset into Lines[CursorLine].
	CursorCol int
	OnApplied func()
}

// InlineReplace is a synchronous shortcode-style rewrite.
type InlineReplace struct {
	// ReplaceLen is UTF-8 bytes immediately before the cursor to delete.
	ReplaceLen int
	Insert     string
}

// AutocompleteProvider supplies candidates and applies selections.
//
// GetSuggestions may be called from a background goroutine; the editor stamps
// a generation so stale results are dropped. ApplyCompletion must be pure and
// synchronous. Render never blocks on provider I/O.
type AutocompleteProvider interface {
	GetSuggestions(lines []string, cursorLine, cursorCol int) *Suggestions
	ApplyCompletion(lines []string, cursorLine, cursorCol int, item AutocompleteItem, prefix string) CompletionResult
}

// AutocompleteProviderExt is optional provider hooks.
// Implementers may embed a base provider and override only the methods they need;
// methods returning zero values are treated as "not applicable".
type AutocompleteProviderExt interface {
	AutocompleteProvider
	GetInlineHint(lines []string, cursorLine, cursorCol int) string
	TrySyncSlashCompletion(textBeforeCursor string) *Suggestions
	TrySyncInlineReplace(textBeforeCursor string) *InlineReplace
	GetForceFileSuggestions(lines []string, cursorLine, cursorCol int) *Suggestions
	ShouldTriggerFileCompletion(lines []string, cursorLine, cursorCol int) bool
}

// ForceFileProvider is the narrow optional force-file hook.
type ForceFileProvider interface {
	GetForceFileSuggestions(lines []string, cursorLine, cursorCol int) *Suggestions
	ShouldTriggerFileCompletion(lines []string, cursorLine, cursorCol int) bool
}

// InlineHintProvider is the optional ghost-text hook.
type InlineHintProvider interface {
	GetInlineHint(lines []string, cursorLine, cursorCol int) string
}

// SyncSlashProvider is the optional sync slash-completion hook.
type SyncSlashProvider interface {
	TrySyncSlashCompletion(textBeforeCursor string) *Suggestions
}

// SyncInlineReplaceProvider is the optional shortcode rewrite hook.
type SyncInlineReplaceProvider interface {
	TrySyncInlineReplace(textBeforeCursor string) *InlineReplace
}

// SubmitMode selects which key submits the editor.
type SubmitMode uint8

const (
	// SubmitOnEnter is the normal prompt behavior: Enter submits and modified
	// Enter inserts a newline.
	SubmitOnEnter SubmitMode = iota
	// SubmitOnCtrlEnter is hook-editor behavior: Enter inserts a newline while
	// Ctrl+Enter or Ctrl+Q submits.
	SubmitOnCtrlEnter
)

// Option configures a new Editor.
type Option func(*Editor)

// WithPlaceholder sets empty-buffer ghost text.
func WithPlaceholder(s string) Option {
	return func(e *Editor) { e.placeholder = s }
}

// WithPromptPrefix sets the borderless first-line gutter (e.g. "> ").
func WithPromptPrefix(s string) Option {
	return func(e *Editor) { e.promptGutter = s }
}

// WithBorder enables or disables box chrome (default true).
func WithBorder(visible bool) Option {
	return func(e *Editor) { e.borderVisible = visible }
}

// WithSubmitMode selects plain-Enter or Ctrl+Enter/Ctrl+Q submission.
func WithSubmitMode(mode SubmitMode) Option {
	return func(e *Editor) { e.submitMode = mode }
}

// WithPaddingX sets horizontal padding inside the border.
func WithPaddingX(n int) Option {
	return func(e *Editor) {
		if n < 0 {
			n = 0
		}
		e.paddingX = n
	}
}

// WithMaxHeight caps total editor height including border chrome.
// Zero means unlimited.
func WithMaxHeight(n int) Option {
	return func(e *Editor) {
		if n < 0 {
			n = 0
		}
		e.maxHeight = n
	}
}

// WithAutocompleteMaxVisible sets dropdown height (clamped 3..20).
func WithAutocompleteMaxVisible(n int) Option {
	return func(e *Editor) {
		if n < 3 {
			n = 3
		}
		if n > 20 {
			n = 20
		}
		e.acMaxVisible = n
	}
}

// WithOnSubmit registers the Enter-submit callback.
func WithOnSubmit(fn func(text string)) Option {
	return func(e *Editor) { e.OnSubmit = fn }
}

// WithOnChange registers the buffer-change callback.
func WithOnChange(fn func(text string)) Option {
	return func(e *Editor) { e.OnChange = fn }
}

// WithOnInterrupt registers Escape/Ctrl+C handling. When unset, they are ignored
// (left for the host). When set, the key is consumed.
func WithOnInterrupt(fn func()) Option {
	return func(e *Editor) { e.OnInterrupt = fn }
}

// WithOnEOF registers Ctrl+D-on-empty handling.
func WithOnEOF(fn func()) Option {
	return func(e *Editor) { e.OnEOF = fn }
}

// WithOnAltEnter registers Alt+Enter. When unset, Alt+Enter inserts a newline.
func WithOnAltEnter(fn func(text string)) Option {
	return func(e *Editor) { e.OnAltEnter = fn }
}

// WithOnLargePaste intercepts marker-sized pastes. Return true to swallow.
func WithOnLargePaste(fn func(text string, lineCount int) bool) Option {
	return func(e *Editor) { e.OnLargePaste = fn }
}

// WithAutocompleteProvider injects the suggestion source.
func WithAutocompleteProvider(p AutocompleteProvider) Option {
	return func(e *Editor) { e.acProvider = p }
}

// WithKeyMatcher injects the host's resolved keybinding source (typically
// *ompui/keymap.Registry). Without it the editor uses its built-in defaults.
func WithKeyMatcher(m KeyMatcher) Option {
	return func(e *Editor) { e.keys = m }
}

// WithBorderColor sets a styling wrapper for border glyphs.
// Default is identity.
func WithBorderColor(fn func(string) string) Option {
	return func(e *Editor) {
		if fn != nil {
			e.borderColor = fn
		}
	}
}

// WithCursorGlyph overrides the software end-of-text cursor glyph.
func WithCursorGlyph(glyph string) Option {
	return func(e *Editor) { e.cursorGlyph = glyph }
}

// WithDisableSubmit blocks plain Enter submit when true.
func WithDisableSubmit(v bool) Option {
	return func(e *Editor) { e.DisableSubmit = v }
}

// WithAtomicTokenPattern sets a Go regexp used to treat matching spans as
// atomic (image/paste markers). Matched with FindAllStringIndex per line;
// empty matches are ignored.
func WithAtomicTokenPattern(pattern string) Option {
	return func(e *Editor) { e.SetAtomicTokenPattern(pattern) }
}

// WithHistory seeds the in-memory prompt history (newest first).
func WithHistory(entries []string) Option {
	return func(e *Editor) {
		e.history = e.history[:0]
		for _, s := range entries {
			e.AddToHistory(s)
		}
	}
}
