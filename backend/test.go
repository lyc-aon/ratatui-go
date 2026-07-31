package backend

import (
	"fmt"
	"strings"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/text"
)

// maxScrollbackLines caps retained scrollback height (matches Ratatui u16::MAX).
const maxScrollbackLines = 65535

// TestBackend is a deterministic in-memory Backend for tests and demos.
//
// It never touches a real terminal. Draw writes into an internal buffer that
// can be inspected with Buffer / BufferLines. Lines scrolled off the top are
// retained in Scrollback.
type TestBackend struct {
	buf           *buffer.Buffer
	scrollback    *buffer.Buffer
	cursorVisible bool
	cursor        layout.Position
}

// NewTestBackend creates a TestBackend with the given cell dimensions.
// Width or height less than zero is clamped to zero.
func NewTestBackend(width, height int) *TestBackend {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &TestBackend{
		buf:           buffer.Empty(layout.Rect{X: 0, Y: 0, Width: width, Height: height}),
		scrollback:    buffer.Empty(layout.Rect{X: 0, Y: 0, Width: width, Height: 0}),
		cursorVisible: false,
		cursor:        layout.Position{},
	}
}

// WithLines creates a TestBackend whose initial screen content is lines.
// Size is derived from the widest line and the number of lines.
func WithLines(lines ...text.Line) *TestBackend {
	buf := buffer.WithLines(lines...)
	width := buf.Area.Width
	return &TestBackend{
		buf:           buf,
		scrollback:    buffer.Empty(layout.Rect{X: 0, Y: 0, Width: width, Height: 0}),
		cursorVisible: false,
		cursor:        layout.Position{},
	}
}

// WithStrings is the string convenience form of WithLines.
func WithStrings(lines ...string) *TestBackend {
	converted := make([]text.Line, len(lines))
	for i := range lines {
		converted[i] = text.RawLine(lines[i])
	}
	return WithLines(converted...)
}

// Buffer returns the internal cell buffer.
func (t *TestBackend) Buffer() *buffer.Buffer {
	return t.buf
}

// Scrollback returns the retained scrollback buffer (rows scrolled off the top).
func (t *TestBackend) Scrollback() *buffer.Buffer {
	return t.scrollback
}

// CursorVisible reports whether the cursor is shown.
func (t *TestBackend) CursorVisible() bool {
	return t.cursorVisible
}

// CursorPosition returns the tracked cursor position without error wrapping.
func (t *TestBackend) CursorPosition() layout.Position {
	return t.cursor
}

// Resize changes the backend dimensions. Content is resized via buffer.Resize.
// Scrollback width is updated to match; its height is preserved.
func (t *TestBackend) Resize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	t.buf.Resize(layout.Rect{X: 0, Y: 0, Width: width, Height: height})
	sbH := 0
	if t.scrollback != nil {
		sbH = t.scrollback.Area.Height
	}
	t.scrollback.Resize(layout.Rect{X: 0, Y: 0, Width: width, Height: sbH})
}

// BufferLines returns the symbol grid as strings, one per row.
// Useful for deterministic assertions in demos and tests.
func (t *TestBackend) BufferLines() []string {
	return bufferLines(t.buf)
}

// ScrollbackLines returns the scrollback symbol grid as strings, one per row.
func (t *TestBackend) ScrollbackLines() []string {
	return bufferLines(t.scrollback)
}

func bufferLines(buf *buffer.Buffer) []string {
	if buf == nil {
		return nil
	}
	area := buf.Area
	if area.Width <= 0 || area.Height <= 0 {
		return nil
	}
	lines := make([]string, area.Height)
	for y := range area.Height {
		var b strings.Builder
		b.Grow(area.Width)
		for x := range area.Width {
			cell, ok := buf.Get(area.X+x, area.Y+y)
			if !ok {
				b.WriteByte(' ')
				continue
			}
			sym := cell.Symbol
			if sym == "" {
				sym = " "
			}
			b.WriteString(sym)
		}
		lines[y] = b.String()
	}
	return lines
}

// String returns a quoted multi-line view of the buffer symbols.
func (t *TestBackend) String() string {
	lines := t.BufferLines()
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteByte('"')
		b.WriteString(line)
		b.WriteString("\"\n")
	}
	return b.String()
}

// Draw implements Backend.
func (t *TestBackend) Draw(cells []buffer.PositionedCell) error {
	if t.buf == nil {
		return fmt.Errorf("backend: test backend has nil buffer")
	}
	for i := range cells {
		pc := &cells[i]
		if c := t.buf.GetMut(pc.Position.X, pc.Position.Y); c != nil {
			*c = pc.Cell
		}
	}
	return nil
}

// HideCursor implements Backend.
func (t *TestBackend) HideCursor() error {
	t.cursorVisible = false
	return nil
}

// ShowCursor implements Backend.
func (t *TestBackend) ShowCursor() error {
	t.cursorVisible = true
	return nil
}

// GetCursorPosition implements Backend.
func (t *TestBackend) GetCursorPosition() (layout.Position, error) {
	return t.cursor, nil
}

// SetCursorPosition implements Backend.
func (t *TestBackend) SetCursorPosition(pos layout.Position) error {
	t.cursor = pos
	return nil
}

// Clear implements Backend.
func (t *TestBackend) Clear() error {
	return t.ClearRegion(All)
}

// ClearRegion implements Backend.
// Clears the requested region without moving the cursor.
func (t *TestBackend) ClearRegion(clearType ClearType) error {
	if t.buf == nil || t.buf.Area.IsEmpty() {
		if clearType == All && t.buf != nil {
			t.buf.Reset()
		}
		return nil
	}
	area := t.buf.Area
	width := area.Width
	height := area.Height
	if width <= 0 || height <= 0 {
		return nil
	}

	cx := t.cursor.X
	cy := t.cursor.Y
	if cx < area.X {
		cx = area.X
	}
	if cy < area.Y {
		cy = area.Y
	}
	// Relative indices into content.
	relX := cx - area.X
	relY := cy - area.Y
	if relX < 0 {
		relX = 0
	}
	if relY < 0 {
		relY = 0
	}
	if relX >= width {
		relX = width - 1
	}
	if relY >= height {
		relY = height - 1
	}

	resetRange := func(from, to int) {
		if from < 0 {
			from = 0
		}
		if to > len(t.buf.Content) {
			to = len(t.buf.Content)
		}
		if from >= to {
			return
		}
		empty := buffer.NewCell()
		for i := from; i < to; i++ {
			t.buf.Content[i] = empty
		}
	}

	switch clearType {
	case All:
		t.buf.Reset()
	case AfterCursor:
		idx := relY*width + relX
		resetRange(idx, len(t.buf.Content))
	case BeforeCursor:
		idx := relY*width + relX
		resetRange(0, idx+1)
	case CurrentLine:
		start := relY * width
		resetRange(start, start+width)
	case UntilNewLine:
		idx := relY*width + relX
		end := relY*width + width
		resetRange(idx, end)
	default:
		return fmt.Errorf("backend: unsupported clear type %d", clearType)
	}
	return nil
}

// AppendLines implements Backend.
//
// Inserts n blank lines of scrolling at the cursor. When n exceeds the rows
// below the cursor, content scrolls up into scrollback and the cursor moves to
// the bottom row. Cursor x advances by one column (raw-mode style), clamped to
// the last column. n <= 0 is a no-op. Zero-size buffers are safe.
func (t *TestBackend) AppendLines(n int) error {
	if n <= 0 || t.buf == nil {
		return nil
	}
	area := t.buf.Area
	if area.IsEmpty() {
		return nil
	}

	width := area.Width
	height := area.Height
	curX := t.cursor.X
	curY := t.cursor.Y

	// Next column, not past last column.
	newCursorX := curX + 1
	if width > 0 && newCursorX > width-1 {
		newCursorX = width - 1
	}
	if newCursorX < 0 {
		newCursorX = 0
	}

	maxY := height - 1
	if maxY < 0 {
		maxY = 0
	}
	linesAfterCursor := maxY - curY
	if linesAfterCursor < 0 {
		linesAfterCursor = 0
	}

	if n > linesAfterCursor {
		scrollBy := n - linesAfterCursor
		if width > 0 && height > 0 {
			cellsToScroll := width * scrollBy
			contentLen := len(t.buf.Content)
			if cellsToScroll > contentLen {
				cellsToScroll = contentLen
			}
			// Move top rows into scrollback, then rotate remaining content up.
			if cellsToScroll > 0 {
				appendToScrollback(t.scrollback, t.buf.Content[:cellsToScroll])
				empty := buffer.NewCell()
				copy(t.buf.Content, t.buf.Content[cellsToScroll:])
				for i := contentLen - cellsToScroll; i < contentLen; i++ {
					t.buf.Content[i] = empty
				}
			}
			// If scroll requested more blank rows than available content, pad scrollback.
			missing := width*scrollBy - cellsToScroll
			if missing > 0 {
				blanks := make([]buffer.Cell, missing)
				empty := buffer.NewCell()
				for i := range blanks {
					blanks[i] = empty
				}
				appendToScrollback(t.scrollback, blanks)
			}
		}
	}

	newCursorY := curY + n
	if newCursorY > maxY {
		newCursorY = maxY
	}
	if newCursorY < 0 {
		newCursorY = 0
	}
	t.cursor = layout.Position{X: newCursorX, Y: newCursorY}
	return nil
}

// Size implements Backend.
func (t *TestBackend) Size() (layout.Size, error) {
	return layout.Size{Width: t.buf.Area.Width, Height: t.buf.Area.Height}, nil
}

// WindowSize implements Backend.
// Pixels are a fixed test value matching Ratatui's TestBackend (640x480).
func (t *TestBackend) WindowSize() (WindowSize, error) {
	return WindowSize{
		ColumnsRows: layout.Size{Width: t.buf.Area.Width, Height: t.buf.Area.Height},
		Pixels:      layout.Size{Width: 640, Height: 480},
	}, nil
}

// Flush implements Backend.
func (t *TestBackend) Flush() error {
	return nil
}

// ScrollRegionUp implements ScrollingRegionBackend.
//
// Scrolls the half-open row range [start, end) up by amount rows. When the
// region includes row 0, scrolled-off rows are appended to scrollback.
func (t *TestBackend) ScrollRegionUp(start, end, amount int) error {
	if t.buf == nil || amount <= 0 {
		return nil
	}
	area := t.buf.Area
	width := area.Width
	height := area.Height
	if width <= 0 || height <= 0 {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > height {
		end = height
	}
	if start >= end {
		return nil
	}

	cellStart := width * start
	cellEnd := width * end
	regionLen := cellEnd - cellStart
	scrollCells := width * amount
	empty := buffer.NewCell()

	// Region does not include top of screen: no scrollback.
	if cellStart > 0 {
		if scrollCells >= regionLen {
			for i := cellStart; i < cellEnd; i++ {
				t.buf.Content[i] = empty
			}
			return nil
		}
		// rotate left within region, clear bottom
		region := t.buf.Content[cellStart:cellEnd]
		copy(region, region[scrollCells:])
		for i := regionLen - scrollCells; i < regionLen; i++ {
			region[i] = empty
		}
		return nil
	}

	// Includes row 0: push into scrollback.
	fromRegion := regionLen
	if fromRegion > scrollCells {
		fromRegion = scrollCells
	}
	if fromRegion > 0 {
		// Copy out rows that leave the screen before mutating.
		leaving := make([]buffer.Cell, fromRegion)
		copy(leaving, t.buf.Content[cellStart:cellStart+fromRegion])
		appendToScrollback(t.scrollback, leaving)
		// Clear those cells then rotate remaining content up.
		for i := cellStart; i < cellStart+fromRegion; i++ {
			t.buf.Content[i] = empty
		}
	}
	if scrollCells < regionLen {
		region := t.buf.Content[cellStart:cellEnd]
		// after clearing top fromRegion cells, rotate left by fromRegion
		copy(region, region[fromRegion:])
		for i := regionLen - fromRegion; i < regionLen; i++ {
			region[i] = empty
		}
	} else {
		// Whole region cleared; pad scrollback with blanks for remainder.
		missing := scrollCells - regionLen
		if missing > 0 {
			blanks := make([]buffer.Cell, missing)
			for i := range blanks {
				blanks[i] = empty
			}
			appendToScrollback(t.scrollback, blanks)
		}
		for i := cellStart; i < cellEnd; i++ {
			t.buf.Content[i] = empty
		}
	}
	return nil
}

// ScrollRegionDown implements ScrollingRegionBackend.
//
// Scrolls the half-open row range [start, end) down by amount rows. Does not
// touch scrollback (matches ANSI terminal asymmetry).
func (t *TestBackend) ScrollRegionDown(start, end, amount int) error {
	if t.buf == nil || amount <= 0 {
		return nil
	}
	area := t.buf.Area
	width := area.Width
	height := area.Height
	if width <= 0 || height <= 0 {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > height {
		end = height
	}
	if start >= end {
		return nil
	}

	cellStart := width * start
	cellEnd := width * end
	regionLen := cellEnd - cellStart
	scrollCells := width * amount
	empty := buffer.NewCell()

	if scrollCells >= regionLen {
		for i := cellStart; i < cellEnd; i++ {
			t.buf.Content[i] = empty
		}
		return nil
	}
	region := t.buf.Content[cellStart:cellEnd]
	// rotate right
	tmp := make([]buffer.Cell, scrollCells)
	copy(tmp, region[regionLen-scrollCells:])
	copy(region[scrollCells:], region[:regionLen-scrollCells])
	copy(region[:scrollCells], tmp)
	for i := 0; i < scrollCells; i++ {
		region[i] = empty
	}
	return nil
}

// appendToScrollback appends cells (multiple of width) to the bottom of
// scrollback, trimming from the top when taller than maxScrollbackLines.
func appendToScrollback(scrollback *buffer.Buffer, cells []buffer.Cell) {
	if scrollback == nil || len(cells) == 0 {
		return
	}
	width := scrollback.Area.Width
	if width <= 0 {
		// Infer width from existing content or refuse.
		if scrollback.Area.Height > 0 && len(scrollback.Content) > 0 {
			width = len(scrollback.Content) / scrollback.Area.Height
		}
		if width <= 0 {
			return
		}
		scrollback.Area.Width = width
	}
	scrollback.Content = append(scrollback.Content, cells...)
	newHeight := len(scrollback.Content) / width
	if newHeight > maxScrollbackLines {
		keepFrom := len(scrollback.Content) - width*maxScrollbackLines
		if keepFrom > 0 {
			scrollback.Content = append([]buffer.Cell(nil), scrollback.Content[keepFrom:]...)
		}
		newHeight = maxScrollbackLines
	}
	scrollback.Area.Height = newHeight
	// Ensure Content length matches area (drop partial trailing row if any).
	want := width * newHeight
	if len(scrollback.Content) > want {
		scrollback.Content = scrollback.Content[:want]
	}
}
