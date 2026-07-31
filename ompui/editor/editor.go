package editor

import (
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/killring"
)

// Compile-time interface checks.
var (
	_ component.Component           = (*Editor)(nil)
	_ component.InputHandler        = (*Editor)(nil)
	_ component.Focusable           = (*Editor)(nil)
	_ component.TerminalCursorAware = (*Editor)(nil)
	_ component.Invalidator         = (*Editor)(nil)
	_ component.Disposable          = (*Editor)(nil)
)

const (
	maxUndoStack        = 100
	maxHistoryEntries   = 100
	defaultPageScroll   = 10
	defaultACMaxVisible = 5
	defaultPaddingX     = 2
	defaultLayoutWidth  = 80
	pasteMarkerLines    = 10
	pasteMarkerChars    = 1000
	defaultCursorGlyph  = "▌"
	wrapCacheLimit      = 256
)

// lastAction tracks kill/yank/type coalescing.
type lastAction uint8

const (
	actionNone lastAction = iota
	actionKill
	actionYank
	actionTypeWord
)

// acMode is the autocomplete popup state.
type acMode uint8

const (
	acOff acMode = iota
	acRegular
	acForce
)

// editorState is one undo snapshot.
type editorState struct {
	lines      []string
	cursorLine int
	cursorCol  int // UTF-8 byte offset
}

// visualLine maps one wrap segment to a logical line span.
type visualLine struct {
	logicalLine int
	startCol    int // UTF-8 byte offset
	length      int // UTF-8 byte length of the segment span in the logical line
}

// layoutLine is one rendered content row before chrome.
type layoutLine struct {
	text        string
	hasCursor   bool
	cursorPos   int // UTF-8 byte offset into text
	selStart    int // inclusive byte offset into text; -1 = none
	selEnd      int // exclusive byte offset into text
	logicalLine int
	startCol    int
}

// Editor is the production OMP multiline editor.
//
// Index unit invariant: CursorPos.Col, Selection ends, and every public
// "col"/"cursor" argument are UTF-8 byte offsets. Grapheme steps and visual
// cell columns are internal only.
type Editor struct {
	component.FocusState

	mu sync.Mutex

	lines      []string
	cursorLine int
	cursorCol  int // UTF-8 byte offset into lines[cursorLine]

	sel Selection

	// preferredVisualCol is sticky column for vertical moves; -1 = unset.
	preferredVisualCol int

	gen component.Gen

	// Layout / chrome
	lastLayoutWidth  int
	wrapCache        map[string][]textChunk
	wrapCacheWidth   int
	paddingX         int
	maxHeight        int
	scrollOffset     int
	borderVisible    bool
	borderColor      func(string) string
	promptGutter     string
	placeholder      string
	cursorGlyph      string
	cursorOverride   string
	cursorOverrideW  int
	decorateText     func(string) string
	topBorderContent string
	topBorderWidth   int

	// Kill ring
	killRing   killring.Ring
	lastAction lastAction

	// Jump mode
	jumpMode string // "", "forward", "backward"

	// History (newest first)
	history        []string
	historyIndex   int    // -1 = not browsing
	historyScratch string // text present when history browse began; restored at index -1

	// Undo / redo
	undoStack   []editorState
	redoStack   []editorState
	suspendUndo bool

	// Paste markers
	pastes       map[int]string
	pasteCounter int

	// Atomic tokens
	atomicPattern string
	atomicRe      *regexp.Regexp

	// Autocomplete
	acProvider       AutocompleteProvider
	acMode           acMode
	acItems          []AutocompleteItem
	acPrefix         string
	acSelected       int
	acRequestID      uint64
	acMaxVisible     int
	acPendingRefresh bool

	// Callbacks
	OnSubmit             func(text string)
	OnChange             func(text string)
	OnAltEnter           func(text string)
	OnInterrupt          func()
	OnEOF                func()
	OnLargePaste         func(text string, lineCount int) bool
	OnAutocompleteUpdate func()
	OnAutocompleteCancel func()
	DisableSubmit        bool
	submitMode           SubmitMode
	// Input callbacks are drained after HandleInput releases mu so callbacks may
	// safely call exported Editor methods.
	inputCallbacks           []func()
	collectingInputCallbacks bool

	// Volatile speech preview length in UTF-8 bytes.
	volatileTextLen int

	// Cached last frame for generation stability.
	lastFrame     component.Frame
	lastFrameOK   bool
	lastRenderKey string
}

// New constructs an Editor with optional configuration.
func New(opts ...Option) *Editor {
	e := &Editor{
		lines:              []string{""},
		preferredVisualCol: -1,
		lastLayoutWidth:    defaultLayoutWidth,
		wrapCache:          make(map[string][]textChunk),
		wrapCacheWidth:     -1,
		paddingX:           defaultPaddingX,
		borderVisible:      true,
		borderColor:        func(s string) string { return s },
		cursorGlyph:        defaultCursorGlyph,
		historyIndex:       -1,
		pastes:             make(map[int]string),
		acMaxVisible:       defaultACMaxVisible,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	e.gen.Next() // start at 1 so empty frame gen is non-zero after first paint
	return e
}

// --- public text / cursor API ---

// Text returns the buffer joined with '\n'.
func (e *Editor) Text() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.lines, "\n")
}

// ExpandedText expands [Paste #N] markers to their stored content.
func (e *Editor) ExpandedText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.expandPasteMarkers(strings.Join(e.lines, "\n"))
}

// Lines returns a copy of the logical lines.
func (e *Editor) Lines() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.lines))
	copy(out, e.lines)
	return out
}

// SetText replaces the buffer and places the caret at the end.
// Clears undo, selection, history browse, and kill sequence.
func (e *Editor) SetText(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.historyIndex = -1
	e.historyScratch = ""
	e.resetKillSequence()
	e.clearSelection()
	e.setTextInternal(text, HistoryAnchorEnd)
	e.markChanged()
}

// SetTextAt replaces the buffer and places the caret per anchor.
func (e *Editor) SetTextAt(text string, anchor HistoryCursorAnchor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.historyIndex = -1
	e.historyScratch = ""
	e.resetKillSequence()
	e.clearSelection()
	e.setTextInternal(text, anchor)
	e.markChanged()
}

// Cursor returns the logical caret (byte-offset col).
func (e *Editor) Cursor() CursorPos {
	e.mu.Lock()
	defer e.mu.Unlock()
	return CursorPos{Line: e.cursorLine, Col: e.cursorCol}
}

// SetCursor moves the caret. Col is a UTF-8 byte offset; it is clamped and
// snapped to a code-point boundary. Clears selection and sticky column.
func (e *Editor) SetCursor(pos CursorPos) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clearSelection()
	e.setCursorRaw(pos.Line, pos.Col)
	e.markChanged()
}

// Selection returns the current selection (Active may be false).
func (e *Editor) Selection() Selection {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sel
}

// SetSelection sets an active selection. Both ends use byte-offset cols.
// The caret becomes the Cursor end. Pass Active=false to clear.
func (e *Editor) SetSelection(sel Selection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !sel.Active {
		e.clearSelection()
		e.markChanged()
		return
	}
	sel.Anchor = e.clampPos(sel.Anchor)
	sel.Cursor = e.clampPos(sel.Cursor)
	e.sel = sel
	e.sel.Active = true
	e.cursorLine = sel.Cursor.Line
	e.cursorCol = sel.Cursor.Col
	e.preferredVisualCol = -1
	e.markChanged()
}

// SelectAll selects the entire buffer.
func (e *Editor) SelectAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	last := len(e.lines) - 1
	if last < 0 {
		last = 0
	}
	endCol := 0
	if last < len(e.lines) {
		endCol = len(e.lines[last])
	}
	e.sel = Selection{
		Active: true,
		Anchor: CursorPos{Line: 0, Col: 0},
		Cursor: CursorPos{Line: last, Col: endCol},
	}
	e.cursorLine = last
	e.cursorCol = endCol
	e.preferredVisualCol = -1
	e.markChanged()
}

// ClearSelection drops any active selection without moving the caret.
func (e *Editor) ClearSelection() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clearSelection()
	e.markChanged()
}

// SelectedText returns the selected substring, or "" if none.
func (e *Editor) SelectedText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.sel.Active {
		return ""
	}
	a, b := orderedSel(e.sel.Anchor, e.sel.Cursor)
	return e.sliceRange(a, b)
}

// Focused reports focus (promoted from FocusState for interface clarity).
// SetFocused / SetUseTerminalCursor come from the embedded FocusState.

// Placeholder returns empty-buffer ghost text.
func (e *Editor) Placeholder() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.placeholder
}

// SetPlaceholder sets empty-buffer ghost text.
func (e *Editor) SetPlaceholder(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.placeholder == s {
		return
	}
	e.placeholder = s
	e.markChanged()
}

// PromptPrefix returns the borderless gutter prefix.
func (e *Editor) PromptPrefix() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.promptGutter
}

// SetPromptPrefix sets the borderless first-line gutter.
func (e *Editor) SetPromptPrefix(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.promptGutter == s {
		return
	}
	e.promptGutter = s
	e.markChanged()
}

// SetBorderVisible shows or hides box chrome.
func (e *Editor) SetBorderVisible(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.borderVisible == v {
		return
	}
	e.borderVisible = v
	e.markChanged()
}

// SetBorderColor replaces the border styling wrapper.
func (e *Editor) SetBorderColor(fn func(string) string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if fn == nil {
		fn = func(s string) string { return s }
	}
	e.borderColor = fn
	e.markChanged()
}

// SetPaddingX sets horizontal padding cells inside the border.
func (e *Editor) SetPaddingX(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n < 0 {
		n = 0
	}
	if e.paddingX == n {
		return
	}
	e.paddingX = n
	e.markChanged()
}

// SetMaxHeight caps total height including border; 0 = unlimited.
func (e *Editor) SetMaxHeight(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n < 0 {
		n = 0
	}
	if e.maxHeight == n {
		return
	}
	e.maxHeight = n
	e.markChanged()
}

// SetTopBorder sets optional status content embedded in the top border row.
// Pass empty content to clear. width is the visible cell width of content.
func (e *Editor) SetTopBorder(content string, width int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.topBorderContent = content
	e.topBorderWidth = width
	e.markChanged()
}

// SetCursorOverride replaces the software end-of-text cursor glyph.
// Pass empty to restore the default. width is the visible cell width.
func (e *Editor) SetCursorOverride(glyph string, width int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cursorOverride = glyph
	e.cursorOverrideW = width
	e.markChanged()
}

// SetDecorateText installs a zero-width SGR decorator for displayed input.
// The function MUST preserve visible width.
func (e *Editor) SetDecorateText(fn func(string) string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.decorateText = fn
	e.markChanged()
}

// SetAtomicTokenPattern compiles pattern as a regexp for atomic spans.
// Empty pattern clears. Invalid patterns are ignored (no panic).
func (e *Editor) SetAtomicTokenPattern(pattern string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if pattern == e.atomicPattern {
		return
	}
	e.atomicPattern = pattern
	e.atomicRe = nil
	if pattern == "" {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.atomicPattern = ""
		return
	}
	e.atomicRe = re
}

// InsertText inserts at the caret (replacing selection if any).
func (e *Editor) InsertText(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exitHistoryForEditing()
	e.insertTextAtCursor(text, true)
	e.markChanged()
	e.fireChange()
}

// PasteText applies paste sanitization and large-paste rules.
func (e *Editor) PasteText(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlePaste(text)
	e.markChanged()
}

// InsertPaste stores content as a collapsed [Paste #N] marker.
func (e *Editor) InsertPaste(content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.historyIndex = -1
	e.resetKillSequence()
	e.recordUndo()
	e.withUndoSuspended(func() {
		e.storePasteMarker(content, strings.Count(content, "\n")+1)
	})
	e.markChanged()
	e.fireChange()
}

// DeleteBeforeCursor removes up to count UTF-8 bytes immediately before the
// caret on the current line (never crosses a line boundary).
func (e *Editor) DeleteBeforeCursor(count int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	removable := count
	if removable > e.cursorCol {
		removable = e.cursorCol
	}
	if removable <= 0 {
		return
	}
	// Snap to code-point boundary.
	line := e.lines[e.cursorLine]
	start := clampByteOffset(line, e.cursorCol-removable)
	removable = e.cursorCol - start
	if removable <= 0 {
		return
	}
	e.exitHistoryForEditing()
	e.recordUndo()
	e.lines[e.cursorLine] = line[:start] + line[e.cursorCol:]
	e.setCursorCol(start)
	e.lastAction = actionNone
	e.markChanged()
	e.fireChange()
}

// --- history ---

// AddToHistory pushes a submitted prompt (newest first). Empty/whitespace and
// consecutive duplicates are skipped. Bounded to 100 entries.
func (e *Editor) AddToHistory(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	if len(e.history) > 0 && e.history[0] == trimmed {
		return
	}
	e.history = append([]string{trimmed}, e.history...)
	if len(e.history) > maxHistoryEntries {
		e.history = e.history[:maxHistoryEntries]
	}
}

// History returns a copy of history entries (newest first).
func (e *Editor) History() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.history))
	copy(out, e.history)
	return out
}

// ClearHistory drops all history entries.
func (e *Editor) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = nil
	e.historyIndex = -1
	e.historyScratch = ""
}

// --- undo / redo ---

// Undo reverts the last undoable edit.
func (e *Editor) Undo() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.applyUndo()
	e.markChanged()
}

// Redo re-applies the last undone edit.
func (e *Editor) Redo() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.applyRedo()
	e.markChanged()
}

// --- kill / yank ---

// Yank inserts the newest kill-ring entry at the caret.
func (e *Editor) Yank() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.yankFromKillRing()
	e.markChanged()
}

// YankPop cycles the kill ring when the last action was yank.
func (e *Editor) YankPop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.yankPop()
	e.markChanged()
}

// KillRingLen returns the number of kill-ring entries.
func (e *Editor) KillRingLen() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.killRing.Len()
}

// --- autocomplete ---

// SetAutocompleteProvider injects or clears the suggestion source.
func (e *Editor) SetAutocompleteProvider(p AutocompleteProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.acProvider = p
}

// SetAutocompleteMaxVisible clamps dropdown height to 3..20.
func (e *Editor) SetAutocompleteMaxVisible(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n < 3 {
		n = 3
	}
	if n > 20 {
		n = 20
	}
	e.acMaxVisible = n
}

// IsShowingAutocomplete reports whether the dropdown is open.
func (e *Editor) IsShowingAutocomplete() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.acMode != acOff
}

// CancelAutocomplete closes the dropdown.
func (e *Editor) CancelAutocomplete() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cancelAutocomplete(false)
	e.markChanged()
}

// ApplyAutocompleteResult feeds a provider response for the given request id.
// Stale ids are ignored. Safe to call from another goroutine.
func (e *Editor) ApplyAutocompleteResult(requestID uint64, suggestions *Suggestions) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if requestID != e.acRequestID {
		return
	}
	if suggestions == nil || len(suggestions.Items) == 0 {
		e.cancelAutocomplete(false)
		e.markChanged()
		if e.OnAutocompleteUpdate != nil {
			go e.OnAutocompleteUpdate()
		}
		return
	}
	e.acPrefix = suggestions.Prefix
	e.acItems = append([]AutocompleteItem(nil), suggestions.Items...)
	e.acSelected = 0
	if e.acMode == acOff {
		e.acMode = acRegular
	}
	e.markChanged()
	if e.OnAutocompleteUpdate != nil {
		go e.OnAutocompleteUpdate()
	}
}

// BeginAutocompleteRequest bumps the generation and returns the new id plus a
// snapshot of lines/cursor for the provider. Host runs GetSuggestions async
// and calls ApplyAutocompleteResult.
func (e *Editor) BeginAutocompleteRequest() (requestID uint64, lines []string, cursorLine, cursorCol int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.acRequestID++
	requestID = e.acRequestID
	lines = make([]string, len(e.lines))
	copy(lines, e.lines)
	return requestID, lines, e.cursorLine, e.cursorCol
}

// Generation returns the current content generation.
func (e *Editor) Generation() uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.gen.Current()
}

// Invalidate drops render caches.
func (e *Editor) Invalidate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.wrapCache = make(map[string][]textChunk)
	e.wrapCacheWidth = -1
	e.lastFrameOK = false
	e.markChanged()
}

// Dispose is a no-op (no timers owned); satisfies Disposable.
func (e *Editor) Dispose() {}

// MoveToLineStart / End / MessageStart / End are public caret helpers.
func (e *Editor) MoveToLineStart() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.moveToLineStart()
	e.markChanged()
}

func (e *Editor) MoveToLineEnd() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.moveToLineEnd()
	e.markChanged()
}

func (e *Editor) MoveToMessageStart() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.moveToMessageStart()
	e.markChanged()
}

func (e *Editor) MoveToMessageEnd() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.moveToMessageEnd()
	e.markChanged()
}

// --- internal helpers (locked) ---

func (e *Editor) markChanged() {
	e.gen.Next()
	e.lastFrameOK = false
}

func (e *Editor) fireChange() {
	if e.OnChange == nil {
		return
	}
	text := strings.Join(e.lines, "\n")
	// Unlock is caller's job; fire while holding lock is OK for sync hosts.
	// Prefer not re-entering; hosts must not call back into editor from OnChange
	// without care. OMP does the same.
	e.OnChange(text)
}

func (e *Editor) deferInputCallback(fn func()) {
	if fn == nil {
		return
	}
	if e.collectingInputCallbacks {
		e.inputCallbacks = append(e.inputCallbacks, fn)
		return
	}
	fn()
}

func (e *Editor) isEmpty() bool {
	return len(e.lines) == 1 && e.lines[0] == ""
}

func (e *Editor) currentLine() string {
	if e.cursorLine < 0 || e.cursorLine >= len(e.lines) {
		return ""
	}
	return e.lines[e.cursorLine]
}

func (e *Editor) setCursorCol(col int) {
	line := e.currentLine()
	e.cursorCol = clampByteOffset(line, col)
	e.preferredVisualCol = -1
}

func (e *Editor) setCursorRaw(line, col int) {
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	if line < 0 {
		line = 0
	}
	if line >= len(e.lines) {
		line = len(e.lines) - 1
	}
	e.cursorLine = line
	e.setCursorCol(col)
}

func (e *Editor) clampPos(p CursorPos) CursorPos {
	if len(e.lines) == 0 {
		return CursorPos{}
	}
	if p.Line < 0 {
		p.Line = 0
	}
	if p.Line >= len(e.lines) {
		p.Line = len(e.lines) - 1
	}
	p.Col = clampByteOffset(e.lines[p.Line], p.Col)
	return p
}

func (e *Editor) clearSelection() {
	e.sel = Selection{}
}

func (e *Editor) resetKillSequence() {
	e.lastAction = actionNone
}

func (e *Editor) setTextInternal(text string, anchor HistoryCursorAnchor) {
	e.undoStack = e.undoStack[:0]
	e.redoStack = e.redoStack[:0]
	clean := sanitizeLoadedText(text)
	parts := strings.Split(clean, "\n")
	if len(parts) == 0 {
		parts = []string{""}
	}
	e.lines = parts
	if anchor == HistoryAnchorStart {
		e.cursorLine = 0
		e.setCursorCol(0)
	} else {
		e.cursorLine = len(e.lines) - 1
		e.setCursorCol(len(e.lines[e.cursorLine]))
	}
	e.scrollOffset = 0
	if e.OnChange != nil {
		e.OnChange(strings.Join(e.lines, "\n"))
	}
}

func sanitizeLoadedText(text string) string {
	// CRLF/CR → LF, tabs → 3 spaces, strip C0 except \n.
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			b.WriteByte('\n')
		case c == '\t':
			b.WriteString("   ")
		case c == '\n':
			b.WriteByte('\n')
		case c < 0x20:
			// strip other C0
		default:
			// copy full rune
			r, size := utf8.DecodeRuneInString(text[i:])
			if r == utf8.RuneError && size == 1 {
				// keep replacement as-is byte to avoid panic/data loss
				b.WriteByte(c)
				continue
			}
			b.WriteString(text[i : i+size])
			i += size - 1
		}
	}
	return b.String()
}

// ensure lines non-nil
func (e *Editor) ensureLines() {
	if len(e.lines) == 0 {
		e.lines = []string{""}
		e.cursorLine = 0
		e.cursorCol = 0
	}
}
