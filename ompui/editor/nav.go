package editor

import (
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/textutil"
)

func (e *Editor) wrapLine(line string, width int) []textChunk {
	if width != e.wrapCacheWidth {
		e.wrapCache = make(map[string][]textChunk)
		e.wrapCacheWidth = width
	}
	if chunks, ok := e.wrapCache[line]; ok {
		return chunks
	}
	if len(e.wrapCache) >= wrapCacheLimit {
		e.wrapCache = make(map[string][]textChunk)
	}
	chunks := wordWrapLine(line, width)
	e.wrapCache[line] = chunks
	return chunks
}

func (e *Editor) buildVisualLineMap(width int) []visualLine {
	out := make([]visualLine, 0, len(e.lines))
	for i, line := range e.lines {
		if len(line) == 0 {
			out = append(out, visualLine{logicalLine: i, startCol: 0, length: 0})
			continue
		}
		chunks := e.wrapLine(line, width)
		for _, c := range chunks {
			out = append(out, visualLine{
				logicalLine: i,
				startCol:    c.start,
				length:      c.end - c.start,
			})
		}
	}
	if len(out) == 0 {
		out = append(out, visualLine{})
	}
	return out
}

func (e *Editor) findCurrentVisualLine(visual []visualLine) int {
	for i, vl := range visual {
		if vl.logicalLine != e.cursorLine {
			continue
		}
		colIn := e.cursorCol - vl.startCol
		isLast := i == len(visual)-1 || visual[i+1].logicalLine != vl.logicalLine
		isFirst := i == 0 || visual[i-1].logicalLine != vl.logicalLine
		if (colIn >= 0 || isFirst) && (colIn < vl.length || (isLast && colIn <= vl.length)) {
			return i
		}
	}
	if len(visual) == 0 {
		return 0
	}
	return len(visual) - 1
}

func (e *Editor) isOnFirstVisualLine() bool {
	visual := e.buildVisualLineMap(e.lastLayoutWidth)
	return e.findCurrentVisualLine(visual) == 0
}

func (e *Editor) isOnLastVisualLine() bool {
	visual := e.buildVisualLineMap(e.lastLayoutWidth)
	return e.findCurrentVisualLine(visual) == len(visual)-1
}

func (e *Editor) moveCursor(deltaLine, deltaCol int) {
	e.resetKillSequence()
	e.clearSelection()
	visual := e.buildVisualLineMap(e.lastLayoutWidth)
	curVL := e.findCurrentVisualLine(visual)

	if deltaLine != 0 {
		target := curVL + deltaLine
		if target >= 0 && target < len(visual) {
			e.moveToVisualLine(visual, curVL, target)
		}
	}

	if deltaCol != 0 {
		cur := e.currentLine()
		if deltaCol > 0 {
			if e.cursorCol < len(cur) {
				_, n := graphemeAfter(cur, e.cursorCol)
				if n == 0 {
					n = 1
				}
				e.setCursorCol(e.cursorCol + n)
			} else if e.cursorLine < len(e.lines)-1 {
				e.cursorLine++
				e.setCursorCol(0)
			} else if curVL >= 0 && curVL < len(visual) {
				vl := visual[curVL]
				end := vl.startCol + vl.length
				if end > len(cur) {
					end = len(cur)
				}
				seg := cur[vl.startCol:end]
				e.preferredVisualCol = visualColAtOffset(seg, e.cursorCol-vl.startCol)
			}
		} else {
			if e.cursorCol > 0 {
				_, n := graphemeBefore(cur, e.cursorCol)
				if n == 0 {
					n = 1
				}
				e.setCursorCol(e.cursorCol - n)
			} else if e.cursorLine > 0 {
				e.cursorLine--
				e.setCursorCol(len(e.currentLine()))
			}
		}
	}
}

func (e *Editor) moveToVisualLine(visual []visualLine, currentIdx, targetIdx int) {
	if currentIdx < 0 || currentIdx >= len(visual) || targetIdx < 0 || targetIdx >= len(visual) {
		return
	}
	cur := visual[currentIdx]
	tgt := visual[targetIdx]
	srcLine := e.lines[cur.logicalLine]
	srcEnd := cur.startCol + cur.length
	if srcEnd > len(srcLine) {
		srcEnd = len(srcLine)
	}
	srcText := srcLine[cur.startCol:srcEnd]
	currentVisualCol := visualColAtOffset(srcText, e.cursorCol-cur.startCol)

	isLastSrc := currentIdx == len(visual)-1 || visual[currentIdx+1].logicalLine != cur.logicalLine
	srcMax := maxSegmentVisualCol(srcText, isLastSrc)

	isLastTgt := targetIdx == len(visual)-1 || visual[targetIdx+1].logicalLine != tgt.logicalLine
	tgtLine := e.lines[tgt.logicalLine]
	tgtEnd := tgt.startCol + tgt.length
	if tgtEnd > len(tgtLine) {
		tgtEnd = len(tgtLine)
	}
	tgtText := tgtLine[tgt.startCol:tgtEnd]
	tgtMax := maxSegmentVisualCol(tgtText, isLastTgt)

	moveCol := e.computeVerticalMoveColumn(currentVisualCol, srcMax, tgtMax)
	e.cursorLine = tgt.logicalLine
	targetCol := tgt.startCol + offsetAtVisualCol(tgtText, moveCol)
	if targetCol > len(tgtLine) {
		targetCol = len(tgtLine)
	}
	// Vertical move manages preferredVisualCol; do not clear via setCursorCol.
	e.cursorCol = clampByteOffset(tgtLine, targetCol)
}

func (e *Editor) computeVerticalMoveColumn(current, srcMax, tgtMax int) int {
	hasPreferred := e.preferredVisualCol >= 0
	cursorInMiddle := current < srcMax
	targetTooShort := tgtMax < current

	if !hasPreferred || cursorInMiddle {
		if targetTooShort {
			e.preferredVisualCol = current
			return tgtMax
		}
		e.preferredVisualCol = -1
		return current
	}
	if targetTooShort || tgtMax < e.preferredVisualCol {
		return tgtMax
	}
	result := e.preferredVisualCol
	e.preferredVisualCol = -1
	return result
}

func (e *Editor) pageScroll(direction int) {
	e.resetKillSequence()
	e.clearSelection()
	visual := e.buildVisualLineMap(e.lastLayoutWidth)
	cur := e.findCurrentVisualLine(visual)
	step := e.pageScrollStep(len(visual))
	target := cur + direction*step
	if target < 0 {
		target = 0
	}
	if target >= len(visual) {
		target = len(visual) - 1
	}
	if target == cur {
		return
	}
	e.moveToVisualLine(visual, cur, target)
}

func (e *Editor) pageScrollStep(totalVisual int) int {
	if e.maxHeight == 0 {
		return defaultPageScroll
	}
	visible := e.visibleContentHeight(totalVisual)
	if visible-1 < 1 {
		return 1
	}
	return visible - 1
}

func (e *Editor) visibleContentHeight(contentLines int) int {
	if e.maxHeight == 0 {
		return contentLines
	}
	chrome := 0
	if e.borderVisible {
		chrome = 2
	}
	h := e.maxHeight - chrome
	if h < 1 {
		return 1
	}
	return h
}

func (e *Editor) moveWordBackwards() {
	e.clearSelection()
	cur := e.currentLine()
	if e.cursorCol == 0 {
		if e.cursorLine > 0 {
			e.cursorLine--
			e.setCursorCol(len(e.currentLine()))
		}
		return
	}
	e.setCursorCol(textutil.MoveWordLeft(cur, e.cursorCol))
}

func (e *Editor) moveWordForwards() {
	e.clearSelection()
	cur := e.currentLine()
	if e.cursorCol >= len(cur) {
		if e.cursorLine < len(e.lines)-1 {
			e.cursorLine++
			e.setCursorCol(0)
		}
		return
	}
	e.setCursorCol(textutil.MoveWordRight(cur, e.cursorCol))
}

func (e *Editor) moveToLineStart() {
	e.resetKillSequence()
	e.clearSelection()
	e.setCursorCol(0)
}

func (e *Editor) moveToLineEnd() {
	e.resetKillSequence()
	e.clearSelection()
	e.setCursorCol(len(e.currentLine()))
}

func (e *Editor) moveToMessageStart() {
	e.resetKillSequence()
	e.clearSelection()
	e.cursorLine = 0
	e.setCursorCol(0)
}

func (e *Editor) moveToMessageEnd() {
	e.resetKillSequence()
	e.clearSelection()
	e.cursorLine = len(e.lines) - 1
	e.setCursorCol(len(e.currentLine()))
}

func (e *Editor) jumpToChar(char, direction string) {
	e.resetKillSequence()
	e.clearSelection()
	if char == "" {
		return
	}
	if direction == "forward" {
		for lineIdx := e.cursorLine; lineIdx < len(e.lines); lineIdx++ {
			line := e.lines[lineIdx]
			from := 0
			if lineIdx == e.cursorLine {
				from = e.cursorCol + 1
				if from > len(line) {
					continue
				}
			}
			rel := strings.Index(line[from:], char)
			if rel >= 0 {
				e.cursorLine = lineIdx
				e.setCursorCol(from + rel)
				return
			}
		}
		return
	}
	for lineIdx := e.cursorLine; lineIdx >= 0; lineIdx-- {
		line := e.lines[lineIdx]
		limit := len(line)
		if lineIdx == e.cursorLine {
			limit = e.cursorCol
			if limit <= 0 {
				continue
			}
		}
		idx := strings.LastIndex(line[:limit], char)
		if idx >= 0 {
			e.cursorLine = lineIdx
			e.setCursorCol(idx)
			return
		}
	}
}

func (e *Editor) navigateHistory(direction int) {
	// direction: -1 = older (Up), +1 = newer (Down)
	e.resetKillSequence()
	e.clearSelection()
	if len(e.history) == 0 {
		return
	}
	// Capture scratch when first entering history browse.
	if e.historyIndex == -1 && direction < 0 {
		e.historyScratch = strings.Join(e.lines, "\n")
	}
	newIndex := e.historyIndex - direction // Up(-1) increases index
	if newIndex < -1 || newIndex >= len(e.history) {
		return
	}
	e.historyIndex = newIndex
	if e.historyIndex == -1 {
		e.setTextInternal(e.historyScratch, HistoryAnchorEnd)
		return
	}
	anchor := HistoryAnchorEnd
	if direction < 0 {
		anchor = HistoryAnchorStart
	}
	e.setTextInternal(e.history[e.historyIndex], anchor)
}
