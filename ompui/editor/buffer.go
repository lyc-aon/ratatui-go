package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/input"
	"github.com/lyc-aon/ratatui-go/ompui/textutil"
	"golang.org/x/text/unicode/norm"
)

func (e *Editor) withUndoSuspended(fn func()) {
	was := e.suspendUndo
	e.suspendUndo = true
	defer func() { e.suspendUndo = was }()
	fn()
}

func (e *Editor) recordUndo() {
	if e.suspendUndo {
		return
	}
	snap := e.cloneState()
	e.undoStack = append(e.undoStack, snap)
	if len(e.undoStack) > maxUndoStack {
		copy(e.undoStack, e.undoStack[1:])
		e.undoStack = e.undoStack[:maxUndoStack]
	}
	// New edit branch kills redo.
	e.redoStack = e.redoStack[:0]
}

func (e *Editor) cloneState() editorState {
	lines := make([]string, len(e.lines))
	copy(lines, e.lines)
	return editorState{
		lines:      lines,
		cursorLine: e.cursorLine,
		cursorCol:  e.cursorCol,
	}
}

func (e *Editor) applyUndo() {
	if len(e.undoStack) == 0 {
		return
	}
	// Push current onto redo.
	e.redoStack = append(e.redoStack, e.cloneState())
	if len(e.redoStack) > maxUndoStack {
		copy(e.redoStack, e.redoStack[1:])
		e.redoStack = e.redoStack[:maxUndoStack]
	}
	snap := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]

	e.historyIndex = -1
	e.historyScratch = ""
	e.resetKillSequence()
	e.preferredVisualCol = -1
	e.clearSelection()
	e.lines = snap.lines
	e.cursorLine = snap.cursorLine
	e.cursorCol = snap.cursorCol
	e.ensureLines()
	e.setCursorCol(e.cursorCol)
	e.fireChange()
	e.retriggerAutocompleteAtCursor()
}

func (e *Editor) applyRedo() {
	if len(e.redoStack) == 0 {
		return
	}
	e.undoStack = append(e.undoStack, e.cloneState())
	if len(e.undoStack) > maxUndoStack {
		copy(e.undoStack, e.undoStack[1:])
		e.undoStack = e.undoStack[:maxUndoStack]
	}
	snap := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]

	e.historyIndex = -1
	e.historyScratch = ""
	e.resetKillSequence()
	e.preferredVisualCol = -1
	e.clearSelection()
	e.lines = snap.lines
	e.cursorLine = snap.cursorLine
	e.cursorCol = snap.cursorCol
	e.ensureLines()
	e.setCursorCol(e.cursorCol)
	e.fireChange()
}

func (e *Editor) exitHistoryForEditing() {
	if e.historyIndex == -1 {
		return
	}
	// OMP: if caret still at history edit anchor (0,0), jump to end before edit.
	if e.cursorLine == 0 && e.cursorCol == 0 {
		e.cursorLine = len(e.lines) - 1
		e.setCursorCol(len(e.currentLine()))
	}
	e.historyIndex = -1
	e.historyScratch = ""
}

func (e *Editor) deleteSelectionIfAny() bool {
	if !e.sel.Active {
		return false
	}
	a, b := orderedSel(e.sel.Anchor, e.sel.Cursor)
	e.deleteRange(a, b)
	e.clearSelection()
	return true
}

func orderedSel(a, b CursorPos) (CursorPos, CursorPos) {
	if a.Line < b.Line || (a.Line == b.Line && a.Col <= b.Col) {
		return a, b
	}
	return b, a
}

func (e *Editor) sliceRange(a, b CursorPos) string {
	if a.Line == b.Line {
		line := e.lines[a.Line]
		if a.Col > len(line) {
			a.Col = len(line)
		}
		if b.Col > len(line) {
			b.Col = len(line)
		}
		if a.Col >= b.Col {
			return ""
		}
		return line[a.Col:b.Col]
	}
	var bld strings.Builder
	first := e.lines[a.Line]
	if a.Col > len(first) {
		a.Col = len(first)
	}
	bld.WriteString(first[a.Col:])
	bld.WriteByte('\n')
	for i := a.Line + 1; i < b.Line; i++ {
		bld.WriteString(e.lines[i])
		bld.WriteByte('\n')
	}
	last := e.lines[b.Line]
	if b.Col > len(last) {
		b.Col = len(last)
	}
	bld.WriteString(last[:b.Col])
	return bld.String()
}

func (e *Editor) deleteRange(a, b CursorPos) {
	if a.Line == b.Line {
		line := e.lines[a.Line]
		e.lines[a.Line] = line[:a.Col] + line[b.Col:]
		e.cursorLine = a.Line
		e.setCursorCol(a.Col)
		return
	}
	first := e.lines[a.Line]
	last := e.lines[b.Line]
	merged := first[:a.Col] + last[b.Col:]
	newLines := make([]string, 0, len(e.lines)-(b.Line-a.Line))
	newLines = append(newLines, e.lines[:a.Line]...)
	newLines = append(newLines, merged)
	newLines = append(newLines, e.lines[b.Line+1:]...)
	e.lines = newLines
	e.cursorLine = a.Line
	e.setCursorCol(a.Col)
}

func (e *Editor) insertTextAtCursor(text string, record bool) {
	if record {
		e.historyIndex = -1
		e.resetKillSequence()
		e.recordUndo()
	}
	_ = e.deleteSelectionIfAny()
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")

	if len(parts) == 1 {
		line := e.currentLine()
		col := e.cursorCol
		if col > len(line) {
			col = len(line)
		}
		e.lines[e.cursorLine] = line[:col] + normalized + line[col:]
		e.setCursorCol(col + len(normalized))
	} else {
		cur := e.currentLine()
		col := e.cursorCol
		if col > len(cur) {
			col = len(cur)
		}
		before := cur[:col]
		after := cur[col:]
		newLines := make([]string, 0, len(e.lines)+len(parts)-1)
		newLines = append(newLines, e.lines[:e.cursorLine]...)
		newLines = append(newLines, before+parts[0])
		for i := 1; i < len(parts)-1; i++ {
			newLines = append(newLines, parts[i])
		}
		last := parts[len(parts)-1]
		newLines = append(newLines, last+after)
		newLines = append(newLines, e.lines[e.cursorLine+1:]...)
		e.lines = newLines
		e.cursorLine += len(parts) - 1
		e.setCursorCol(len(last))
	}
}

func (e *Editor) insertCharacter(char string) {
	e.exitHistoryForEditing()
	hadSel := e.sel.Active
	if hadSel {
		e.recordUndo()
		e.deleteSelectionIfAny()
		e.lastAction = actionNone
	}
	isWord := allWordGraphemes(char)
	if !hadSel && (!isWord || e.lastAction != actionTypeWord) {
		e.recordUndo()
	}
	if isWord {
		e.lastAction = actionTypeWord
	} else {
		e.lastAction = actionNone
	}

	line := e.currentLine()
	col := e.cursorCol
	e.lines[e.cursorLine] = line[:col] + char + line[col:]
	e.setCursorCol(col + len(char))
	e.fireChange()

	// Sync inline replace (emoji shortcodes etc.)
	if utf8.RuneCountInString(char) == 1 {
		if p, ok := e.acProvider.(SyncInlineReplaceProvider); ok {
			replaceLine := e.currentLine()
			before := replaceLine[:e.cursorCol]
			if rep := p.TrySyncInlineReplace(before); rep != nil && rep.ReplaceLen > 0 && rep.ReplaceLen <= e.cursorCol {
				start := e.cursorCol - rep.ReplaceLen
				e.lines[e.cursorLine] = replaceLine[:start] + rep.Insert + replaceLine[e.cursorCol:]
				e.setCursorCol(start + len(rep.Insert))
				e.fireChange()
				if e.acMode != acOff {
					e.cancelAutocomplete(false)
					if e.OnAutocompleteUpdate != nil {
						e.OnAutocompleteUpdate()
					}
				}
				return
			}
		}
	}

	e.maybeTriggerAutocompleteAfterInsert(char)
}

func (e *Editor) addNewLine() {
	e.historyIndex = -1
	e.resetKillSequence()
	e.recordUndo()
	if e.deleteSelectionIfAny() {
		// ok
	}
	cur := e.currentLine()
	col := e.cursorCol
	before := cur[:col]
	after := cur[col:]
	e.lines[e.cursorLine] = before
	tail := append([]string{after}, e.lines[e.cursorLine+1:]...)
	e.lines = append(e.lines[:e.cursorLine+1], tail...)
	e.cursorLine++
	e.setCursorCol(0)
	e.fireChange()
}

func (e *Editor) handleBackspace() {
	e.historyIndex = -1
	e.resetKillSequence()
	if e.sel.Active {
		e.recordUndo()
		e.deleteSelectionIfAny()
		e.fireChange()
		e.retriggerAutocompleteAtCursor()
		return
	}
	e.recordUndo()
	if e.cursorCol > 0 {
		line := e.currentLine()
		if tok := e.atomicTokenAt(line, e.cursorCol-1); tok != nil {
			e.lines[e.cursorLine] = line[:tok.start] + line[tok.end:]
			e.setCursorCol(tok.start)
		} else {
			_, n := graphemeBefore(line, e.cursorCol)
			if n == 0 {
				n = 1
				for e.cursorCol-n > 0 && (line[e.cursorCol-n]&0xC0) == 0x80 {
					n++
				}
			}
			e.lines[e.cursorLine] = line[:e.cursorCol-n] + line[e.cursorCol:]
			e.setCursorCol(e.cursorCol - n)
		}
	} else if e.cursorLine > 0 {
		cur := e.currentLine()
		prev := e.lines[e.cursorLine-1]
		e.lines[e.cursorLine-1] = prev + cur
		e.lines = append(e.lines[:e.cursorLine], e.lines[e.cursorLine+1:]...)
		e.cursorLine--
		e.setCursorCol(len(prev))
	}
	e.fireChange()
	e.retriggerAutocompleteAtCursor()
}

func (e *Editor) handleForwardDelete() {
	e.historyIndex = -1
	e.resetKillSequence()
	if e.sel.Active {
		e.recordUndo()
		e.deleteSelectionIfAny()
		e.fireChange()
		e.retriggerAutocompleteAtCursor()
		return
	}
	e.recordUndo()
	line := e.currentLine()
	if e.cursorCol < len(line) {
		if tok := e.atomicTokenAt(line, e.cursorCol); tok != nil {
			e.lines[e.cursorLine] = line[:tok.start] + line[tok.end:]
			e.setCursorCol(tok.start)
		} else {
			_, n := graphemeAfter(line, e.cursorCol)
			if n == 0 {
				n = 1
			}
			e.lines[e.cursorLine] = line[:e.cursorCol] + line[e.cursorCol+n:]
		}
	} else if e.cursorLine < len(e.lines)-1 {
		next := e.lines[e.cursorLine+1]
		e.lines[e.cursorLine] = line + next
		e.lines = append(e.lines[:e.cursorLine+1], e.lines[e.cursorLine+2:]...)
	}
	e.fireChange()
	e.retriggerAutocompleteAtCursor()
}

func (e *Editor) deleteToStartOfLine() {
	e.historyIndex = -1
	e.recordUndo()
	if e.sel.Active {
		text := e.SelectedText()
		e.deleteSelectionIfAny()
		e.recordKill(text, true)
		e.fireChange()
		return
	}
	cur := e.currentLine()
	var deleted string
	if e.cursorCol > 0 {
		rng := e.expandRangeOverAtomicTokens(cur, 0, e.cursorCol)
		deleted = cur[:rng.end]
		e.lines[e.cursorLine] = cur[rng.end:]
		e.setCursorCol(0)
	} else if e.cursorLine > 0 {
		deleted = "\n"
		prev := e.lines[e.cursorLine-1]
		e.lines[e.cursorLine-1] = prev + cur
		e.lines = append(e.lines[:e.cursorLine], e.lines[e.cursorLine+1:]...)
		e.cursorLine--
		e.setCursorCol(len(prev))
	}
	e.recordKill(deleted, true)
	e.fireChange()
}

func (e *Editor) deleteToEndOfLine() {
	e.historyIndex = -1
	e.recordUndo()
	if e.sel.Active {
		text := e.SelectedText()
		e.deleteSelectionIfAny()
		e.recordKill(text, false)
		e.fireChange()
		return
	}
	cur := e.currentLine()
	var deleted string
	if e.cursorCol < len(cur) {
		rng := e.expandRangeOverAtomicTokens(cur, e.cursorCol, len(cur))
		deleted = cur[rng.start:]
		e.lines[e.cursorLine] = cur[:rng.start]
		if rng.start < e.cursorCol {
			e.setCursorCol(rng.start)
		}
	} else if e.cursorLine < len(e.lines)-1 {
		next := e.lines[e.cursorLine+1]
		deleted = "\n"
		e.lines[e.cursorLine] = cur + next
		e.lines = append(e.lines[:e.cursorLine+1], e.lines[e.cursorLine+2:]...)
	}
	e.recordKill(deleted, false)
	e.fireChange()
}

func (e *Editor) deleteWordBackwards() {
	e.historyIndex = -1
	e.recordUndo()
	if e.sel.Active {
		text := e.SelectedText()
		e.deleteSelectionIfAny()
		e.recordKill(text, true)
		e.fireChange()
		return
	}
	cur := e.currentLine()
	if e.cursorCol == 0 {
		if e.cursorLine > 0 {
			e.recordKill("\n", true)
			prev := e.lines[e.cursorLine-1]
			e.lines[e.cursorLine-1] = prev + cur
			e.lines = append(e.lines[:e.cursorLine], e.lines[e.cursorLine+1:]...)
			e.cursorLine--
			e.setCursorCol(len(prev))
		}
	} else {
		old := e.cursorCol
		e.moveWordBackwards()
		rng := e.expandRangeOverAtomicTokens(cur, e.cursorCol, old)
		deleted := cur[rng.start:rng.end]
		e.lines[e.cursorLine] = cur[:rng.start] + cur[rng.end:]
		e.setCursorCol(rng.start)
		e.recordKill(deleted, true)
	}
	e.fireChange()
}

func (e *Editor) deleteWordForwards() {
	e.historyIndex = -1
	e.recordUndo()
	if e.sel.Active {
		text := e.SelectedText()
		e.deleteSelectionIfAny()
		e.recordKill(text, false)
		e.fireChange()
		return
	}
	cur := e.currentLine()
	if e.cursorCol >= len(cur) {
		if e.cursorLine < len(e.lines)-1 {
			e.recordKill("\n", false)
			next := e.lines[e.cursorLine+1]
			e.lines[e.cursorLine] = cur + next
			e.lines = append(e.lines[:e.cursorLine+1], e.lines[e.cursorLine+2:]...)
		}
	} else {
		old := e.cursorCol
		e.moveWordForwards()
		rng := e.expandRangeOverAtomicTokens(cur, old, e.cursorCol)
		deleted := cur[rng.start:rng.end]
		e.lines[e.cursorLine] = cur[:rng.start] + cur[rng.end:]
		e.setCursorCol(rng.start)
		e.recordKill(deleted, false)
	}
	e.fireChange()
}

func (e *Editor) recordKill(text string, backward bool) {
	if text == "" {
		return
	}
	e.killRing.Push(text, backward, e.lastAction == actionKill)
	e.lastAction = actionKill
}

func (e *Editor) yankFromKillRing() {
	text, ok := e.killRing.Peek()
	if !ok {
		return
	}
	e.insertTextAtCursor(text, true)
	e.lastAction = actionYank
	e.fireChange()
}

func (e *Editor) yankPop() {
	if e.lastAction != actionYank {
		return
	}
	if e.killRing.Len() <= 1 {
		return
	}
	e.historyIndex = -1
	e.recordUndo()
	e.withUndoSuspended(func() {
		if !e.deleteYankedText() {
			return
		}
		e.killRing.Rotate()
		if text, ok := e.killRing.Peek(); ok {
			e.insertTextAtCursor(text, false)
		}
	})
	e.lastAction = actionYank
	e.fireChange()
}

func (e *Editor) deleteYankedText() bool {
	yanked, ok := e.killRing.Peek()
	if !ok {
		return false
	}
	yankLines := strings.Split(yanked, "\n")
	endLine := e.cursorLine
	endCol := e.cursorCol
	startLine := endLine - (len(yankLines) - 1)
	if startLine < 0 {
		return false
	}
	if len(yankLines) == 1 {
		line := e.lines[endLine]
		startCol := endCol - len(yanked)
		if startCol < 0 {
			return false
		}
		if line[startCol:endCol] != yanked {
			return false
		}
		e.lines[endLine] = line[:startCol] + line[endCol:]
		e.cursorLine = endLine
		e.setCursorCol(startCol)
		return true
	}
	firstIns := yankLines[0]
	lastIns := yankLines[len(yankLines)-1]
	firstLine := e.lines[startLine]
	lastLine := e.lines[endLine]
	if !strings.HasSuffix(firstLine, firstIns) {
		return false
	}
	if endCol != len(lastIns) {
		return false
	}
	if lastLine[:endCol] != lastIns {
		return false
	}
	startCol := len(firstLine) - len(firstIns)
	if startCol < 0 {
		return false
	}
	suffix := lastLine[endCol:]
	newLine := firstLine[:startCol] + suffix
	// splice
	head := append([]string{}, e.lines[:startLine]...)
	head = append(head, newLine)
	head = append(head, e.lines[endLine+1:]...)
	e.lines = head
	e.cursorLine = startLine
	e.setCursorCol(startCol)
	return true
}

// --- paste ---

func (e *Editor) handlePaste(pasted string) {
	filtered := e.sanitizePastedText(pasted)

	// Path paste: prepend space after word char.
	if filtered != "" {
		switch filtered[0] {
		case '/', '~', '.':
			cur := e.currentLine()
			if e.cursorCol > 0 {
				r, _ := utf8.DecodeLastRuneInString(cur[:e.cursorCol])
				if r != utf8.RuneError && (textutil.Kind(string(r)) == textutil.WordText || r == '_') {
					filtered = " " + filtered
				}
			}
		}
	}

	pastedLines := strings.Split(filtered, "\n")
	totalChars := len(filtered)
	isMarker := len(pastedLines) > pasteMarkerLines || totalChars > pasteMarkerChars

	if isMarker && e.OnLargePaste != nil && e.OnLargePaste(filtered, len(pastedLines)) {
		return
	}

	e.historyIndex = -1
	e.resetKillSequence()
	e.recordUndo()
	e.withUndoSuspended(func() {
		if isMarker {
			e.storePasteMarker(filtered, len(pastedLines))
			return
		}
		if filtered != "" {
			e.insertTextAtCursor(filtered, true)
			if len(pastedLines) == 1 {
				e.retriggerAutocompleteAtCursor()
			}
		}
	})
	e.fireChange()
}

func (e *Editor) sanitizePastedText(pasted string) string {
	decoded := input.DecodeReencodedPasteControls(pasted)
	// CRLF → LF, NFC, tabs → 3 spaces, strip C0 except \n.
	normalized := norm.NFC.String(strings.ToValidUTF8(decoded, ""))
	var b strings.Builder
	b.Grow(len(normalized))
	for i := 0; i < len(normalized); {
		c := normalized[i]
		if c == '\r' {
			if i+1 < len(normalized) && normalized[i+1] == '\n' {
				i++
			}
			b.WriteByte('\n')
			i++
			continue
		}
		if c == '\t' {
			b.WriteString("   ")
			i++
			continue
		}
		if c == '\n' {
			b.WriteByte('\n')
			i++
			continue
		}
		if c < 0x20 {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(normalized[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		b.WriteString(normalized[i : i+size])
		i += size
	}
	return b.String()
}

func (e *Editor) storePasteMarker(content string, lineCount int) {
	e.pasteCounter++
	id := e.pasteCounter
	e.pastes[id] = content
	var marker string
	if lineCount > pasteMarkerLines {
		marker = "[Paste #" + itoa(id) + ", +" + itoa(lineCount) + " lines]"
	} else {
		marker = "[Paste #" + itoa(id) + ", " + itoa(len(content)) + " chars]"
	}
	e.insertTextAtCursor(marker, true)
}

func (e *Editor) expandPasteMarkers(text string) string {
	if len(e.pastes) == 0 {
		return text
	}
	result := text
	for id, content := range e.pastes {
		// Match [Paste #N], [Paste #N, +M lines], [Paste #N, M chars]
		prefix := "[Paste #" + itoa(id)
		for {
			idx := strings.Index(result, prefix)
			if idx < 0 {
				break
			}
			end := strings.IndexByte(result[idx:], ']')
			if end < 0 {
				break
			}
			end = idx + end + 1
			result = result[:idx] + content + result[end:]
		}
	}
	return result
}

func (e *Editor) submitValue() {
	e.resetKillSequence()
	result := strings.TrimSpace(e.expandPasteMarkers(strings.Join(e.lines, "\n")))
	e.lines = []string{""}
	e.cursorLine = 0
	e.cursorCol = 0
	e.clearSelection()
	clear(e.pastes)
	e.pasteCounter = 0
	e.historyIndex = -1
	e.historyScratch = ""
	e.scrollOffset = 0
	e.undoStack = e.undoStack[:0]
	e.redoStack = e.redoStack[:0]
	e.cancelAutocomplete(false)
	if fn := e.OnChange; fn != nil {
		e.deferInputCallback(func() { fn("") })
	}
	if fn := e.OnSubmit; fn != nil {
		e.deferInputCallback(func() { fn(result) })
	}
}

// --- atomic tokens ---

type byteRange struct{ start, end int }

func (e *Editor) atomicTokenAt(line string, col int) *byteRange {
	if e.atomicRe == nil || col < 0 || col >= len(line) {
		// also allow col == len for end checks? OMP uses col inside token.
		if e.atomicRe == nil {
			return nil
		}
		if col < 0 {
			return nil
		}
	}
	if e.atomicRe == nil {
		return nil
	}
	idxs := e.atomicRe.FindAllStringIndex(line, -1)
	for _, m := range idxs {
		if m[1] <= m[0] {
			continue
		}
		if col < m[0] {
			break
		}
		if col < m[1] {
			return &byteRange{start: m[0], end: m[1]}
		}
	}
	return nil
}

func (e *Editor) expandRangeOverAtomicTokens(line string, start, end int) byteRange {
	if start < 0 {
		start = 0
	}
	if end > len(line) {
		end = len(line)
	}
	if tok := e.atomicTokenAt(line, start); tok != nil && tok.start < start {
		start = tok.start
	}
	if end > start {
		if tok := e.atomicTokenAt(line, end-1); tok != nil && tok.end > end {
			end = tok.end
		}
	}
	return byteRange{start: start, end: end}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
