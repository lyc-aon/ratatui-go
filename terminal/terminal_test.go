package terminal

import (
	"errors"
	"testing"

	"github.com/michaelkelly/ratatui-go/backend"
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
)

type failingBackend struct {
	backend.Backend
	failDraw  bool
	failFlush bool
}

func (f *failingBackend) Draw(cells []buffer.PositionedCell) error {
	if f.failDraw {
		return errors.New("simulated draw error")
	}
	return f.Backend.Draw(cells)
}

func (f *failingBackend) Flush() error {
	if f.failFlush {
		return errors.New("simulated flush error")
	}
	return f.Backend.Flush()
}

// noScrollBackend forwards Backend only, so InsertBefore takes the fallback path
// even when the inner TestBackend also implements ScrollingRegionBackend.
type noScrollBackend struct {
	inner *backend.TestBackend
}

func (n *noScrollBackend) Draw(cells []buffer.PositionedCell) error {
	return n.inner.Draw(cells)
}
func (n *noScrollBackend) HideCursor() error { return n.inner.HideCursor() }
func (n *noScrollBackend) ShowCursor() error { return n.inner.ShowCursor() }
func (n *noScrollBackend) GetCursorPosition() (layout.Position, error) {
	return n.inner.GetCursorPosition()
}
func (n *noScrollBackend) SetCursorPosition(pos layout.Position) error {
	return n.inner.SetCursorPosition(pos)
}
func (n *noScrollBackend) Clear() error { return n.inner.Clear() }
func (n *noScrollBackend) ClearRegion(clearType backend.ClearType) error {
	return n.inner.ClearRegion(clearType)
}
func (n *noScrollBackend) AppendLines(nLines int) error {
	return n.inner.AppendLines(nLines)
}
func (n *noScrollBackend) Size() (layout.Size, error) { return n.inner.Size() }
func (n *noScrollBackend) WindowSize() (backend.WindowSize, error) {
	return n.inner.WindowSize()
}
func (n *noScrollBackend) Flush() error { return n.inner.Flush() }

func TestTwoFrameDiff(t *testing.T) {
	tb := backend.NewTestBackend(10, 5)
	term, err := New(tb)
	if err != nil {
		t.Fatalf("failed to create terminal: %v", err)
	}

	// Frame 1: Write "Hello"
	_, err = term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "Hello", style.New())
	})
	if err != nil {
		t.Fatalf("Frame 1 Draw error: %v", err)
	}
	if term.FrameCount() != 1 {
		t.Errorf("FrameCount after 1st draw = %d, want 1", term.FrameCount())
	}
	if lines := tb.BufferLines(); lines[0] != "Hello     " {
		t.Errorf("Backend line 0 = %q, want %q", lines[0], "Hello     ")
	}

	// Frame 2: Write "Hello" (unchanged) and "World" at row 1
	_, err = term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "Hello", style.New())
		f.Buffer().SetString(0, 1, "World", style.New())
	})
	if err != nil {
		t.Fatalf("Frame 2 Draw error: %v", err)
	}
	if term.FrameCount() != 2 {
		t.Errorf("FrameCount after 2nd draw = %d, want 2", term.FrameCount())
	}
	lines := tb.BufferLines()
	if lines[0] != "Hello     " {
		t.Errorf("Backend line 0 = %q, want %q", lines[0], "Hello     ")
	}
	if lines[1] != "World     " {
		t.Errorf("Backend line 1 = %q, want %q", lines[1], "World     ")
	}
}

func TestFailedDrawAndBackendFlushCommitOrder(t *testing.T) {
	tb := backend.NewTestBackend(10, 5)
	fb := &failingBackend{Backend: tb, failDraw: true}

	term, err := New(fb)
	if err != nil {
		t.Fatalf("failed to create terminal: %v", err)
	}

	// Draw fails
	_, err = term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "Fail", style.New())
	})
	if err == nil {
		t.Fatalf("expected error from failed Draw, got nil")
	}
	if term.FrameCount() != 0 {
		t.Errorf("FrameCount after failed draw = %d, want 0", term.FrameCount())
	}

	// Allow draw to succeed now
	fb.failDraw = false
	_, err = term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "Fail", style.New())
	})
	if err != nil {
		t.Fatalf("subsequent Draw error: %v", err)
	}
	if term.FrameCount() != 1 {
		t.Errorf("FrameCount after successful draw = %d, want 1", term.FrameCount())
	}

	// Now fail Flush
	fb.failFlush = true
	_, err = term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "Pass2", style.New())
	})
	if err == nil {
		t.Fatalf("expected error from failed Flush, got nil")
	}
	if term.FrameCount() != 1 {
		t.Errorf("FrameCount after failed flush = %d, want 1", term.FrameCount())
	}
	if got, ok := term.CurrentBuffer().Get(0, 0); !ok || got.DisplaySymbol() != " " {
		t.Fatalf("current buffer after failed backend flush = %q, ok=%v; want reset buffer", got.DisplaySymbol(), ok)
	}
}

func TestTryDrawRenderErrorIsAtomic(t *testing.T) {
	tb := backend.NewTestBackend(10, 5)
	term, err := New(tb)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Seed one successful frame.
	if _, err := term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "OK", style.New())
	}); err != nil {
		t.Fatalf("seed Draw: %v", err)
	}
	before := tb.BufferLines()[0]
	count := term.FrameCount()

	boom := errors.New("render boom")
	_, err = term.TryDraw(func(f *Frame) error {
		f.Buffer().SetString(0, 0, "XX", style.New())
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("TryDraw err = %v, want boom", err)
	}
	if term.FrameCount() != count {
		t.Errorf("FrameCount = %d, want %d (unchanged)", term.FrameCount(), count)
	}
	// Backend must not have received the aborted frame's cells.
	if got := tb.BufferLines()[0]; got != before {
		t.Errorf("backend line0 = %q, want %q (no backend output on render error)", got, before)
	}
}

func TestFixedClearResetCells(t *testing.T) {
	tb := backend.NewTestBackend(10, 10)
	fixedArea := layout.NewRect(2, 2, 4, 3)

	term, err := New(tb, WithViewport(ViewportFixed), WithFixedArea(fixedArea))
	if err != nil {
		t.Fatalf("failed to create terminal with fixed viewport: %v", err)
	}

	err = term.Clear()
	if err != nil {
		t.Fatalf("Clear error: %v", err)
	}

	// Verify cells in fixed area were cleared to space symbol
	b := tb.Buffer()
	for y := fixedArea.Y; y < fixedArea.Bottom(); y++ {
		for x := fixedArea.X; x < fixedArea.Right(); x++ {
			c, ok := b.Get(x, y)
			if !ok {
				t.Errorf("cell (%d, %d) not found in backend buffer", x, y)
			} else if c.DisplaySymbol() != " " {
				t.Errorf("cell (%d, %d) symbol = %q, want space", x, y, c.DisplaySymbol())
			}
		}
	}
}

func TestInlineResizeAppendBehavior(t *testing.T) {
	tb := backend.NewTestBackend(20, 10)

	term, err := New(tb, WithViewport(ViewportInline), WithInlineHeight(4))
	if err != nil {
		t.Fatalf("failed to create terminal with inline viewport: %v", err)
	}

	area := term.ViewportArea()
	if area.Height != 4 {
		t.Errorf("inline viewport height = %d, want 4", area.Height)
	}
	if area.Width != 20 {
		t.Errorf("inline viewport width = %d, want 20", area.Width)
	}

	// Resize backend size and trigger Autoresize
	tb.Resize(20, 15)
	err = term.Autoresize()
	if err != nil {
		t.Fatalf("Autoresize error: %v", err)
	}

	newArea := term.ViewportArea()
	if newArea.Width != 20 || newArea.Height != 4 {
		t.Errorf("viewport area after autoresize = %+v, want width 20 height 4", newArea)
	}
}

func TestInlineResizePreservesCursorAcrossRepeatedResizes(t *testing.T) {
	tb := backend.NewTestBackend(10, 10)
	if err := tb.SetCursorPosition(layout.NewPosition(0, 4)); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}
	term, err := New(tb, WithInlineHeight(4))
	if err != nil {
		t.Fatalf("New inline: %v", err)
	}

	term.lastKnownCursorPos = layout.NewPosition(0, 5)
	if err := tb.SetCursorPosition(layout.NewPosition(0, 6)); err != nil {
		t.Fatalf("move cursor: %v", err)
	}
	if err := term.Resize(layout.NewRect(0, 0, 10, 12)); err != nil {
		t.Fatalf("first Resize: %v", err)
	}
	if got, want := term.ViewportArea(), layout.NewRect(0, 5, 10, 4); got != want {
		t.Fatalf("first viewport = %+v, want %+v", got, want)
	}
	if got, err := tb.GetCursorPosition(); err != nil || got != layout.NewPosition(0, 6) {
		t.Fatalf("cursor after first resize = %+v, %v", got, err)
	}

	if err := term.Resize(layout.NewRect(0, 0, 10, 14)); err != nil {
		t.Fatalf("second Resize: %v", err)
	}
	if got, want := term.ViewportArea(), layout.NewRect(0, 6, 10, 4); got != want {
		t.Fatalf("second viewport = %+v, want %+v", got, want)
	}
	if got, err := tb.GetCursorPosition(); err != nil || got != layout.NewPosition(0, 6) {
		t.Fatalf("cursor after second resize = %+v, %v", got, err)
	}
}

func TestResizeHorizontalShrinkClearsAllViewports(t *testing.T) {
	// Fullscreen: shrinking width must ClearRegion(All) and force Y=0.
	tb := backend.NewTestBackend(10, 5)
	term, err := New(tb)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := term.Draw(func(f *Frame) {
		f.Buffer().SetString(0, 0, "ABCDEFGHIJ", style.New())
	}); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	if err := term.Resize(layout.NewRect(0, 0, 5, 5)); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if y := term.ViewportArea().Y; y != 0 {
		t.Errorf("fullscreen viewport Y after shrink = %d, want 0", y)
	}
	if w := term.ViewportArea().Width; w != 5 {
		t.Errorf("fullscreen viewport width = %d, want 5", w)
	}

	// Fixed viewport: same horizontal-shrink full clear + Y forced to 0.
	tb2 := backend.NewTestBackend(10, 10)
	fixed := layout.NewRect(1, 2, 6, 3)
	term2, err := New(tb2, WithFixedArea(fixed))
	if err != nil {
		t.Fatalf("New fixed: %v", err)
	}
	if err := term2.Resize(layout.NewRect(1, 2, 3, 3)); err != nil {
		t.Fatalf("Resize fixed: %v", err)
	}
	if y := term2.ViewportArea().Y; y != 0 {
		t.Errorf("fixed viewport Y after shrink = %d, want 0", y)
	}
}

func TestInsertBeforeNonInlineNoop(t *testing.T) {
	tb := backend.NewTestBackend(10, 5)
	term, err := New(tb)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before := term.ViewportArea()
	if err := term.InsertBefore(2, func(b *buffer.Buffer) {
		b.SetString(0, 0, "nope", style.New())
	}); err != nil {
		t.Fatalf("InsertBefore: %v", err)
	}
	if term.ViewportArea() != before {
		t.Errorf("viewport changed on non-inline InsertBefore: %+v -> %+v", before, term.ViewportArea())
	}
}

func TestInsertBeforeFallbackPushesViewport(t *testing.T) {
	inner := backend.NewTestBackend(10, 10)
	// Place cursor so inline viewport starts below existing content.
	_ = inner.SetCursorPosition(layout.Position{X: 0, Y: 3})
	tb := &noScrollBackend{inner: inner}

	term, err := New(tb, WithInlineHeight(4))
	if err != nil {
		t.Fatalf("New inline: %v", err)
	}
	startY := term.ViewportArea().Y
	if startY != 3 {
		t.Fatalf("inline start Y = %d, want 3", startY)
	}

	if err := term.InsertBefore(1, func(b *buffer.Buffer) {
		b.SetString(0, 0, "LOG", style.New())
	}); err != nil {
		t.Fatalf("InsertBefore: %v", err)
	}
	// Viewport should move down by the inserted height when space remains.
	if got := term.ViewportArea().Y; got != startY+1 {
		t.Errorf("viewport Y after InsertBefore = %d, want %d", got, startY+1)
	}
	// Fallback path ends with Clear; next draw must still work.
	if _, err := term.Draw(func(f *Frame) {
		f.Buffer().SetString(f.Area().X, f.Area().Y, "UI", style.New())
	}); err != nil {
		t.Fatalf("Draw after InsertBefore: %v", err)
	}
}

func TestInsertBeforeScrollingRegionsPushesViewport(t *testing.T) {
	tb := backend.NewTestBackend(10, 10)
	_ = tb.SetCursorPosition(layout.Position{X: 0, Y: 3})
	term, err := New(tb, WithInlineHeight(4))
	if err != nil {
		t.Fatalf("New inline: %v", err)
	}
	startY := term.ViewportArea().Y

	// TestBackend implements ScrollingRegionBackend → fast path.
	if _, ok := any(tb).(backend.ScrollingRegionBackend); !ok {
		t.Fatal("TestBackend should implement ScrollingRegionBackend")
	}

	if err := term.InsertBefore(1, func(b *buffer.Buffer) {
		b.SetString(0, 0, "LOG", style.New())
	}); err != nil {
		t.Fatalf("InsertBefore: %v", err)
	}
	if got := term.ViewportArea().Y; got != startY+1 {
		t.Errorf("viewport Y after scrolling InsertBefore = %d, want %d", got, startY+1)
	}
}

func TestInsertBeforeZeroHeightSafe(t *testing.T) {
	tb := backend.NewTestBackend(10, 10)
	term, err := New(tb, WithInlineHeight(2))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	before := term.ViewportArea()
	if err := term.InsertBefore(0, nil); err != nil {
		t.Fatalf("InsertBefore(0): %v", err)
	}
	if err := term.InsertBefore(-3, nil); err != nil {
		t.Fatalf("InsertBefore(-3): %v", err)
	}
	// Scrolling path with height 0 should not move the viewport.
	if term.ViewportArea() != before {
		t.Errorf("viewport changed on zero InsertBefore: %+v -> %+v", before, term.ViewportArea())
	}
}

func TestCloseRestoresHiddenCursor(t *testing.T) {
	tb := backend.NewTestBackend(10, 5)
	term, err := New(tb)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := term.HideCursor(); err != nil {
		t.Fatalf("HideCursor: %v", err)
	}
	if tb.CursorVisible() {
		t.Fatal("cursor still visible after HideCursor")
	}
	if err := term.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !tb.CursorVisible() {
		t.Fatal("cursor still hidden after Close")
	}
	// Idempotent when already shown.
	if err := term.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestApplyBufferExportsCompletedFrame(t *testing.T) {
	tb := backend.NewTestBackend(8, 3)
	term, err := New(tb)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f := term.GetFrame()
	f.Buffer().SetString(0, 0, "AB", style.New())
	cf, err := term.ApplyBuffer()
	if err != nil {
		t.Fatalf("ApplyBuffer: %v", err)
	}
	if cf.Count != 0 {
		t.Errorf("CompletedFrame.Count = %d, want 0", cf.Count)
	}
	if term.FrameCount() != 1 {
		t.Errorf("FrameCount = %d, want 1", term.FrameCount())
	}
	if lines := tb.BufferLines(); lines[0][:2] != "AB" {
		t.Errorf("backend line0 = %q, want AB…", lines[0])
	}
}
