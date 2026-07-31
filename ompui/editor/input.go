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

func (e *Editor) handleKey(k event.Key) {
	// Character jump mode
	if e.jumpMode != "" {
		if event.MatchesAnyKey(k, "ctrl+t", "ctrl+g", "alt+t", "alt+g") {
			// cancel if jump hotkey pressed again — exact bindings below also cancel
		}
		if event.MatchesKey(k, "escape") {
			e.jumpMode = ""
			return
		}
		if k.Text != "" {
			dir := e.jumpMode
			e.jumpMode = ""
			e.jumpToChar(k.Text, dir)
			return
		}
		// Control key cancels and falls through
		e.jumpMode = ""
	}

	// Autocomplete navigation takes priority for a subset of keys.
	if e.acMode != acOff && e.handleAutocompleteKey(k) {
		return
	}

	// Interrupt / EOF — host-owned when callbacks set.
	if event.MatchesAnyKey(k, "ctrl+c", "escape") {
		if fn := e.OnInterrupt; fn != nil {
			e.deferInputCallback(fn)
		}
		return
	}
	if event.MatchesKey(k, "ctrl+d") {
		if e.isEmpty() {
			if fn := e.OnEOF; fn != nil {
				e.deferInputCallback(fn)
				return
			}
		}
		// otherwise forward-delete
		e.handleForwardDelete()
		return
	}
	if e.submitMode == SubmitOnCtrlEnter && event.MatchesKey(k, "ctrl+q") {
		if !e.DisableSubmit {
			e.submitValue()
		}
		return
	}

	// Undo / redo
	if event.MatchesAnyKey(k, "ctrl+z", "ctrl+_") {
		e.applyUndo()
		return
	}
	if event.MatchesKey(k, "ctrl+shift+z") {
		e.applyRedo()
		return
	}

	// Tab / Shift+Tab
	if event.MatchesKey(k, "tab") {
		if e.acMode == acOff {
			e.handleTabCompletion()
		}
		return
	}
	if event.MatchesKey(k, "shift+tab") {
		// OMP: no indent outdent — ignore unless autocomplete wants it
		return
	}

	// Kill / yank
	if event.MatchesKey(k, "ctrl+k") {
		e.deleteToEndOfLine()
		return
	}
	if event.MatchesKey(k, "ctrl+u") {
		e.deleteToStartOfLine()
		return
	}
	if event.MatchesAnyKey(k, "ctrl+w", "alt+backspace", "super+alt+backspace") {
		e.deleteWordBackwards()
		return
	}
	if event.MatchesAnyKey(k, "alt+d", "alt+delete", "super+alt+d", "super+alt+delete") {
		e.deleteWordForwards()
		return
	}
	if event.MatchesKey(k, "ctrl+y") {
		e.yankFromKillRing()
		return
	}
	if event.MatchesKey(k, "alt+y") {
		e.yankPop()
		return
	}

	// Line nav
	if event.MatchesAnyKey(k, "ctrl+a", "home") {
		e.moveToLineStart()
		return
	}
	if event.MatchesAnyKey(k, "ctrl+e", "end") {
		e.moveToLineEnd()
		return
	}
	if event.MatchesAnyKey(k, "ctrl+home", "alt+<") {
		e.moveToMessageStart()
		return
	}
	if event.MatchesAnyKey(k, "ctrl+end", "alt+>") {
		e.moveToMessageEnd()
		return
	}

	// Alt+Enter
	if event.MatchesKey(k, "alt+enter") {
		if fn := e.OnAltEnter; fn != nil {
			text := strings.Join(e.lines, "\n")
			e.deferInputCallback(func() { fn(text) })
		} else {
			e.addNewLine()
		}
		return
	}

	// Newline variants: shift+enter, ctrl+enter
	if e.submitMode == SubmitOnCtrlEnter && event.MatchesKey(k, "ctrl+enter") {
		if !e.DisableSubmit {
			e.submitValue()
		}
		return
	}
	if event.MatchesAnyKey(k, "shift+enter", "ctrl+enter") {
		if e.shouldSubmitOnBackslashEnter(k) {
			e.handleBackspace()
			e.submitValue()
			return
		}
		e.addNewLine()
		return
	}

	// Plain Enter — submit
	if event.MatchesKey(k, "enter") {
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
	if event.MatchesAnyKey(k, "backspace", "shift+backspace") {
		e.handleBackspace()
		return
	}
	if event.MatchesAnyKey(k, "delete", "shift+delete") {
		e.handleForwardDelete()
		return
	}

	// Page up/down
	if event.MatchesKey(k, "pageup") {
		if e.isEmpty() {
			e.navigateHistory(-1)
		} else if e.historyIndex > -1 && e.isOnFirstVisualLine() {
			e.navigateHistory(-1)
		} else {
			e.pageScroll(-1)
		}
		return
	}
	if event.MatchesKey(k, "pagedown") {
		if e.historyIndex > -1 && e.isOnLastVisualLine() {
			e.navigateHistory(1)
		} else {
			e.pageScroll(1)
		}
		return
	}

	// Word nav
	if event.MatchesAnyKey(k, "alt+left", "ctrl+left", "alt+b") {
		e.resetKillSequence()
		e.moveWordBackwards()
		return
	}
	if event.MatchesAnyKey(k, "alt+right", "ctrl+right", "alt+f") {
		e.resetKillSequence()
		e.moveWordForwards()
		return
	}

	// Arrows
	if event.MatchesKey(k, "up") {
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
	if event.MatchesKey(k, "down") {
		if e.historyIndex > -1 && e.isOnLastVisualLine() {
			e.navigateHistory(1)
		} else if e.isOnLastVisualLine() {
			e.moveToLineEnd()
		} else {
			e.moveCursor(1, 0)
		}
		return
	}
	if event.MatchesKey(k, "right") {
		e.moveCursor(0, 1)
		return
	}
	if event.MatchesKey(k, "left") {
		e.moveCursor(0, -1)
		return
	}

	// Select-all
	if event.MatchesKey(k, "ctrl+a") {
		// already handled as line-start above (emacs). Host wanting select-all
		// can call SelectAll(). OMP editor uses ctrl+a as line start.
		return
	}

	// Shift+Space
	if event.MatchesKey(k, "shift+space") {
		e.insertCharacter(" ")
		return
	}

	// Escape cancels autocomplete (already handled) or clears selection
	if event.MatchesKey(k, "escape") {
		if e.sel.Active {
			e.clearSelection()
		}
		return
	}

	// Jump mode triggers (OMP defaults: often unbound; expose via ctrl+t / ctrl+g style)
	// No default OMP hardcodes beyond keybindings; skip unless ID matches known ones.
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
	if !event.MatchesKey(k, "enter") {
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
