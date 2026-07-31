// Package backend abstracts terminal output for ratatui-go.
//
// Terminal owns double-buffered diffs; backends own writing cells, cursor
// control, clear, and size reporting. No package-level mutable render state.
package backend

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
)

// ClearType selects which region of the visible display area is cleared.
//
// Clearing operates on character cells. It does not move, hide, or reset the
// cursor. If the cursor lies inside the cleared region, that cell is cleared.
// Clearing applies to the active display surface only; scrollback and
// off-screen buffers are not guaranteed to change.
type ClearType int

const (
	// All clears every character cell in the visible display area.
	All ClearType = iota
	// AfterCursor clears from the cursor cell (inclusive) through the end of
	// the display area.
	AfterCursor
	// BeforeCursor clears from the start of the display area through the
	// cursor cell (inclusive).
	BeforeCursor
	// CurrentLine clears every character cell on the cursor's current line.
	CurrentLine
	// UntilNewLine clears from the cursor cell (inclusive) to the end of the
	// current line.
	UntilNewLine
)

// WindowSize is the terminal size in character cells and pixels.
//
// Pixels may be (0,0) when the host API does not expose a pixel size.
type WindowSize struct {
	ColumnsRows layout.Size
	Pixels      layout.Size
}

// Backend draws cell updates and manipulates the cursor on a display surface.
//
// Implementations must accept empty draw batches and zero-sized terminals
// without panicking. Draw applies the given cells; Flush pushes any buffered
// output to the underlying sink.
type Backend interface {
	// Draw applies positioned cells to the display surface.
	// Cells may be sparse and unordered. An empty slice is a no-op.
	Draw(cells []buffer.PositionedCell) error

	// HideCursor makes the cursor invisible.
	HideCursor() error

	// ShowCursor makes the cursor visible.
	ShowCursor() error

	// GetCursorPosition returns the cursor position in cell coordinates.
	GetCursorPosition() (layout.Position, error)

	// SetCursorPosition moves the cursor to position.
	SetCursorPosition(pos layout.Position) error

	// Clear clears the visible display surface.
	// Equivalent to ClearRegion(All). Must preserve cursor position.
	Clear() error

	// ClearRegion clears a region of the visible display defined by clearType.
	// Must preserve cursor position.
	ClearRegion(clearType ClearType) error

	// AppendLines inserts n line breaks at the current cursor, scrolling the
	// display when the cursor is near the bottom. Used to reserve space for
	// inline viewports. n <= 0 is a no-op.
	AppendLines(n int) error

	// Size returns the current terminal size in columns and rows.
	Size() (layout.Size, error)

	// WindowSize returns the terminal size in cells and, when available, pixels.
	WindowSize() (WindowSize, error)

	// Flush flushes any backend-buffered output.
	Flush() error
}

// ScrollingRegionBackend is an optional extension for DECSTBM-style region
// scrolling. start/end are a half-open row range [start, end) in 0-based
// screen coordinates. amount is the number of rows to scroll. amount <= 0 is a
// no-op. Cursor position afterwards is undefined.
type ScrollingRegionBackend interface {
	ScrollRegionUp(start, end, amount int) error
	ScrollRegionDown(start, end, amount int) error
}

// Compile-time interface satisfaction checks.
var (
	_ Backend                = (*ANSIBackend)(nil)
	_ Backend                = (*TestBackend)(nil)
	_ Backend                = (*TTYBackend)(nil)
	_ ScrollingRegionBackend = (*ANSIBackend)(nil)
	_ ScrollingRegionBackend = (*TestBackend)(nil)
	_ ScrollingRegionBackend = (*TTYBackend)(nil)
)
