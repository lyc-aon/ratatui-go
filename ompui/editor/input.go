package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// HandleInput implements component.InputHandler.
//
// Key releases are ignored (editor does not implement KeyReleaseInterest).
// Escape/Ctrl+C delegate to OnInterrupt when set; otherwise they are ignored.
// Ctrl+D on empty buffer invokes OnEOF when set. Input callbacks run after the
// editor mutex is released, so callbacks may safely call exported Editor methods.
func (e *Editor) HandleInput(ev event.Event) {
	e.mu.Lock()
	e.collectingInputCallbacks = true
	e.inputCallbacks = e.inputCallbacks[:0]
	defer func() {
		callbacks := e.inputCallbacks
		e.inputCallbacks = nil
		e.collectingInputCallbacks = false
		e.mu.Unlock()
		for _, callback := range callbacks {
			callback()
		}
	}()
	e.ensureLines()

	switch ev.Kind {
	case event.KindPaste:
		e.handlePaste(ev.Text)
		e.markChanged()
		return
	case event.KindText:
		if ev.Text != "" {
			e.insertCharacter(ev.Text)
			e.markChanged()
		}
		return
	case event.KindKey:
		if ev.Key.Action == event.ActionRelease {
			return
		}
		e.handleKey(ev.Key)
		e.markChanged()
		return
	default:
		// resize/mouse/focus/raw/error: ignore at editor leaf
		return
	}
}

// handleKey dispatches one key press.
//
// Every action declared by ompui/keymap resolves through e.matches, so an
// injected KeyMatcher replaces the defaults instead of layering on top of them.
// The literals that remain are keys the editor owns outright — see keys.go for
// the list and why each one is not a configurable action.
func (e *Editor) handleKey(k event.Key) {
	// Character jump mode consumes the next key as the jump target.
	if e.jumpMode != "" {
		if event.MatchesKey(k, keyInterruptEscape) {
			e.jumpMode = ""
			return
		}
		if k.Text != "" {
			dir := e.jumpMode
			e.jumpMode = ""
			e.jumpToChar(k.Text, dir)
			return
		}
		// Control key cancels and falls through.
		e.jumpMode = ""
	}

	// Autocomplete navigation takes priority for a subset of keys. This is what
	// keeps the dropdown ahead of the host's global model cycling.
	if e.acMode != acOff && e.handleAutocompleteKey(k) {
		return
	}

	// Interrupt / EOF — host-owned when callbacks set.
	if event.MatchesAnyKey(k, keyInterruptCtrlC, keyInterruptEscape) {
		if fn := e.OnInterrupt; fn != nil {
			e.deferInputCallback(fn)
		}
		return
	}
	// EOF fires only on an empty buffer; otherwise ctrl+d falls through to the
	// resolved deleteCharForward action below.
	if event.MatchesKey(k, keyEOF) && e.isEmpty() {
		if fn := e.OnEOF; fn != nil {
			e.deferInputCallback(fn)
			return
		}
	}
	if e.submitMode == SubmitOnCtrlEnter && event.MatchesKey(k, keyCtrlQSubmit) {
		if !e.DisableSubmit {
			e.submitValue()
		}
		return
	}

	// Undo / redo
	if e.matches(k, actUndo) {
		e.applyUndo()
		return
	}
	if event.MatchesKey(k, keyRedo) {
		e.applyRedo()
		return
	}

	// Tab / Shift+Tab
	if e.matches(k, actTab) {
		if e.acMode == acOff {
			e.handleTabCompletion()
		}
		return
	}
	if event.MatchesKey(k, keyShiftTab) {
		// OMP: no indent outdent — ignore unless autocomplete wants it
		return
	}

	// Kill / yank
	if e.matches(k, actDeleteToLineEnd) {
		e.deleteToEndOfLine()
		return
	}
	if e.matches(k, actDeleteToLineStart) {
		e.deleteToStartOfLine()
		return
	}
	if e.matches(k, actDeleteWordBackward) {
		e.deleteWordBackwards()
		return
	}
	if e.matches(k, actDeleteWordForward) {
		e.deleteWordForwards()
		return
	}
	if e.matches(k, actYank) {
		e.yankFromKillRing()
		return
	}
	if e.matches(k, actYankPop) {
		e.yankPop()
		return
	}

	// Line nav
	if e.matches(k, actCursorLineStart) {
		e.moveToLineStart()
		return
	}
	if e.matches(k, actCursorLineEnd) {
		e.moveToLineEnd()
		return
	}
	if event.MatchesAnyKey(k, keysMessageStart...) {
		e.moveToMessageStart()
		return
	}
	if event.MatchesAnyKey(k, keysMessageEnd...) {
		e.moveToMessageEnd()
		return
	}

	// Character jump — arms jump mode; the next printable key is the target.
	if e.matches(k, actJumpForward) {
		e.jumpMode = "forward"
		return
	}
	if e.matches(k, actJumpBackward) {
		e.jumpMode = "backward"
		return
	}

	// Alt+Enter
	if event.MatchesKey(k, keyAltEnter) {
		if fn := e.OnAltEnter; fn != nil {
			text := strings.Join(e.lines, "\n")
			e.deferInputCallback(func() { fn(text) })
		} else {
			e.addNewLine()
		}
		return
	}

	// Newline / submit
	if e.submitMode == SubmitOnCtrlEnter && event.MatchesKey(k, keyCtrlEnterSubmit) {
		if !e.DisableSubmit {
			e.submitValue()
		}
		return
	}
	if e.matches(k, actNewLine) {
		if e.shouldSubmitOnBackslashEnter(k) {
			e.handleBackspace()
			e.submitValue()
			return
		}
		e.addNewLine()
		return
	}
	if e.matches(k, actSubmit) {
		if e.submitMode == SubmitOnCtrlEnter {
			e.addNewLine()
			return
		}
		if e.DisableSubmit {
			return
		}
		// Sync slash completion race
		if e.acMode == acOff {
			e.trySyncSlashOnSubmit()
		}
		e.submitValue()
		return
	}

	// Backspace / delete
	if e.matches(k, actDeleteCharBackward) || event.MatchesKey(k, keyShiftBackspace) {
		e.handleBackspace()
		return
	}
	if e.matches(k, actDeleteCharForward) || event.MatchesKey(k, keyShiftDelete) {
		e.handleForwardDelete()
		return
	}

	// Page up/down
	if e.matches(k, actPageUp) {
		if e.isEmpty() {
			e.navigateHistory(-1)
		} else if e.historyIndex > -1 && e.isOnFirstVisualLine() {
			e.navigateHistory(-1)
		} else {
			e.pageScroll(-1)
		}
		return
	}
	if e.matches(k, actPageDown) {
		if e.historyIndex > -1 && e.isOnLastVisualLine() {
			e.navigateHistory(1)
		} else {
			e.pageScroll(1)
		}
		return
	}

	// Word nav — checked before the plain arrows so modified arrows win.
	if e.matches(k, actCursorWordLeft) {
		e.resetKillSequence()
		e.moveWordBackwards()
		return
	}
	if e.matches(k, actCursorWordRight) {
		e.resetKillSequence()
		e.moveWordForwards()
		return
	}

	// Arrows
	if e.matches(k, actCursorUp) {
		if e.isEmpty() {
			e.navigateHistory(-1)
		} else if e.historyIndex > -1 && e.isOnFirstVisualLine() {
			e.navigateHistory(-1)
		} else if e.isOnFirstVisualLine() {
			e.moveToLineStart()
		} else {
			e.moveCursor(-1, 0)
		}
		return
	}
	if e.matches(k, actCursorDown) {
		if e.historyIndex > -1 && e.isOnLastVisualLine() {
			e.navigateHistory(1)
		} else if e.isOnLastVisualLine() {
			e.moveToLineEnd()
		} else {
			e.moveCursor(1, 0)
		}
		return
	}
	if e.matches(k, actCursorRight) {
		e.moveCursor(0, 1)
		return
	}
	if e.matches(k, actCursorLeft) {
		e.moveCursor(0, -1)
		return
	}

	// Shift+Space inserts a literal space.
	if event.MatchesKey(k, keyShiftSpace) {
		e.insertCharacter(" ")
		return
	}

	// Printable text from key
	if k.Text != "" {
		// Filter pure control
		if isPrintableInsert(k.Text) {
			e.insertCharacter(k.Text)
		}
		return
	}
}

func isPrintableInsert(s string) bool {
	for _, r := range s {
		if r == '\t' || r == '\n' {
			return true
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return s != ""
}

func (e *Editor) shouldSubmitOnBackslashEnter(k event.Key) bool {
	if e.DisableSubmit {
		return false
	}
	// Only when the physical key is plain enter (some terminals send enter for shift+enter maps)
	if !event.MatchesKey(k, keyPhysicalEnter) {
		return false
	}
	cur := e.currentLine()
	return e.cursorCol > 0 && cur[e.cursorCol-1] == '\\'
}

func (e *Editor) trySyncSlashOnSubmit() {
	if e.acProvider == nil {
		return
	}
	p, ok := e.acProvider.(SyncSlashProvider)
	if !ok {
		return
	}
	cur := e.currentLine()
	before := cur[:e.cursorCol]
	if findLeadingSlashCommandStart(before) < 0 || !e.isInSubmittedSlashCommandContext() {
		return
	}
	sync := p.TrySyncSlashCompletion(before)
	if sync == nil || len(sync.Items) == 0 {
		return
	}
	e.acRequestID++
	item := sync.Items[0]
	result := e.acProvider.ApplyCompletion(cloneLines(e.lines), e.cursorLine, e.cursorCol, item, sync.Prefix)
	e.applyCompletionResult(result)
}

func cloneLines(lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

func (e *Editor) applyCompletionResult(result CompletionResult) {
	if len(result.Lines) == 0 {
		result.Lines = []string{""}
	}
	e.lines = result.Lines
	e.cursorLine = result.CursorLine
	if e.cursorLine < 0 {
		e.cursorLine = 0
	}
	if e.cursorLine >= len(e.lines) {
		e.cursorLine = len(e.lines) - 1
	}
	e.setCursorCol(result.CursorCol)
	if result.OnApplied != nil {
		result.OnApplied()
	}
}

// findLeadingSlashCommandStart returns byte index of '/' starting a slash command, or -1.
func findLeadingSlashCommandStart(text string) int {
	i := 0
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == ' ' || r == '\t' {
			i += size
			continue
		}
		if r == '/' {
			return i
		}
		return -1
	}
	return -1
}
