package backend

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"golang.org/x/term"
)

// TTYBackend is a real-terminal Backend over an *os.File.
//
// It reuses ANSI emission for drawing and cursor control, reports live size
// from the host TTY, and can query the real cursor position via CPR. Raw mode
// and alternate-screen lifecycle are explicit methods; they are not enabled by
// construction.
type TTYBackend struct {
	// out is the write side (usually stdout). Drawing and size queries use it.
	out   *os.File
	outFD int

	// in is the read side for raw mode and CPR replies (usually stdin).
	// When out is a TTY that is also readable it may equal out; otherwise it
	// is resolved to the controlling terminal input.
	in   *os.File
	inFD int

	mu sync.Mutex

	// ansi is the emission helper. Its writer is out; size is refreshed from
	// the live TTY on Size/WindowSize.
	ansi *ANSIBackend

	// rawState is non-nil while raw mode is active on this backend.
	rawState *term.State
	// rawFD is the descriptor raw mode was applied to (inFD).
	rawFD int
	// altScreen tracks whether we entered the alternate screen via this backend.
	altScreen bool

	// cursorQueryTimeout bounds CPR reads for GetCursorPosition.
	cursorQueryTimeout time.Duration
}

// NewTTYBackend wraps out as a TTY backend.
//
// out must be non-nil and refer to a terminal device used for drawing and size.
// Raw mode and CPR replies use the controlling input TTY (os.Stdin when it is a
// terminal, otherwise out when readable). Size is taken live from the host;
// there is no static-size fallback when attached to a TTY.
func NewTTYBackend(out *os.File) (*TTYBackend, error) {
	if out == nil {
		return nil, fmt.Errorf("backend: nil tty file")
	}
	outFD := int(out.Fd())
	if !term.IsTerminal(outFD) {
		return nil, fmt.Errorf("backend: file is not a terminal")
	}
	width, height, err := term.GetSize(outFD)
	if err != nil {
		return nil, fmt.Errorf("backend: get terminal size: %w", err)
	}
	in, inFD := resolveInputTTY(out, outFD)
	return &TTYBackend{
		out:                out,
		outFD:              outFD,
		in:                 in,
		inFD:               inFD,
		ansi:               NewANSIBackend(out, width, height),
		cursorQueryTimeout: 200 * time.Millisecond,
	}, nil
}

// resolveInputTTY picks the file descriptor used for raw mode and CPR reads.
func resolveInputTTY(out *os.File, outFD int) (*os.File, int) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, int(os.Stdin.Fd())
	}
	// Fall back to the output descriptor when it is the only TTY (e.g. tests
	// that pass a bidirectional PTY slave).
	return out, outFD
}

// NewTTYBackendStdout wraps os.Stdout.
func NewTTYBackendStdout() (*TTYBackend, error) {
	return NewTTYBackend(os.Stdout)
}

// File returns the output *os.File used for drawing.
func (t *TTYBackend) File() *os.File {
	return t.out
}

// Fd returns the output terminal file descriptor.
func (t *TTYBackend) Fd() int {
	return t.outFD
}

// InputFile returns the input *os.File used for raw mode and CPR. Do not read
// it concurrently with GetCursorPosition: a CPR query owns the input stream
// until it receives the terminal reply or times out.
func (t *TTYBackend) InputFile() *os.File {
	return t.in
}

// EnableRawMode puts the input terminal into raw mode. Safe to call when
// already raw (no-op). The prior termios state is retained for DisableRawMode.
func (t *TTYBackend) EnableRawMode() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rawState != nil {
		return nil
	}
	if !term.IsTerminal(t.inFD) {
		return fmt.Errorf("backend: input is not a terminal")
	}
	state, err := term.MakeRaw(t.inFD)
	if err != nil {
		return fmt.Errorf("backend: enable raw mode: %w", err)
	}
	t.rawState = state
	t.rawFD = t.inFD
	return nil
}

// DisableRawMode restores the termios state captured by EnableRawMode.
// No-op if raw mode was not enabled by this backend.
func (t *TTYBackend) DisableRawMode() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rawState == nil {
		return nil
	}
	if err := term.Restore(t.rawFD, t.rawState); err != nil {
		// Keep the captured state so DisableRawMode can be retried.
		return fmt.Errorf("backend: disable raw mode: %w", err)
	}
	t.rawState = nil
	t.rawFD = 0
	return nil
}

// EnterAlternateScreen writes the alternate-screen enter sequence.
func (t *TTYBackend) EnterAlternateScreen() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := EnterAlternateScreen(t.out); err != nil {
		return err
	}
	t.altScreen = true
	return nil
}

// LeaveAlternateScreen writes the alternate-screen leave sequence.
func (t *TTYBackend) LeaveAlternateScreen() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := LeaveAlternateScreen(t.out); err != nil {
		return err
	}
	t.altScreen = false
	return nil
}

// InRawMode reports whether this backend currently owns raw mode.
func (t *TTYBackend) InRawMode() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rawState != nil
}

// InAlternateScreen reports whether this backend entered the alternate screen.
func (t *TTYBackend) InAlternateScreen() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.altScreen
}

// refreshSize updates the embedded ANSI backend dimensions from the live TTY.
func (t *TTYBackend) refreshSize() (layout.Size, error) {
	width, height, err := term.GetSize(t.outFD)
	if err != nil {
		return layout.Size{}, fmt.Errorf("backend: get terminal size: %w", err)
	}
	t.ansi.Resize(width, height)
	return layout.Size{Width: width, Height: height}, nil
}

// Draw implements Backend.
func (t *TTYBackend) Draw(cells []buffer.PositionedCell) error {
	return t.ansi.Draw(cells)
}

// HideCursor implements Backend.
func (t *TTYBackend) HideCursor() error {
	return t.ansi.HideCursor()
}

// ShowCursor implements Backend.
func (t *TTYBackend) ShowCursor() error {
	return t.ansi.ShowCursor()
}

// GetCursorPosition implements Backend by querying the real terminal via CPR
// (CSI 6 n). It returns query failures rather than a synthetic position.
//
// CPR shares the terminal input stream. Bytes received before the CPR reply
// cannot be returned to an independent input reader, so callers must not read
// InputFile concurrently with this method.
func (t *TTYBackend) GetCursorPosition() (layout.Position, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pos, err := queryCursorPosition(t.out, t.in, t.cursorQueryTimeout)
	if err != nil {
		return layout.Position{}, err
	}
	// Keep ANSI tracker in sync.
	t.ansi.mu.Lock()
	t.ansi.cursor = pos
	t.ansi.mu.Unlock()
	return pos, nil
}

// SetCursorPosition implements Backend.
func (t *TTYBackend) SetCursorPosition(pos layout.Position) error {
	return t.ansi.SetCursorPosition(pos)
}

// Clear implements Backend.
func (t *TTYBackend) Clear() error {
	return t.ansi.Clear()
}

// ClearRegion implements Backend.
func (t *TTYBackend) ClearRegion(clearType ClearType) error {
	return t.ansi.ClearRegion(clearType)
}

// AppendLines implements Backend.
func (t *TTYBackend) AppendLines(n int) error {
	// Keep ANSI size in sync before append clamping.
	if _, err := t.Size(); err != nil {
		return err
	}
	return t.ansi.AppendLines(n)
}

// Size implements Backend.
// Always queries the live TTY; never returns a stale static size.
func (t *TTYBackend) Size() (layout.Size, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.refreshSize()
}

// WindowSize implements Backend.
// Columns/rows always come from the live TTY. Pixels come from the host API
// when available (TIOCGWINSZ on Unix); otherwise (0,0).
func (t *TTYBackend) WindowSize() (WindowSize, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	colsRows, err := t.refreshSize()
	if err != nil {
		return WindowSize{}, err
	}
	pixels, err := windowPixelSize(t.outFD)
	if err != nil {
		// Pixel size is best-effort; cell size already succeeded.
		pixels = layout.Size{}
	}
	return WindowSize{
		ColumnsRows: colsRows,
		Pixels:      pixels,
	}, nil
}

// Flush implements Backend.
func (t *TTYBackend) Flush() error {
	return t.ansi.Flush()
}

// ScrollRegionUp implements ScrollingRegionBackend.
func (t *TTYBackend) ScrollRegionUp(start, end, amount int) error {
	return t.ansi.ScrollRegionUp(start, end, amount)
}

// ScrollRegionDown implements ScrollingRegionBackend.
func (t *TTYBackend) ScrollRegionDown(start, end, amount int) error {
	return t.ansi.ScrollRegionDown(start, end, amount)
}

// queryCursorPosition writes CSI 6 n on out and parses the CPR reply from in.
// Coordinates are converted from 1-based ANSI to 0-based layout.Position.
func queryCursorPosition(out, in *os.File, timeout time.Duration) (layout.Position, error) {
	if out == nil || in == nil {
		return layout.Position{}, fmt.Errorf("backend: nil tty for cursor query")
	}
	if _, err := io.WriteString(out, "\x1b[6n"); err != nil {
		return layout.Position{}, fmt.Errorf("backend: write cursor position request: %w", err)
	}

	if timeout <= 0 {
		timeout = 200 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)

	var buf bytes.Buffer
	tmp := make([]byte, 64)
	for time.Now().Before(deadline) {
		wait := time.Until(deadline)
		if wait > 20*time.Millisecond {
			wait = 20 * time.Millisecond
		}
		n, err := readTTYWithTimeout(in, tmp, wait)
		if n > 0 {
			buf.Write(tmp[:n])
			if pos, ok := parseCPR(buf.Bytes()); ok {
				return pos, nil
			}
		}
		if err != nil && !isTimeout(err) && buf.Len() == 0 {
			return layout.Position{}, fmt.Errorf("backend: read cursor position: %w", err)
		}
	}
	if pos, ok := parseCPR(buf.Bytes()); ok {
		return pos, nil
	}
	return layout.Position{}, fmt.Errorf("backend: cursor position query timed out")
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	type timeout interface{ Timeout() bool }
	if te, ok := err.(timeout); ok && te.Timeout() {
		return true
	}
	return false
}

// parseCPR finds CSI row;col R in data and returns a 0-based position.
func parseCPR(data []byte) (layout.Position, bool) {
	// Scan for ESC [ ... R
	for i := 0; i < len(data); i++ {
		if data[i] != 0x1b {
			continue
		}
		if i+1 >= len(data) || data[i+1] != '[' {
			continue
		}
		j := i + 2
		// Optional leading '?' for some terminals; skip non-digits briefly.
		for j < len(data) && (data[j] < '0' || data[j] > '9') && data[j] != 'R' {
			// allow nothing but digits/semicolon once numbers start; skip one marker
			if data[j] == '?' {
				j++
				continue
			}
			break
		}
		start := j
		for j < len(data) && data[j] != 'R' {
			j++
		}
		if j >= len(data) || data[j] != 'R' {
			continue
		}
		payload := string(data[start:j])
		// payload is "row;col"
		semi := -1
		for k := 0; k < len(payload); k++ {
			if payload[k] == ';' {
				semi = k
				break
			}
		}
		if semi < 0 {
			continue
		}
		row, err1 := strconv.Atoi(payload[:semi])
		col, err2 := strconv.Atoi(payload[semi+1:])
		if err1 != nil || err2 != nil {
			continue
		}
		if row < 1 {
			row = 1
		}
		if col < 1 {
			col = 1
		}
		return layout.Position{X: col - 1, Y: row - 1}, true
	}
	return layout.Position{}, false
}
