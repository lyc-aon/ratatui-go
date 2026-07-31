package editor

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
)

// box drawing (rounded)
const (
	boxTL = "╭"
	boxTR = "╮"
	boxBL = "╰"
	boxBR = "╯"
	boxH  = "─"
	boxV  = "│"
)

// Render implements component.Component.
//
// Returns a width-correct Frame with at most one hardware cursor when focused
// and not showing autocomplete. Live-region seams cover the whole editor so
// the transcript never commits mutable input rows.
func (e *Editor) Render(width int) component.Frame {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureLines()

	if width < 1 {
		width = 1
	}

	paddingX := e.paddingX
	if paddingX < 0 {
		paddingX = 0
	}
	borderVisible := e.borderVisible
	promptGutter := e.getPromptGutter(width, paddingX)
	contentAreaWidth := e.getContentWidth(width, paddingX)
	layoutWidth := e.getLayoutWidth(width, paddingX)
	e.lastLayoutWidth = layoutWidth

	layoutLines := e.layoutText(layoutWidth)
	// Selection painting on layout lines
	e.applySelectionToLayout(layoutLines)

	visibleH := e.visibleContentHeight(len(layoutLines))
	e.updateScrollOffset(layoutWidth, layoutLines, visibleH)
	visEnd := e.scrollOffset + visibleH
	if visEnd > len(layoutLines) {
		visEnd = len(layoutLines)
	}
	visible := layoutLines[e.scrollOffset:visEnd]

	bc := e.borderColor
	if bc == nil {
		bc = func(s string) string { return s }
	}

	result := make([]string, 0, len(visible)+4)
	var cursor *component.Cursor
	emitMarker := e.Focused() && e.acMode == acOff
	useTermCursor := e.UseTerminalCursor()
	lineContentWidth := contentAreaWidth

	if borderVisible {
		borderW := e.horizontalChromeWidth(paddingX)
		topLeft := bc(boxTL + strings.Repeat(boxH, paddingX))
		topRight := bc(strings.Repeat(boxH, paddingX) + boxTR)
		topFill := max(0, width-borderW*2)
		if e.topBorderContent != "" {
			sw := e.topBorderWidth
			if sw <= 0 {
				sw = ansitext.VisibleWidth(e.topBorderContent)
			}
			if sw <= topFill {
				fill := topFill - sw
				result = append(result, topLeft+e.topBorderContent+bc(strings.Repeat(boxH, fill))+topRight)
			} else {
				trunc := ansitext.TruncateToWidth(e.topBorderContent, max(0, topFill-1), "…")
				tw := ansitext.VisibleWidth(trunc)
				fill := max(0, topFill-tw)
				result = append(result, topLeft+trunc+bc(strings.Repeat(boxH, fill))+topRight)
			}
		} else {
			result = append(result, topLeft+bc(strings.Repeat(boxH, topFill))+topRight)
		}
	}

	hint := e.inlineHint()
	hintStyle := func(t string) string { return "\x1b[2m" + t + "\x1b[0m" }
	placeholderActive := e.isEmpty() && e.placeholder != "" && !e.sel.Active

	for vi, ll := range visible {
		displayText := ll.text
		displayWidth := ansitext.VisibleWidth(displayText)
		cursorInPadding := false
		decorated := false

		gutterText := ""
		if promptGutter != nil {
			if vi == 0 && e.scrollOffset == 0 {
				gutterText = promptGutter.firstLine
			} else {
				gutterText = promptGutter.continuation
			}
		}

		hasCursor := ll.hasCursor && e.Focused()
		marker := ""
		if hasCursor && emitMarker {
			marker = component.CursorMarker
		}

		// Placeholder ghost text
		if placeholderActive && hasCursor && displayText == "" {
			ph := ansitext.TruncateToWidth(e.placeholder, max(0, lineContentWidth), "…")
			displayText = hintStyle(ph)
			displayWidth = ansitext.VisibleWidth(ph)
			// cursor still at col 0
		}

		if !borderVisible && displayWidth > lineContentWidth {
			displayText = ansitext.SliceByColumn(displayText, 0, lineContentWidth)
			displayWidth = ansitext.VisibleWidth(displayText)
		}

		// Selection reverse-video on this layout line
		if ll.selStart >= 0 && ll.selEnd > ll.selStart && ll.selStart <= len(ll.text) {
			end := ll.selEnd
			if end > len(ll.text) {
				end = len(ll.text)
			}
			start := ll.selStart
			if start < 0 {
				start = 0
			}
			// Only when not placeholder
			if !placeholderActive {
				sel := ll.text[start:end]
				displayText = ll.text[:start] + "\x1b[7m" + sel + "\x1b[0m" + ll.text[end:]
				// width unchanged
			}
		}

		if hasCursor && useTermCursor {
			// Hardware cursor: embed marker at caret byte offset; Frame.Cursor also set.
			cpos := ll.cursorPos
			if cpos > len(ll.text) {
				cpos = len(ll.text)
			}
			before := ""
			after := ""
			if !placeholderActive {
				// Use original plain text offsets when selection styling present —
				// cursorPos refers to plain ll.text.
				before = ll.text[:cpos]
				after = ll.text[cpos:]
				// Re-apply selection around marker carefully: use plain + marker + rest,
				// re-paint selection below if needed. Simple path: marker in plain text.
				if marker != "" {
					if after == "" && hint != "" {
						avail := max(0, lineContentWidth-ansitext.VisibleWidth(before))
						ht := hintStyle(ansitext.TruncateToWidth(hint, avail, ""))
						displayText = e.decorate(before) + marker + ht
						displayWidth = ansitext.VisibleWidth(before) + min(ansitext.VisibleWidth(hint), avail)
						decorated = true
					} else if after == "" && !borderVisible && ansitext.VisibleWidth(before) >= lineContentWidth {
						displayText = e.renderTerminalCursorMarker(before, marker, lineContentWidth)
						decorated = true
					} else {
						displayText = e.decorate(before) + marker + e.decorate(after)
						// re-apply selection is approximate; selection paint already applied above on displayText.
						// Prefer marker path on plain:
						displayText = before + marker + after
						if ll.selStart >= 0 {
							displayText = paintSelectionWithMarker(ll.text, ll.selStart, ll.selEnd, cpos, marker)
						}
						displayText = e.decorate(displayText)
						decorated = true
					}
				}
			} else if marker != "" {
				displayText = marker + displayText
			}
			// Frame cursor position
			row := len(result)
			col := ansitext.VisibleWidth(gutterText) + visualColAtOffset(ll.text, cpos)
			if borderVisible {
				col += 1 + paddingX // vertical border + pad
			}
			cursor = &component.Cursor{Row: row, Column: col}
		} else if hasCursor && !useTermCursor {
			cpos := ll.cursorPos
			if cpos > len(ll.text) {
				cpos = len(ll.text)
			}
			before := ll.text[:cpos]
			after := ll.text[cpos:]
			if after != "" {
				g, n := graphemeAfter(after, 0)
				if n == 0 {
					g = after[:1]
					n = 1
				}
				rest := after[n:]
				cur := "\x1b[7m" + g + "\x1b[0m"
				displayText = e.decorate(before) + marker + cur + e.decorate(rest)
				decorated = true
			} else if e.cursorOverride != "" {
				ow := e.cursorOverrideW
				if ow <= 0 {
					ow = ansitext.VisibleWidth(e.cursorOverride)
					if ow <= 0 {
						ow = 1
					}
				}
				if !borderVisible && displayWidth+ow > lineContentWidth {
					wl := e.renderEOLCursorAtLimit(before, marker, lineContentWidth, e.cursorOverride, ow)
					displayText = wl.text
					displayWidth = wl.width
				} else if hint != "" {
					avail := max(0, lineContentWidth-displayWidth-ow)
					ht := hintStyle(ansitext.TruncateToWidth(hint, avail, ""))
					displayText = before + marker + e.cursorOverride + ht
					displayWidth += ow + min(ansitext.VisibleWidth(hint), avail)
				} else {
					displayText = before + marker + e.cursorOverride
					displayWidth += ow
				}
			} else {
				cw := ansitext.VisibleWidth(e.cursorGlyph)
				if cw <= 0 {
					cw = 1
				}
				if !borderVisible && displayWidth+cw > lineContentWidth {
					wl := e.renderEOLCursorAtLimit(before, marker, lineContentWidth, "", 0)
					displayText = wl.text
					displayWidth = wl.width
				} else if hint != "" {
					avail := max(0, lineContentWidth-displayWidth-cw)
					ht := hintStyle(ansitext.TruncateToWidth(hint, avail, ""))
					displayText = before + marker + e.cursorGlyph + ht
					displayWidth += cw + min(ansitext.VisibleWidth(hint), avail)
				} else {
					displayText = before + marker + e.cursorGlyph
					displayWidth += cw
				}
				if displayWidth > lineContentWidth && paddingX > 0 {
					cursorInPadding = true
				}
			}
		}

		if !decorated {
			displayText = e.decorate(displayText)
		}

		linePad := strings.Repeat(" ", max(0, lineContentWidth-displayWidth))

		if !borderVisible {
			result = append(result, gutterText+displayText+linePad)
			continue
		}

		isLast := vi == len(visible)-1
		rightPadW := max(0, paddingX)
		if cursorInPadding {
			rightPadW = max(0, paddingX-1)
		}
		if isLast {
			// bottom corners on last content row (OMP style)
			bottomLeft := bc(boxBL + boxH + strings.Repeat(" ", max(0, paddingX-1)))
			brPad := max(0, paddingX-1)
			if cursorInPadding {
				brPad = max(0, paddingX-2)
			}
			bottomRight := bc(strings.Repeat(" ", brPad) + boxH + boxBR)
			result = append(result, bottomLeft+displayText+linePad+bottomRight)
		} else {
			left := bc(boxV + strings.Repeat(" ", paddingX))
			right := bc(strings.Repeat(" ", rightPadW) + boxV)
			result = append(result, left+displayText+linePad+right)
		}
	}

	// Ensure at least one content row
	if len(result) == 0 || (borderVisible && len(result) == 1) {
		// empty content with border: need a bottom row
		if borderVisible && len(layoutLines) == 0 {
			// shouldn't happen
		}
	}
	if !borderVisible && len(result) == 0 {
		result = append(result, "")
	}

	// Autocomplete dropdown
	if e.acMode != acOff {
		acLines := e.renderAutocompleteLines(width)
		result = append(result, acLines...)
	}

	gen := e.gen.Current()
	if gen == 0 {
		gen = e.gen.Next()
	}
	frame := component.NewFrame(result, gen)
	// Entire editor is live — never commit input rows to scrollback.
	frame = frame.WithSeams(0, 0, len(result))
	if cursor != nil {
		frame = frame.WithCursor(cursor)
	}
	e.lastFrame = frame
	e.lastFrameOK = true
	return frame
}

type gutterInfo struct {
	firstLine    string
	continuation string
	width        int
}

func (e *Editor) horizontalChromeWidth(paddingX int) int {
	if e.borderVisible {
		return paddingX + 1
	}
	return 0
}

func (e *Editor) getPromptGutterWidth(width, paddingX int) int {
	if e.borderVisible || e.promptGutter == "" {
		return 0
	}
	chrome := 2 * e.horizontalChromeWidth(paddingX)
	avail := max(0, width-chrome)
	return min(ansitext.VisibleWidth(e.promptGutter), avail)
}

func (e *Editor) getPromptGutter(width, paddingX int) *gutterInfo {
	if e.borderVisible || e.promptGutter == "" {
		return nil
	}
	gw := e.getPromptGutterWidth(width, paddingX)
	if gw == 0 {
		return nil
	}
	return &gutterInfo{
		firstLine:    ansitext.SliceByColumn(e.promptGutter, 0, gw),
		continuation: strings.Repeat(" ", gw),
		width:        gw,
	}
}

func (e *Editor) getContentWidth(width, paddingX int) int {
	chrome := 2 * e.horizontalChromeWidth(paddingX)
	return max(0, width-chrome-e.getPromptGutterWidth(width, paddingX))
}

func (e *Editor) getLayoutWidth(width, paddingX int) int {
	cw := e.getContentWidth(width, paddingX)
	cursorReserve := 0
	if e.borderVisible && paddingX == 0 {
		cursorReserve = 1
	}
	return max(1, cw-cursorReserve)
}

func (e *Editor) updateScrollOffset(layoutWidth int, layout []layoutLine, visibleH int) {
	if len(layout) <= visibleH {
		e.scrollOffset = 0
		return
	}
	visual := e.buildVisualLineMap(layoutWidth)
	cur := e.findCurrentVisualLine(visual)
	if cur < e.scrollOffset {
		e.scrollOffset = cur
	} else if cur >= e.scrollOffset+visibleH {
		e.scrollOffset = cur - visibleH + 1
	}
	maxOff := max(0, len(layout)-visibleH)
	if e.scrollOffset > maxOff {
		e.scrollOffset = maxOff
	}
	if e.scrollOffset < 0 {
		e.scrollOffset = 0
	}
}

func (e *Editor) layoutText(contentWidth int) []layoutLine {
	out := make([]layoutLine, 0, len(e.lines))
	if e.isEmpty() {
		out = append(out, layoutLine{
			text:        "",
			hasCursor:   true,
			cursorPos:   0,
			selStart:    -1,
			selEnd:      -1,
			logicalLine: 0,
			startCol:    0,
		})
		return out
	}
	for i, line := range e.lines {
		isCur := i == e.cursorLine
		if ansitext.VisibleWidth(line) <= contentWidth {
			ll := layoutLine{
				text:        line,
				hasCursor:   isCur,
				selStart:    -1,
				selEnd:      -1,
				logicalLine: i,
				startCol:    0,
			}
			if isCur {
				ll.cursorPos = e.cursorCol
			}
			out = append(out, ll)
			continue
		}
		chunks := e.wrapLine(line, contentWidth)
		for ci, chunk := range chunks {
			isLast := ci == len(chunks)-1
			ll := layoutLine{
				text:        chunk.text,
				hasCursor:   false,
				selStart:    -1,
				selEnd:      -1,
				logicalLine: i,
				startCol:    chunk.start,
			}
			if isCur {
				cursorPos := e.cursorCol
				chunkStart := chunk.start
				if ci == 0 {
					chunkStart = 0
				}
				inChunk := false
				if isLast {
					inChunk = cursorPos >= chunkStart
				} else {
					inChunk = cursorPos >= chunkStart && cursorPos < chunk.end
				}
				if inChunk {
					adj := cursorPos - chunk.start
					if adj < 0 {
						adj = 0
					}
					if adj > len(chunk.text) {
						adj = len(chunk.text)
					}
					ll.hasCursor = true
					ll.cursorPos = adj
				}
			}
			out = append(out, ll)
		}
	}
	return out
}

func (e *Editor) applySelectionToLayout(lines []layoutLine) {
	if !e.sel.Active {
		return
	}
	a, b := orderedSel(e.sel.Anchor, e.sel.Cursor)
	for i := range lines {
		ll := &lines[i]
		// selection intersection with [startCol, startCol+len(text)] on logical line
		lineStart := CursorPos{Line: ll.logicalLine, Col: ll.startCol}
		lineEndCol := ll.startCol + len(ll.text)
		// For wrapped chunks, text length may be less than span; use start+len(text)
		lineEnd := CursorPos{Line: ll.logicalLine, Col: lineEndCol}

		// no overlap if selection entirely before or after
		if b.Line < ll.logicalLine || a.Line > ll.logicalLine {
			continue
		}
		if a.Line == ll.logicalLine && a.Col >= lineEnd.Col {
			continue
		}
		if b.Line == ll.logicalLine && b.Col <= lineStart.Col {
			continue
		}

		selStart := 0
		selEnd := len(ll.text)
		if a.Line == ll.logicalLine && a.Col > ll.startCol {
			selStart = a.Col - ll.startCol
			if selStart > len(ll.text) {
				selStart = len(ll.text)
			}
		}
		if b.Line == ll.logicalLine && b.Col < lineEndCol {
			selEnd = b.Col - ll.startCol
			if selEnd < 0 {
				selEnd = 0
			}
			if selEnd > len(ll.text) {
				selEnd = len(ll.text)
			}
		}
		if selStart < selEnd {
			ll.selStart = selStart
			ll.selEnd = selEnd
		}
	}
}

func (e *Editor) decorate(text string) string {
	fn := e.decorateText
	if fn == nil || text == "" {
		return text
	}
	marker := component.CursorMarker
	idx := strings.Index(text, marker)
	if idx < 0 {
		return fn(text)
	}
	before := text[:idx]
	after := text[idx+len(marker):]
	out := ""
	if before != "" {
		out += fn(before)
	}
	out += marker
	if after != "" {
		out += fn(after)
	}
	return out
}

type widthLimited struct {
	text  string
	width int
}

func (e *Editor) renderEOLCursorAtLimit(before, marker string, maxWidth int, override string, overrideW int) widthLimited {
	last, _ := graphemeBefore(before, len(before))
	lastW := 0
	if last != "" {
		lastW = ansitext.VisibleWidth(last)
	}
	builtIn := e.cursorGlyph
	builtInW := ansitext.VisibleWidth(builtIn)
	if builtInW <= 0 {
		builtInW = 1
	}

	repl := override
	if repl == "" {
		if last != "" {
			repl = "\x1b[7m" + last + "\x1b[0m"
		} else {
			repl = builtIn
		}
	}

	// clamp replacement
	clamped := ansitext.SliceByColumn(repl, 0, maxWidth)
	cw := ansitext.VisibleWidth(clamped)
	if cw > maxWidth {
		clamped = ""
		cw = 0
	}
	if override != "" && cw == 0 && last != "" {
		clamped = "\x1b[7m" + last + "\x1b[0m"
		cw = lastW
		if cw > maxWidth {
			clamped = ansitext.SliceByColumn(clamped, 0, maxWidth)
			cw = ansitext.VisibleWidth(clamped)
		}
	}
	if cw == 0 {
		clamped = ansitext.SliceByColumn(builtIn, 0, maxWidth)
		cw = ansitext.VisibleWidth(clamped)
	}

	span := min(maxWidth, max(lastW, cw))
	prefixW := max(0, maxWidth-span)
	beforePrefix := ansitext.SliceByColumn(before, 0, prefixW)
	pad := strings.Repeat(" ", max(0, span-cw))
	return widthLimited{
		text:  beforePrefix + pad + clamped + marker,
		width: ansitext.VisibleWidth(beforePrefix) + span,
	}
}

func (e *Editor) renderTerminalCursorMarker(text, marker string, maxWidth int) string {
	if marker == "" {
		return text
	}
	if ansitext.VisibleWidth(text) < maxWidth {
		return text + marker
	}
	// insert before last non-zero-width grapheme
	insertAt := len(text)
	off := 0
	rest := text
	for rest != "" {
		g, n := graphemeAfter(rest, 0)
		if n == 0 {
			break
		}
		if ansitext.VisibleWidth(g) > 0 {
			insertAt = off
		}
		off += n
		rest = rest[n:]
	}
	return text[:insertAt] + marker + text[insertAt:]
}

func paintSelectionWithMarker(plain string, selStart, selEnd, cursorPos int, marker string) string {
	if selStart < 0 || selEnd <= selStart {
		if cursorPos <= len(plain) {
			return plain[:cursorPos] + marker + plain[cursorPos:]
		}
		return plain + marker
	}
	// Build: plain with reverse on [selStart,selEnd) and marker at cursorPos
	var b strings.Builder
	i := 0
	for i <= len(plain) {
		if i == cursorPos {
			b.WriteString(marker)
		}
		if i == len(plain) {
			break
		}
		if i == selStart {
			b.WriteString("\x1b[7m")
		}
		b.WriteByte(plain[i])
		i++
		if i == selEnd {
			b.WriteString("\x1b[0m")
		}
	}
	if cursorPos > len(plain) {
		b.WriteString(marker)
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
