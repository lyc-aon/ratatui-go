package interact

import (
	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/killring"
	"github.com/lyc-aon/ratatui-go/ompui/textutil"
	"github.com/rivo/uniseg"
)

type inputState struct {
	value  string
	cursor int
}

type inputLastAction uint8

const (
	inputActionNone inputLastAction = iota
	inputActionKill
	inputActionYank
	inputActionTypeWord
)

// Input is a single-line text field with horizontal scrolling, kill-ring,
// undo, paste sanitization, and focus-aware cursor marker.
type Input struct {
	component.FocusState

	value  string
	cursor int // byte offset into value

	// Prompt is rendered before the editable area. Default "> ".
	Prompt string

	// OnSubmit fires on enter with the current value.
	OnSubmit func(value string)
	// OnEscape fires on cancel (escape / ctrl+c).
	OnEscape func()

	killRing   killring.Ring
	lastAction inputLastAction
	undoStack  []inputState

	gen    component.Gen
	cached component.Frame
	cacheW int
	dirty  bool
}

// NewInput constructs an empty single-line input with default prompt "> ".
func NewInput() *Input {
	return &Input{
		Prompt: "> ",
		dirty:  true,
	}
}

// Value returns the current text.
func (in *Input) Value() string { return in.value }

// SetValue replaces the text and moves the cursor to the end.
func (in *Input) SetValue(value string) {
	in.value = value
	in.cursor = len(value)
	in.lastAction = inputActionNone
	in.dirty = true
}

// Cursor returns the byte offset of the caret.
func (in *Input) Cursor() int { return in.cursor }

// SetCursor sets the caret byte offset (clamped to grapheme-safe range via clamp).
func (in *Input) SetCursor(offset int) {
	in.cursor = clampInt(offset, 0, len(in.value))
	in.lastAction = inputActionNone
	in.dirty = true
}

// PasteText inserts sanitized paste content at the cursor (non-bracketed path).
func (in *Input) PasteText(text string) {
	in.handlePaste(text)
}

// HandleInput implements component.InputHandler.
func (in *Input) HandleInput(ev event.Event) {
	switch ev.Kind {
	case event.KindPaste:
		in.handlePaste(ev.Paste)
		return
	case event.KindText:
		if IsEnter(ev) {
			if in.OnSubmit != nil {
				in.OnSubmit(in.value)
			}
			return
		}
		if t := PrintableText(ev); t != "" {
			in.insertText(t)
		}
		return
	case event.KindKey:
		// fall through
	default:
		return
	}
	if ev.Key.Action == event.ActionRelease {
		return
	}

	if IsCancel(ev) {
		if in.OnEscape != nil {
			in.OnEscape()
		}
		return
	}
	if matchKeys(ev, keysUndo) {
		in.undo()
		return
	}
	if IsEnter(ev) {
		if in.OnSubmit != nil {
			in.OnSubmit(in.value)
		}
		return
	}
	if matchKeys(ev, keysDeleteCharBackward) {
		in.backspace()
		return
	}
	if matchKeys(ev, keysDeleteCharForward) {
		in.forwardDelete()
		return
	}
	if matchKeys(ev, keysDeleteWordBackward) {
		in.deleteWordBackwards()
		return
	}
	if matchKeys(ev, keysDeleteWordForward) {
		in.deleteWordForward()
		return
	}
	if matchKeys(ev, keysDeleteToLineStart) {
		in.deleteToLineStart()
		return
	}
	if matchKeys(ev, keysDeleteToLineEnd) {
		in.deleteToLineEnd()
		return
	}
	if matchKeys(ev, keysYank) {
		in.yank()
		return
	}
	if matchKeys(ev, keysYankPop) {
		in.yankPop()
		return
	}
	if matchKeys(ev, keysCursorLeft) {
		in.lastAction = inputActionNone
		if in.cursor > 0 {
			in.cursor -= prevGraphemeByteLen(in.value, in.cursor)
		}
		in.dirty = true
		return
	}
	if matchKeys(ev, keysCursorRight) {
		in.lastAction = inputActionNone
		if in.cursor < len(in.value) {
			in.cursor += nextGraphemeByteLen(in.value, in.cursor)
		}
		in.dirty = true
		return
	}
	if matchKeys(ev, keysCursorLineStart) {
		in.lastAction = inputActionNone
		in.cursor = 0
		in.dirty = true
		return
	}
	if matchKeys(ev, keysCursorLineEnd) {
		in.lastAction = inputActionNone
		in.cursor = len(in.value)
		in.dirty = true
		return
	}
	if matchKeys(ev, keysCursorWordLeft) {
		in.moveWordBackwards()
		return
	}
	if matchKeys(ev, keysCursorWordRight) {
		in.moveWordForwards()
		return
	}
	if t := PrintableText(ev); t != "" {
		in.insertText(t)
	}
}

// Invalidate drops render cache.
func (in *Input) Invalidate() {
	in.dirty = true
	in.cached = component.Frame{}
	in.cacheW = -1
}

// Dispose implements component.Disposable.
func (in *Input) Dispose() {
	in.OnSubmit = nil
	in.OnEscape = nil
	in.undoStack = nil
	in.killRing.Reset()
}

// Render implements component.Component.
func (in *Input) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	// Focus / cursor mode affect output even without dirty flag.
	line, cursorCol := in.renderLine(width)
	lines := []string{line}
	var cur *component.Cursor
	if in.Focused() {
		cur = &component.Cursor{Row: 0, Column: cursorCol}
	}
	changed := in.dirty || in.cacheW != width ||
		!sameLines(in.cached.Lines, lines) ||
		!cursorEqual(in.cached.Cursor, cur)
	gen := in.gen.Touch(changed)
	in.dirty = false
	in.cacheW = width
	if !changed && in.cached.Lines != nil {
		return in.cached
	}
	fr := component.NewFrame(lines, gen).WithCursor(cur)
	in.cached = fr
	return fr
}

func cursorEqual(a, b *component.Cursor) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Row == b.Row && a.Column == b.Column
}

func (in *Input) renderLine(width int) (line string, cursorCol int) {
	prompt := in.Prompt
	promptW := ansitext.VisibleWidth(prompt)
	available := width - promptW
	if available <= 0 {
		return prompt, promptW
	}

	cursorIndex := clampInt(in.cursor, 0, len(in.value))
	displayValue := in.value
	if cursorIndex >= len(in.value) {
		displayValue = in.value + " "
	}

	totalCols := ansitext.VisibleWidth(displayValue)
	cursorCols := ansitext.VisibleWidth(displayValue[:minInt(cursorIndex, len(displayValue))])
	// Width of grapheme at cursor.
	cursorG, _ := plainSlice(displayValue[minInt(cursorIndex, len(displayValue)):], 0, 1024, false)
	// first grapheme only
	cg := firstGrapheme(displayValue[minInt(cursorIndex, len(displayValue)):])
	if cg == "" {
		cg = " "
	}
	cursorGWidth := ansitext.VisibleWidth(cg)
	_ = cursorG

	maxStart := maxInt(0, totalCols-available)
	startCol := 0
	if totalCols > available {
		half := available / 2
		startCol = clampInt(cursorCols-half, 0, maxStart)
		maxCursorRel := maxInt(0, available-cursorGWidth)
		cursorRel := cursorCols - startCol
		if cursorRel > maxCursorRel {
			startCol = clampInt(cursorCols-maxCursorRel, 0, maxStart)
		}
	}

	visibleText, _ := plainSlice(displayValue, startCol, available, true)
	prefixText, _ := plainSlice(displayValue, startCol, maxInt(0, cursorCols-startCol), true)

	// Align cursorDisplay to grapheme boundary inside visibleText.
	cursorDisplay := len(prefixText)
	if cursorDisplay > len(visibleText) {
		cursorDisplay = len(visibleText)
	}
	beforeCursor := visibleText[:cursorDisplay]
	rest := visibleText[cursorDisplay:]
	atCursor := firstGrapheme(rest)
	afterCursor := ""
	if len(atCursor) < len(rest) {
		afterCursor = rest[len(atCursor):]
	}

	marker := ""
	if in.Focused() {
		marker = component.CursorMarker
	}
	var cursorChar string
	if in.UseTerminalCursor() {
		cursorChar = atCursor
	} else {
		cursorChar = invertCell(atCursor)
	}

	beforeWidth := ansitext.VisibleWidth(beforeCursor)
	cursorWidth := ansitext.VisibleWidth(atCursor)
	if !in.UseTerminalCursor() {
		cursorWidth = ansitext.VisibleWidth(atCursor)
		if atCursor == "" {
			cursorWidth = 1
		}
	}
	remainingAfter := maxInt(0, available-beforeWidth-cursorWidth)
	clampedAfter, _ := plainSlice(afterCursor, 0, remainingAfter, true)

	textWithCursor := beforeCursor + marker + cursorChar + clampedAfter
	renderedNoMarker := beforeCursor + cursorChar + clampedAfter
	visualLength := ansitext.VisibleWidth(renderedNoMarker)
	pad := padSpaces(maxInt(0, available-visualLength))
	line = prompt + textWithCursor + pad
	cursorCol = promptW + beforeWidth
	return line, cursorCol
}

func firstGrapheme(s string) string {
	if s == "" {
		return ""
	}
	gr := uniseg.NewGraphemes(s)
	if !gr.Next() {
		return ""
	}
	return gr.Str()
}

func (in *Input) insertText(text string) {
	isWordChunk := true
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		if textutil.Kind(gr.Str()) == textutil.WordWhitespace {
			isWordChunk = false
			break
		}
	}
	if !isWordChunk || in.lastAction != inputActionTypeWord {
		in.pushUndo()
	}
	in.lastAction = inputActionTypeWord
	in.value = in.value[:in.cursor] + text + in.value[in.cursor:]
	in.cursor += len(text)
	in.dirty = true
}

func (in *Input) backspace() {
	in.lastAction = inputActionNone
	if in.cursor <= 0 {
		return
	}
	in.pushUndo()
	n := prevGraphemeByteLen(in.value, in.cursor)
	in.value = in.value[:in.cursor-n] + in.value[in.cursor:]
	in.cursor -= n
	in.dirty = true
}

func (in *Input) forwardDelete() {
	in.lastAction = inputActionNone
	if in.cursor >= len(in.value) {
		return
	}
	in.pushUndo()
	n := nextGraphemeByteLen(in.value, in.cursor)
	in.value = in.value[:in.cursor] + in.value[in.cursor+n:]
	in.dirty = true
}

func (in *Input) deleteToLineStart() {
	if in.cursor == 0 {
		return
	}
	in.pushUndo()
	deleted := in.value[:in.cursor]
	in.killRing.Push(deleted, true, in.lastAction == inputActionKill)
	in.lastAction = inputActionKill
	in.value = in.value[in.cursor:]
	in.cursor = 0
	in.dirty = true
}

func (in *Input) deleteToLineEnd() {
	if in.cursor >= len(in.value) {
		return
	}
	in.pushUndo()
	deleted := in.value[in.cursor:]
	in.killRing.Push(deleted, false, in.lastAction == inputActionKill)
	in.lastAction = inputActionKill
	in.value = in.value[:in.cursor]
	in.dirty = true
}

func (in *Input) deleteWordBackwards() {
	if in.cursor == 0 {
		return
	}
	wasKill := in.lastAction == inputActionKill
	in.pushUndo()
	old := in.cursor
	in.moveWordBackwards()
	from := in.cursor
	in.cursor = old
	deleted := in.value[from:in.cursor]
	in.killRing.Push(deleted, true, wasKill)
	in.lastAction = inputActionKill
	in.value = in.value[:from] + in.value[in.cursor:]
	in.cursor = from
	in.dirty = true
}

func (in *Input) deleteWordForward() {
	if in.cursor >= len(in.value) {
		return
	}
	wasKill := in.lastAction == inputActionKill
	in.pushUndo()
	old := in.cursor
	in.moveWordForwards()
	to := in.cursor
	in.cursor = old
	deleted := in.value[in.cursor:to]
	in.killRing.Push(deleted, false, wasKill)
	in.lastAction = inputActionKill
	in.value = in.value[:in.cursor] + in.value[to:]
	in.dirty = true
}

func (in *Input) yank() {
	text, ok := in.killRing.Peek()
	if !ok || text == "" {
		return
	}
	in.pushUndo()
	in.value = in.value[:in.cursor] + text + in.value[in.cursor:]
	in.cursor += len(text)
	in.lastAction = inputActionYank
	in.dirty = true
}

func (in *Input) yankPop() {
	if in.lastAction != inputActionYank || in.killRing.Len() <= 1 {
		return
	}
	in.pushUndo()
	prev, _ := in.killRing.Peek()
	if len(prev) > in.cursor {
		prev = prev[:in.cursor]
	}
	in.value = in.value[:in.cursor-len(prev)] + in.value[in.cursor:]
	in.cursor -= len(prev)
	in.killRing.Rotate()
	text, _ := in.killRing.Peek()
	in.value = in.value[:in.cursor] + text + in.value[in.cursor:]
	in.cursor += len(text)
	in.lastAction = inputActionYank
	in.dirty = true
}

func (in *Input) pushUndo() {
	in.undoStack = append(in.undoStack, inputState{value: in.value, cursor: in.cursor})
}

func (in *Input) undo() {
	n := len(in.undoStack)
	if n == 0 {
		return
	}
	snap := in.undoStack[n-1]
	in.undoStack = in.undoStack[:n-1]
	in.value = snap.value
	in.cursor = snap.cursor
	in.lastAction = inputActionNone
	in.dirty = true
}

func (in *Input) moveWordBackwards() {
	if in.cursor == 0 {
		return
	}
	in.lastAction = inputActionNone
	in.cursor = textutil.MoveWordLeft(in.value, in.cursor)
	in.dirty = true
}

func (in *Input) moveWordForwards() {
	if in.cursor >= len(in.value) {
		return
	}
	in.lastAction = inputActionNone
	in.cursor = textutil.MoveWordRight(in.value, in.cursor)
	in.dirty = true
}

func (in *Input) handlePaste(pasted string) {
	in.lastAction = inputActionNone
	in.pushUndo()
	clean := cleanPasteText(pasted)
	in.value = in.value[:in.cursor] + clean + in.value[in.cursor:]
	in.cursor += len(clean)
	in.dirty = true
}
