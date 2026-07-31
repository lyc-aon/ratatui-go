package backend

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
)

// ANSIBackend writes VT/ANSI escape sequences to an io.Writer.
//
// Dimensions are owned by the backend (not process-global state). Setup and
// Restore emit alternate-screen / cursor sequences to the writer only; they do
// not enable raw mode or mutate file-descriptor terminal attributes.
type ANSIBackend struct {
	w io.Writer

	mu     sync.Mutex
	width  int
	height int

	cursor        layout.Position
	cursorVisible bool

	// scratch for Draw batch assembly
	buf bytes.Buffer
}

// NewANSIBackend creates an ANSI backend writing to w with the given size.
// Width or height less than zero is clamped to zero. A nil writer becomes
// io.Discard.
func NewANSIBackend(w io.Writer, width, height int) *ANSIBackend {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	if w == nil {
		w = io.Discard
	}
	return &ANSIBackend{
		w:             w,
		width:         width,
		height:        height,
		cursorVisible: true,
	}
}

// Writer returns the underlying writer.
func (a *ANSIBackend) Writer() io.Writer {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.w
}

// SetWriter replaces the underlying writer. Nil becomes io.Discard.
func (a *ANSIBackend) SetWriter(w io.Writer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if w == nil {
		w = io.Discard
	}
	a.w = w
}

// Resize updates the reported terminal dimensions.
func (a *ANSIBackend) Resize(width, height int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	a.width = width
	a.height = height
}

// Setup writes sequences to enter the alternate screen, clear it, move the
// cursor home, and hide the cursor. It does not take process-global ownership
// of the terminal device (no raw mode, no signal hooks).
func (a *ANSIBackend) Setup() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := io.WriteString(a.w, seqEnterAltScreen+seqClearAll+seqCursorHome+seqHideCursor)
	if err != nil {
		return err
	}
	a.cursorVisible = false
	a.cursor = layout.Position{}
	return nil
}

// Restore writes sequences to show the cursor and leave the alternate screen.
// Pair with Setup. Safe to call multiple times.
func (a *ANSIBackend) Restore() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := io.WriteString(a.w, seqShowCursor+seqLeaveAltScreen)
	if err != nil {
		return err
	}
	a.cursorVisible = true
	return nil
}

// EnterAlternateScreen writes the alternate-screen enter sequence to w.
func EnterAlternateScreen(w io.Writer) error {
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, seqEnterAltScreen)
	return err
}

// LeaveAlternateScreen writes the alternate-screen leave sequence to w.
func LeaveAlternateScreen(w io.Writer) error {
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, seqLeaveAltScreen)
	return err
}

// WriteHideCursor writes the hide-cursor sequence to w.
func WriteHideCursor(w io.Writer) error {
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, seqHideCursor)
	return err
}

// WriteShowCursor writes the show-cursor sequence to w.
func WriteShowCursor(w io.Writer) error {
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, seqShowCursor)
	return err
}

// WriteClearAll writes a full-screen clear sequence to w.
// Does not move or reset the cursor.
func WriteClearAll(w io.Writer) error {
	return WriteClearRegion(w, All)
}

// WriteClearRegion writes the ANSI CSI clear sequence for clearType to w.
// Does not move or reset the cursor.
func WriteClearRegion(w io.Writer, clearType ClearType) error {
	if w == nil {
		return nil
	}
	seq, err := clearTypeSequence(clearType)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, seq)
	return err
}

const (
	seqEnterAltScreen  = "\x1b[?1049h"
	seqLeaveAltScreen  = "\x1b[?1049l"
	seqHideCursor      = "\x1b[?25l"
	seqShowCursor      = "\x1b[?25h"
	seqClearAll        = "\x1b[2J"
	seqClearAfter      = "\x1b[0J"
	seqClearBefore     = "\x1b[1J"
	seqClearLine       = "\x1b[2K"
	seqClearUntilNL    = "\x1b[0K"
	seqCursorHome      = "\x1b[H"
	seqSGRReset        = "\x1b[0m"
	seqScrollRegionRst = "\x1b[r"
)

func clearTypeSequence(clearType ClearType) (string, error) {
	switch clearType {
	case All:
		return seqClearAll, nil
	case AfterCursor:
		return seqClearAfter, nil
	case BeforeCursor:
		return seqClearBefore, nil
	case CurrentLine:
		return seqClearLine, nil
	case UntilNewLine:
		return seqClearUntilNL, nil
	default:
		return "", fmt.Errorf("backend: unsupported clear type %d", clearType)
	}
}

// Draw implements Backend.
func (a *ANSIBackend) Draw(cells []buffer.PositionedCell) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(cells) == 0 {
		return nil
	}

	a.buf.Reset()

	var (
		fg       style.Color
		bg       style.Color
		ul       style.Color
		haveFG   bool
		haveBG   bool
		haveUL   bool
		mod      style.Modifier
		last     layout.Position
		haveLast bool
		started  bool
	)

	for i := range cells {
		pc := &cells[i]
		x, y := pc.Position.X, pc.Position.Y
		cell := &pc.Cell

		// Move cursor when not continuing on the same row at x == last.X+1.
		if !haveLast || y != last.Y || x != last.X+1 {
			// ANSI cursor positions are 1-based.
			writeCursorMove(&a.buf, x+1, y+1)
		}
		last = layout.Position{X: x, Y: y}
		haveLast = true
		a.cursor = last

		st := cell.StyleValue()
		nextMod := st.Modifiers()
		nextFG, hasFG := st.Foreground()
		nextBG, hasBG := st.Background()
		nextUL, hasUL := st.Underline()

		styleChanged := !started ||
			nextMod != mod ||
			hasFG != haveFG || (hasFG && nextFG != fg) ||
			hasBG != haveBG || (hasBG && nextBG != bg) ||
			hasUL != haveUL || (hasUL && nextUL != ul)

		if styleChanged {
			// Reset and re-apply. Correct; only emitted when style changes.
			a.buf.WriteString(seqSGRReset)
			writeModifier(&a.buf, nextMod)
			if hasFG {
				writeFG(&a.buf, nextFG)
			}
			if hasBG {
				writeBG(&a.buf, nextBG)
			}
			if hasUL {
				writeUnderlineColor(&a.buf, nextUL)
			}
			mod = nextMod
			fg, haveFG = nextFG, hasFG
			bg, haveBG = nextBG, hasBG
			ul, haveUL = nextUL, hasUL
			started = true
		}

		sym := cell.Symbol
		if sym == "" {
			sym = " "
		}
		a.buf.WriteString(sym)
	}

	// Always leave the writer style in a clean state after a batch.
	a.buf.WriteString(seqSGRReset)

	_, err := a.w.Write(a.buf.Bytes())
	return err
}

// HideCursor implements Backend.
func (a *ANSIBackend) HideCursor() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := io.WriteString(a.w, seqHideCursor)
	if err == nil {
		a.cursorVisible = false
	}
	return err
}

// ShowCursor implements Backend.
func (a *ANSIBackend) ShowCursor() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := io.WriteString(a.w, seqShowCursor)
	if err == nil {
		a.cursorVisible = true
	}
	return err
}

// GetCursorPosition implements Backend.
//
// Pure writer backends cannot query the real cursor; this returns the last
// position set via SetCursorPosition or inferred from Draw.
func (a *ANSIBackend) GetCursorPosition() (layout.Position, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cursor, nil
}

// SetCursorPosition implements Backend.
func (a *ANSIBackend) SetCursorPosition(pos layout.Position) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var scratch bytes.Buffer
	writeCursorMove(&scratch, pos.X+1, pos.Y+1)
	_, err := a.w.Write(scratch.Bytes())
	if err == nil {
		a.cursor = pos
	}
	return err
}

// Clear implements Backend.
// Writes CSI 2 J only. Does not move or reset the cursor.
func (a *ANSIBackend) Clear() error {
	return a.ClearRegion(All)
}

// ClearRegion implements Backend.
// Emits the exact ANSI CSI erase sequence for clearType without moving the cursor.
func (a *ANSIBackend) ClearRegion(clearType ClearType) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	seq, err := clearTypeSequence(clearType)
	if err != nil {
		return err
	}
	_, err = io.WriteString(a.w, seq)
	return err
}

// AppendLines implements Backend.
//
// Writes n newline characters and advances the tracked cursor downward,
// clamping to the bottom row. Cursor x advances by one column (raw-mode style).
// n <= 0 is a no-op. Zero-height terminals are safe.
func (a *ANSIBackend) AppendLines(n int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n <= 0 {
		return nil
	}
	for range n {
		if _, err := io.WriteString(a.w, "\n"); err != nil {
			return err
		}
	}
	// Update tracked cursor: move down, clamp; advance x by 1.
	curX := a.cursor.X + 1
	if a.width > 0 && curX > a.width-1 {
		curX = a.width - 1
	}
	if curX < 0 {
		curX = 0
	}
	curY := a.cursor.Y + n
	maxY := a.height - 1
	if maxY < 0 {
		maxY = 0
	}
	if curY > maxY {
		curY = maxY
	}
	if curY < 0 {
		curY = 0
	}
	a.cursor = layout.Position{X: curX, Y: curY}
	// Flush if possible so appended lines take effect before next draw.
	if f, ok := a.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// Size implements Backend.
func (a *ANSIBackend) Size() (layout.Size, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return layout.Size{Width: a.width, Height: a.height}, nil
}

// WindowSize implements Backend.
// Writer backends have no pixel metrics; Pixels is (0,0).
func (a *ANSIBackend) WindowSize() (WindowSize, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return WindowSize{
		ColumnsRows: layout.Size{Width: a.width, Height: a.height},
		Pixels:      layout.Size{},
	}, nil
}

// Flush implements Backend.
func (a *ANSIBackend) Flush() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if f, ok := a.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// ScrollRegionUp implements ScrollingRegionBackend.
//
// Emits DECSTBM for [start, end), SU, then resets the scrolling region.
// start/end are 0-based half-open row indices. amount <= 0 is a no-op.
func (a *ANSIBackend) ScrollRegionUp(start, end, amount int) error {
	return a.scrollRegion(start, end, amount, true)
}

// ScrollRegionDown implements ScrollingRegionBackend.
//
// Emits DECSTBM for [start, end), SD, then resets the scrolling region.
// start/end are 0-based half-open row indices. amount <= 0 is a no-op.
func (a *ANSIBackend) ScrollRegionDown(start, end, amount int) error {
	return a.scrollRegion(start, end, amount, false)
}

func (a *ANSIBackend) scrollRegion(start, end, amount int, up bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if amount <= 0 {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	// Inclusive 1-based last row for DECSTBM from half-open [start,end).
	first := start + 1
	last := end
	if last < first {
		return nil
	}
	a.buf.Reset()
	a.buf.WriteByte('\x1b')
	a.buf.WriteByte('[')
	appendUint(&a.buf, uint64(first))
	a.buf.WriteByte(';')
	appendUint(&a.buf, uint64(last))
	a.buf.WriteByte('r')
	a.buf.WriteByte('\x1b')
	a.buf.WriteByte('[')
	appendUint(&a.buf, uint64(amount))
	if up {
		a.buf.WriteByte('S')
	} else {
		a.buf.WriteByte('T')
	}
	a.buf.WriteString(seqScrollRegionRst)
	_, err := a.w.Write(a.buf.Bytes())
	if err != nil {
		return err
	}
	if f, ok := a.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

func writeModifier(buf *bytes.Buffer, m style.Modifier) {
	if m == 0 {
		return
	}
	if m&style.ModBold != 0 {
		buf.WriteString("\x1b[1m")
	}
	if m&style.ModDim != 0 {
		buf.WriteString("\x1b[2m")
	}
	if m&style.ModItalic != 0 {
		buf.WriteString("\x1b[3m")
	}
	if m&style.ModUnderlined != 0 {
		buf.WriteString("\x1b[4m")
	}
	if m&style.ModSlowBlink != 0 {
		buf.WriteString("\x1b[5m")
	}
	if m&style.ModRapidBlink != 0 {
		buf.WriteString("\x1b[6m")
	}
	if m&style.ModReversed != 0 {
		buf.WriteString("\x1b[7m")
	}
	if m&style.ModHidden != 0 {
		buf.WriteString("\x1b[8m")
	}
	if m&style.ModCrossedOut != 0 {
		buf.WriteString("\x1b[9m")
	}
}

func writeFG(buf *bytes.Buffer, c style.Color) {
	writeColor(buf, true, c)
}

func writeBG(buf *bytes.Buffer, c style.Color) {
	writeColor(buf, false, c)
}

// writeColor emits an SGR foreground (fg=true) or background (fg=false) color.
func writeColor(buf *bytes.Buffer, fg bool, c style.Color) {
	switch c.Kind() {
	case style.KindUnset:
		return
	case style.KindReset:
		if fg {
			buf.WriteString("\x1b[39m")
		} else {
			buf.WriteString("\x1b[49m")
		}
		return
	case style.KindRGB:
		r, g, b, ok := c.RGB()
		if !ok {
			return
		}
		writeRGBColor(buf, fg, r, g, b)
		return
	case style.KindIndexed:
		idx, ok := c.Index()
		if !ok {
			return
		}
		writeIndexedColor(buf, fg, int(idx))
		return
	case style.KindNamed:
		idx, ok := c.Index()
		if !ok {
			if code, ok2 := ansiNamedCode(c); ok2 {
				if fg {
					writeSGRCode(buf, code)
				} else {
					writeSGRCode(buf, code+10)
				}
			}
			return
		}
		// Named palette: 0..7 normal ANSI, 8..15 bright ANSI.
		var code int
		if idx < 8 {
			code = 30 + int(idx)
		} else if idx < 16 {
			code = 90 + int(idx-8)
		} else {
			writeIndexedColor(buf, fg, int(idx))
			return
		}
		if fg {
			writeSGRCode(buf, code)
		} else {
			writeSGRCode(buf, code+10)
		}
		return
	default:
		if r, g, b, ok := c.RGB(); ok {
			writeRGBColor(buf, fg, r, g, b)
			return
		}
		if idx, ok := c.Index(); ok {
			writeIndexedColor(buf, fg, int(idx))
			return
		}
		if code, ok := ansiNamedCode(c); ok {
			if fg {
				writeSGRCode(buf, code)
			} else {
				writeSGRCode(buf, code+10)
			}
		}
	}
}

func writeUnderlineColor(buf *bytes.Buffer, c style.Color) {
	switch c.Kind() {
	case style.KindUnset:
		return
	case style.KindReset:
		buf.WriteString("\x1b[59m")
		return
	case style.KindRGB:
		r, g, b, ok := c.RGB()
		if !ok {
			return
		}
		writeUnderlineRGB(buf, r, g, b)
		return
	case style.KindIndexed:
		idx, ok := c.Index()
		if !ok {
			return
		}
		writeUnderlineIndexed(buf, int(idx))
		return
	case style.KindNamed:
		idx, ok := c.Index()
		if ok {
			writeUnderlineIndexed(buf, int(idx))
		}
		return
	default:
		if r, g, b, ok := c.RGB(); ok {
			writeUnderlineRGB(buf, r, g, b)
			return
		}
		if idx, ok := c.Index(); ok {
			writeUnderlineIndexed(buf, int(idx))
		}
	}
}

func ansiNamedCode(c style.Color) (int, bool) {
	switch {
	case c == style.Black:
		return 30, true
	case c == style.Red:
		return 31, true
	case c == style.Green:
		return 32, true
	case c == style.Yellow:
		return 33, true
	case c == style.Blue:
		return 34, true
	case c == style.Magenta:
		return 35, true
	case c == style.Cyan:
		return 36, true
	case c == style.Gray:
		return 37, true
	case c == style.DarkGray:
		return 90, true
	case c == style.LightRed:
		return 91, true
	case c == style.LightGreen:
		return 92, true
	case c == style.LightYellow:
		return 93, true
	case c == style.LightBlue:
		return 94, true
	case c == style.LightMagenta:
		return 95, true
	case c == style.LightCyan:
		return 96, true
	case c == style.White:
		return 97, true
	case c == style.Reset:
		return 39, true
	default:
		return 0, false
	}
}

// --- non-allocating ANSI integer emitters ---------------------------------

// writeCursorMove writes CSI <row>;<col> H with 1-based coordinates.
func writeCursorMove(buf *bytes.Buffer, col, row int) {
	buf.WriteByte(0x1b)
	buf.WriteByte('[')
	appendUint(buf, uint64(uint(row)))
	buf.WriteByte(';')
	appendUint(buf, uint64(uint(col)))
	buf.WriteByte('H')
}

// writeSGRCode writes CSI <n> m.
func writeSGRCode(buf *bytes.Buffer, code int) {
	buf.WriteByte(0x1b)
	buf.WriteByte('[')
	appendUint(buf, uint64(uint(code)))
	buf.WriteByte('m')
}

// writeRGBColor writes CSI 38;2;r;g;b m or CSI 48;2;r;g;b m.
func writeRGBColor(buf *bytes.Buffer, fg bool, r, g, b uint8) {
	buf.WriteByte(0x1b)
	buf.WriteByte('[')
	if fg {
		buf.WriteString("38;2;")
	} else {
		buf.WriteString("48;2;")
	}
	appendUint(buf, uint64(r))
	buf.WriteByte(';')
	appendUint(buf, uint64(g))
	buf.WriteByte(';')
	appendUint(buf, uint64(b))
	buf.WriteByte('m')
}

// writeIndexedColor writes CSI 38;5;n m or CSI 48;5;n m.
func writeIndexedColor(buf *bytes.Buffer, fg bool, idx int) {
	buf.WriteByte(0x1b)
	buf.WriteByte('[')
	if fg {
		buf.WriteString("38;5;")
	} else {
		buf.WriteString("48;5;")
	}
	appendUint(buf, uint64(uint(idx)))
	buf.WriteByte('m')
}

// writeUnderlineRGB writes CSI 58;2;r;g;b m.
func writeUnderlineRGB(buf *bytes.Buffer, r, g, b uint8) {
	buf.WriteByte(0x1b)
	buf.WriteByte('[')
	buf.WriteString("58;2;")
	appendUint(buf, uint64(r))
	buf.WriteByte(';')
	appendUint(buf, uint64(g))
	buf.WriteByte(';')
	appendUint(buf, uint64(b))
	buf.WriteByte('m')
}

// writeUnderlineIndexed writes CSI 58;5;n m.
func writeUnderlineIndexed(buf *bytes.Buffer, idx int) {
	buf.WriteByte(0x1b)
	buf.WriteByte('[')
	buf.WriteString("58;5;")
	appendUint(buf, uint64(uint(idx)))
	buf.WriteByte('m')
}

// appendUint writes n in decimal with no allocation.
func appendUint(buf *bytes.Buffer, n uint64) {
	if n == 0 {
		buf.WriteByte('0')
		return
	}
	var tmp [20]byte
	i := len(tmp)
	for n > 0 {
		i--
		tmp[i] = byte('0' + n%10)
		n /= 10
	}
	buf.Write(tmp[i:])
}
