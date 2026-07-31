package renderer

import (
	"strconv"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// CursorPos is a hardware-cursor target in frame coordinates (row absolute in
// the composed frame, col visible cells from column 0).
type CursorPos struct {
	Row int
	Col int
}

// hardwareCursorState is the last known painted hardware cursor.
type hardwareCursorState struct {
	row     int
	col     int
	visible bool
	known   bool // visibility known
	full    bool // full state (row+col+vis) known, not just row
}

// ExtractCursorMarkers strips every ansitext.CursorMarker from lines (in place
// on the slice elements) and returns marker positions bottom-most first.
// Markers never reach the terminal, the committed prefix, or the audit.
//
// A nil or empty lines is a no-op. Malformed / missing markers are fine.
func ExtractCursorMarkers(lines []string) []CursorPos {
	if len(lines) == 0 {
		return nil
	}
	const marker = ansitext.CursorMarker
	var markers []CursorPos
	for row := len(lines) - 1; row >= 0; row-- {
		line := lines[row]
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		before := line[:idx]
		markers = append(markers, CursorPos{Row: row, Col: ansitext.VisibleWidth(before)})
		// Strip all occurrences.
		var b strings.Builder
		b.Grow(len(line))
		rest := line
		for {
			i := strings.Index(rest, marker)
			if i < 0 {
				b.WriteString(rest)
				break
			}
			b.WriteString(rest[:i])
			rest = rest[i+len(marker):]
		}
		lines[row] = b.String()
	}
	return markers
}

// pickVisibleCursor chooses the bottom-most marker at or below windowTop.
// overlay markers (if any) win when provided as absolute frame coords.
func pickVisibleCursor(markers []CursorPos, windowTop int) *CursorPos {
	for i := 0; i < len(markers); i++ {
		// markers are bottom-most first from ExtractCursorMarkers
		if markers[i].Row >= windowTop {
			c := markers[i]
			return &c
		}
	}
	return nil
}

// cursorControl builds relative move + column + show/hide sequences from fromRow
// to the target. When target is nil the cursor is hidden.
func (e *Engine) cursorControl(cursor *CursorPos, totalLines, fromRow int) (seq string, toRow, toCol int, visible bool, full *hardwareCursorState) {
	target := e.targetHardwareCursor(cursor, totalLines)
	if target == nil {
		return hideCursor, fromRow, 0, false, nil
	}
	var b strings.Builder
	rowDelta := target.row - fromRow
	if rowDelta > 0 {
		b.WriteString("\x1b[")
		b.WriteString(strconv.Itoa(rowDelta))
		b.WriteByte('B')
	} else if rowDelta < 0 {
		b.WriteString("\x1b[")
		b.WriteString(strconv.Itoa(-rowDelta))
		b.WriteByte('A')
	}
	// Absolute column (1-indexed).
	b.WriteString("\x1b[")
	b.WriteString(strconv.Itoa(target.col + 1))
	b.WriteByte('G')
	if target.visible {
		b.WriteString(showCursor)
	} else {
		b.WriteString(hideCursor)
	}
	st := *target
	return b.String(), target.row, target.col, target.visible, &st
}

func (e *Engine) targetHardwareCursor(cursor *CursorPos, totalLines int) *hardwareCursorState {
	if cursor == nil || totalLines <= 0 {
		return nil
	}
	row := cursor.Row
	if row < 0 {
		row = 0
	}
	if row > totalLines-1 {
		row = totalLines - 1
	}
	col := cursor.Col
	if col < 0 {
		col = 0
	}
	return &hardwareCursorState{
		row:     row,
		col:     col,
		visible: e.caps.ShowHardwareCursor,
		known:   true,
		full:    true,
	}
}

func (e *Engine) recordHardwareCursorUpdate(toRow, toCol int, visible bool, full *hardwareCursorState) {
	if full != nil {
		e.hwCursor = *full
		e.hwCursor.known = true
		e.hwCursor.full = true
		return
	}
	e.hwCursor.row = toRow
	e.hwCursor.col = toCol
	e.hwCursor.visible = visible
	e.hwCursor.known = true
	e.hwCursor.full = false
}

func (e *Engine) recordHardwareCursorHidden() {
	e.hwCursor.visible = false
	e.hwCursor.known = true
	if e.hwCursor.full {
		e.hwCursor.visible = false
	}
}

func (e *Engine) forgetHardwareCursorState() {
	e.hwCursor.full = false
	e.hwCursor.known = false
}

func (e *Engine) sameHardwareCursor(st *hardwareCursorState) bool {
	if st == nil || !e.hwCursor.full {
		return false
	}
	return e.hwCursor.row == st.row && e.hwCursor.col == st.col && e.hwCursor.visible == st.visible
}

func (e *Engine) isHiddenCursorKnown() bool {
	return e.hwCursor.known && !e.hwCursor.visible
}

// writeCursorOnly emits a standalone synchronized cursor move when the frame
// content is unchanged.
func (e *Engine) writeCursorOnly(cursor *CursorPos, totalLines int) error {
	target := e.targetHardwareCursor(cursor, totalLines)
	if target == nil {
		if e.isHiddenCursorKnown() {
			return nil
		}
		_, err := ioWriteString(e.out, hideCursor)
		if err != nil {
			return err
		}
		e.recordHardwareCursorHidden()
		return nil
	}
	if e.sameHardwareCursor(target) {
		return nil
	}
	seq, toRow, toCol, visible, full := e.cursorControl(cursor, totalLines, e.hwCursor.row)
	buf := e.caps.cursorBegin() + seq + e.caps.cursorEnd()
	if _, err := ioWriteString(e.out, buf); err != nil {
		return err
	}
	e.recordHardwareCursorUpdate(toRow, toCol, visible, full)
	return nil
}
