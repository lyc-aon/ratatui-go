// Package terminal provides double-buffered Frame rendering over a Backend.
//
// Draw renders widgets into the current buffer, diffs against the previous
// buffer, sends updates to the backend, and only swaps buffers after a
// successful backend Draw+Flush. Failed draws leave the prior buffer intact.
package terminal

import (
	"fmt"

	"github.com/michaelkelly/ratatui-go/backend"
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/widget"
)

// Viewport controls where the terminal draws.
type Viewport int

const (
	// ViewportFullscreen draws into the entire backend surface (default).
	ViewportFullscreen Viewport = iota
	// ViewportFixed draws into a fixed rectangle in terminal coordinates.
	// Fixed viewports are not autoresized; call Resize to change the region.
	ViewportFixed
	// ViewportInline draws a full-width band anchored to the backend cursor row.
	// Height is the requested band height in rows (clamped to terminal height).
	// Autoresize recomputes placement when the backend size changes.
	ViewportInline
)

// Options configure Terminal construction.
type Options struct {
	// Viewport selects fullscreen, fixed, or inline drawing. Zero value is fullscreen.
	Viewport Viewport
	// Area is the fixed rectangle when Viewport == ViewportFixed.
	// Ignored for fullscreen and inline.
	Area layout.Rect
	// InlineHeight is the requested inline viewport height in rows when
	// Viewport == ViewportInline. Clamped to the backend height; zero is safe.
	InlineHeight int
}

// Option is a functional option for New.
type Option func(*Options)

// WithViewport sets the viewport mode.
func WithViewport(v Viewport) Option {
	return func(o *Options) { o.Viewport = v }
}

// WithFixedArea selects a fixed viewport covering area.
func WithFixedArea(area layout.Rect) Option {
	return func(o *Options) {
		o.Viewport = ViewportFixed
		o.Area = area
	}
}

// WithInlineHeight selects an inline viewport of the given height in rows.
// Height less than zero is clamped to zero.
func WithInlineHeight(height int) Option {
	return func(o *Options) {
		o.Viewport = ViewportInline
		if height < 0 {
			height = 0
		}
		o.InlineHeight = height
	}
}

// Terminal is the main entry point for rendering Frames onto a Backend.
//
// It owns two buffers sized to the current viewport, diffs them on each Draw,
// and keeps cursor bookkeeping for the end of each successful pass.
type Terminal struct {
	backend backend.Backend

	buffers [2]*buffer.Buffer
	current int

	hiddenCursor bool

	viewport     Viewport
	viewportArea layout.Rect

	// inlineHeight is the requested height for ViewportInline (pre-clamp).
	inlineHeight int

	// lastKnownArea tracks the last observed backend size (fullscreen/inline)
	// or the fixed area (fixed viewport). Used by Autoresize.
	lastKnownArea layout.Rect

	lastKnownCursorPos layout.Position
	frameCount         int
}

// CompletedFrame is a snapshot of the buffer after a successful Draw.
//
// It is only valid until the next successful Draw, which swaps buffers.
type CompletedFrame struct {
	Buffer *buffer.Buffer
	Area   layout.Rect
	Count  int
}

// New constructs a Terminal over backend b.
//
// For fullscreen (default), the viewport is the backend's current Size.
// Zero-sized backends are accepted; drawing is a no-op until resized.
func New(b backend.Backend, opts ...Option) (*Terminal, error) {
	if b == nil {
		return nil, fmt.Errorf("terminal: nil backend")
	}
	var o Options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return newWithOptions(b, o)
}

// NewWithOptions constructs a Terminal with explicit Options.
func NewWithOptions(b backend.Backend, o Options) (*Terminal, error) {
	if b == nil {
		return nil, fmt.Errorf("terminal: nil backend")
	}
	return newWithOptions(b, o)
}

func newWithOptions(b backend.Backend, o Options) (*Terminal, error) {
	var (
		viewportArea layout.Rect
		lastKnown    layout.Rect
		cursorPos    layout.Position
		inlineH      int
	)

	switch o.Viewport {
	case ViewportFixed:
		viewportArea = clampRect(o.Area)
		lastKnown = viewportArea
		cursorPos = layout.Position{X: viewportArea.X, Y: viewportArea.Y}
	case ViewportInline:
		inlineH = o.InlineHeight
		if inlineH < 0 {
			inlineH = 0
		}
		sz, err := b.Size()
		if err != nil {
			return nil, err
		}
		lastKnown = sizeToRect(sz)
		area, pos, err := computeInlineSize(b, inlineH, sz, 0)
		if err != nil {
			return nil, err
		}
		viewportArea = area
		cursorPos = pos
	default:
		// Fullscreen
		sz, err := b.Size()
		if err != nil {
			return nil, err
		}
		viewportArea = sizeToRect(sz)
		lastKnown = viewportArea
		cursorPos = layout.Position{}
	}

	t := &Terminal{
		backend:            b,
		buffers:            [2]*buffer.Buffer{buffer.Empty(viewportArea), buffer.Empty(viewportArea)},
		current:            0,
		hiddenCursor:       false,
		viewport:           o.Viewport,
		viewportArea:       viewportArea,
		inlineHeight:       inlineH,
		lastKnownArea:      lastKnown,
		lastKnownCursorPos: cursorPos,
		frameCount:         0,
	}
	return t, nil
}

// Backend returns the underlying backend.
func (t *Terminal) Backend() backend.Backend { return t.backend }

// BackendMut returns the underlying backend for mutation.
// Prefer Draw for normal rendering; direct backend writes can desync buffers.
func (t *Terminal) BackendMut() backend.Backend { return t.backend }

// Size queries the backend size.
func (t *Terminal) Size() (layout.Size, error) { return t.backend.Size() }

// ViewportArea returns the current viewport rectangle.
func (t *Terminal) ViewportArea() layout.Rect { return t.viewportArea }

// FrameCount returns the number of successfully completed frames.
func (t *Terminal) FrameCount() int { return t.frameCount }

// CurrentBuffer returns the buffer the next Frame will render into.
func (t *Terminal) CurrentBuffer() *buffer.Buffer {
	return t.buffers[t.current]
}

// GetFrame builds a Frame for manual rendering without running the full Draw pipeline.
//
// Unlike Draw, this does not autoresize, flush, swap, or touch the cursor.
// After rendering, call Flush, SwapBuffers, and Backend.Flush yourself, or
// prefer Draw / ApplyBuffer for the full atomic path.
func (t *Terminal) GetFrame() *Frame {
	return &Frame{
		cursorPosition: nil,
		viewportArea:   t.viewportArea,
		buffer:         t.buffers[t.current],
		count:          t.frameCount,
	}
}

// Draw runs one render pass via TryDraw. The render callback cannot fail;
// use TryDraw when rendering can return an error.
//
// Pipeline:
//  1. Autoresize (fullscreen and inline)
//  2. Invoke render against a Frame backed by the current buffer
//  3. Diff previous vs current; Backend.Draw the updates
//  4. Apply cursor show/hide + position
//  5. Backend.Flush
//  6. Swap buffers (only after successful backend draw+flush)
//
// If Backend.Draw or Backend.Flush fails, the previous buffer is not swapped
// and the frame count is not incremented. The current buffer still holds the
// just-rendered content so a subsequent successful Draw can retry.
func (t *Terminal) Draw(render func(*Frame)) (CompletedFrame, error) {
	if render == nil {
		return CompletedFrame{}, fmt.Errorf("terminal: nil render callback")
	}
	return t.TryDraw(func(f *Frame) error {
		render(f)
		return nil
	})
}

// TryDraw runs one render pass whose callback may fail.
//
// If render returns an error, the backend, buffers, cursor state, and frame
// count are left unchanged (no backend output from this pass).
func (t *Terminal) TryDraw(render func(*Frame) error) (CompletedFrame, error) {
	if render == nil {
		return CompletedFrame{}, fmt.Errorf("terminal: nil render callback")
	}
	if err := t.Autoresize(); err != nil {
		return CompletedFrame{}, err
	}

	frame := t.GetFrame()
	if err := render(frame); err != nil {
		return CompletedFrame{}, err
	}
	return t.ApplyBufferWithCursor(frame.cursorPosition)
}

// ApplyBuffer flushes the current buffer diff to the backend, hides the cursor,
// flushes backend output, and swaps buffers. Prefer Draw / TryDraw for the
// full autoresize + render path.
func (t *Terminal) ApplyBuffer() (CompletedFrame, error) {
	return t.ApplyBufferWithCursor(nil)
}

// ApplyBufferWithCursor flushes the current buffer diff, applies cursor
// visibility/position, swaps buffers, then flushes backend output.
//
// The swap-before-backend-flush order matches Ratatui 0.30.2. A backend flush
// failure therefore leaves the rendered buffer committed for the next diff.
// cursor == nil hides the cursor; non-nil shows and positions it.
func (t *Terminal) ApplyBufferWithCursor(cursor *layout.Position) (CompletedFrame, error) {
	if err := t.Flush(); err != nil {
		return CompletedFrame{}, err
	}

	if cursor == nil {
		if err := t.HideCursor(); err != nil {
			return CompletedFrame{}, err
		}
	} else {
		if err := t.ShowCursor(); err != nil {
			return CompletedFrame{}, err
		}
		if err := t.SetCursorPosition(*cursor); err != nil {
			return CompletedFrame{}, err
		}
	}

	t.SwapBuffers()

	if err := t.backend.Flush(); err != nil {
		return CompletedFrame{}, err
	}
	completed := CompletedFrame{
		Buffer: t.buffers[1-t.current],
		Area:   t.lastKnownArea,
		Count:  t.frameCount,
	}
	t.frameCount++
	return completed, nil
}

// Flush diffs the current buffer against the previous buffer and sends updates
// to the backend. It does not swap buffers, update the cursor, or call
// Backend.Flush.
//
// Prefer Draw for the full atomic path.
func (t *Terminal) Flush() error {
	prev := t.buffers[1-t.current]
	curr := t.buffers[t.current]
	updates := diffBuffers(prev, curr)
	if len(updates) == 0 {
		return nil
	}
	if err := t.backend.Draw(updates); err != nil {
		return err
	}
	last := updates[len(updates)-1]
	t.lastKnownCursorPos = last.Position
	return nil
}

// SwapBuffers resets the inactive buffer and flips current/previous roles.
// Call after a successful Flush when managing the pipeline manually.
func (t *Terminal) SwapBuffers() {
	t.buffers[1-t.current].Reset()
	t.current = 1 - t.current
}

// Clear clears the backend surface for the active viewport and resets the
// previous buffer so the next Draw is a full redraw. Cursor position is
// preserved.
func (t *Terminal) Clear() error {
	orig, err := t.backend.GetCursorPosition()
	if err != nil {
		return err
	}
	if err := t.clearViewport(); err != nil {
		return err
	}
	if err := t.backend.SetCursorPosition(orig); err != nil {
		return err
	}
	return nil
}

// Resize updates internal buffers and clears so the next Draw fully repaints.
//
// For fullscreen and fixed viewports, area becomes the new viewport rectangle.
// For inline viewports, area is interpreted as the backend's new terminal size;
// the viewport origin is recomputed from the current cursor, preserving the
// cursor's relative row within the previous viewport when possible.
//
// On horizontal shrink (any viewport), the whole screen is cleared and the
// next viewport Y is forced to 0 to avoid line-wrap glitches — matching Rust
// resize.rs.
func (t *Terminal) Resize(area layout.Rect) error {
	area = clampRect(area)

	var (
		nextArea        layout.Rect
		cursorToRestore *layout.Position
	)

	switch t.viewport {
	case ViewportInline:
		// Preserve relative cursor row inside the previous viewport.
		offset := t.lastKnownCursorPos.Y - t.viewportArea.Y
		if offset < 0 {
			offset = 0
		}
		sz := layout.Size{Width: area.Width, Height: area.Height}
		computed, cursorPos, err := computeInlineSize(t.backend, t.inlineHeight, sz, offset)
		if err != nil {
			return err
		}
		nextArea = computed
		cursorToRestore = &cursorPos
	default:
		nextArea = area
	}

	// Clear screen on horizontal shrink for every viewport (Rust resize.rs).
	if nextArea.Width < t.viewportArea.Width {
		nextArea.Y = 0
		if err := t.backend.ClearRegion(backend.All); err != nil {
			return err
		}
	}

	t.setViewportArea(nextArea)
	if err := t.clearViewport(); err != nil {
		return err
	}
	if cursorToRestore != nil {
		if err := t.backend.SetCursorPosition(*cursorToRestore); err != nil {
			return err
		}
	}
	t.lastKnownArea = area
	return nil
}

// Autoresize queries the backend size and resizes when it differs from the
// last known area. Fixed viewports are not autoresized. Fullscreen and inline
// viewports are.
func (t *Terminal) Autoresize() error {
	if t.viewport == ViewportFixed {
		return nil
	}
	sz, err := t.backend.Size()
	if err != nil {
		return err
	}
	area := sizeToRect(sz)
	if area != t.lastKnownArea {
		return t.Resize(area)
	}
	return nil
}

// HideCursor hides the cursor via the backend.
func (t *Terminal) HideCursor() error {
	if err := t.backend.HideCursor(); err != nil {
		return err
	}
	t.hiddenCursor = true
	return nil
}

// ShowCursor shows the cursor via the backend.
func (t *Terminal) ShowCursor() error {
	if err := t.backend.ShowCursor(); err != nil {
		return err
	}
	t.hiddenCursor = false
	return nil
}

// GetCursorPosition queries the backend cursor.
func (t *Terminal) GetCursorPosition() (layout.Position, error) {
	return t.backend.GetCursorPosition()
}

// SetCursorPosition sets the backend cursor and updates internal tracking.
func (t *Terminal) SetCursorPosition(pos layout.Position) error {
	if err := t.backend.SetCursorPosition(pos); err != nil {
		return err
	}
	t.lastKnownCursorPos = pos
	return nil
}

// Close restores a hidden cursor. Safe to call multiple times.
//
// Matches Rust Terminal Drop behavior for cursor restoration. Does not close
// or release the underlying backend.
func (t *Terminal) Close() error {
	if t == nil || t.backend == nil {
		return nil
	}
	if t.hiddenCursor {
		return t.ShowCursor()
	}
	return nil
}

// InsertBefore inserts content above the current inline viewport.
//
// Non-inline viewports are a no-op. height < 0 is treated as 0. A nil draw
// callback is treated as a no-op draw.
//
// When the backend implements backend.ScrollingRegionBackend, uses the
// scrolling-regions fast path (including the full-screen-inline special case).
// Otherwise uses the portable fallback: temp full buffer, chunked direct
// backend draws, whole-screen scroll via AppendLines, viewport Y update, and
// a final Clear so the next Draw repaints the viewport.
func (t *Terminal) InsertBefore(height int, draw func(*buffer.Buffer)) error {
	if t.viewport != ViewportInline {
		return nil
	}
	if height < 0 {
		height = 0
	}
	if draw == nil {
		draw = func(*buffer.Buffer) {}
	}
	if sb, ok := t.backend.(backend.ScrollingRegionBackend); ok {
		return t.insertBeforeScrollingRegions(sb, height, draw)
	}
	return t.insertBeforeFallback(height, draw)
}

// insertBeforeFallback ports Rust insert_before_no_scrolling_regions.
func (t *Terminal) insertBeforeFallback(height int, draw func(*buffer.Buffer)) error {
	area := layout.Rect{
		X:      0,
		Y:      0,
		Width:  nonNeg(t.viewportArea.Width),
		Height: height,
	}
	buf := buffer.Empty(area)
	draw(buf)
	cells := buf.Content

	drawnHeight := t.viewportArea.Top()
	bufferHeight := height
	viewportHeight := t.viewportArea.Height
	screenHeight := t.lastKnownArea.Height
	if drawnHeight < 0 {
		drawnHeight = 0
	}
	if bufferHeight < 0 {
		bufferHeight = 0
	}
	if viewportHeight < 0 {
		viewportHeight = 0
	}
	if screenHeight < 0 {
		screenHeight = 0
	}

	// Drain oversized insertions in screen-sized chunks. Guard to_draw==0 so a
	// zero-height screen cannot spin forever.
	for bufferHeight+viewportHeight > screenHeight {
		toDraw := bufferHeight
		if screenHeight < toDraw {
			toDraw = screenHeight
		}
		if toDraw <= 0 {
			break
		}
		scrollUp := drawnHeight + toDraw - screenHeight
		if scrollUp < 0 {
			scrollUp = 0
		}
		if err := t.scrollUp(scrollUp); err != nil {
			return err
		}
		var err error
		cells, err = t.drawLines(drawnHeight-scrollUp, toDraw, cells)
		if err != nil {
			return err
		}
		drawnHeight += toDraw - scrollUp
		bufferHeight -= toDraw
	}

	scrollUp := drawnHeight + bufferHeight + viewportHeight - screenHeight
	if scrollUp < 0 {
		scrollUp = 0
	}
	if err := t.scrollUp(scrollUp); err != nil {
		return err
	}
	var err error
	cells, err = t.drawLines(drawnHeight-scrollUp, bufferHeight, cells)
	if err != nil {
		return err
	}
	_ = cells
	drawnHeight += bufferHeight - scrollUp
	if drawnHeight < 0 {
		drawnHeight = 0
	}

	next := t.viewportArea
	next.Y = drawnHeight
	t.setViewportArea(next)

	return t.Clear()
}

// insertBeforeScrollingRegions ports Rust insert_before_scrolling_regions.
func (t *Terminal) insertBeforeScrollingRegions(sb backend.ScrollingRegionBackend, height int, draw func(*buffer.Buffer)) error {
	area := layout.Rect{
		X:      0,
		Y:      0,
		Width:  nonNeg(t.viewportArea.Width),
		Height: height,
	}
	buf := buffer.Empty(area)
	draw(buf)
	cells := buf.Content

	// Full-screen inline: borrow the top line, scroll each inserted row into
	// scrollback, then restore the top line of the previous frame.
	if t.viewportArea.Height == t.lastKnownArea.Height {
		first := true
		for len(cells) > 0 {
			var err error
			if first {
				cells, err = t.drawLines(0, 1, cells)
			} else {
				cells, err = t.drawLinesOverCleared(0, 1, cells)
			}
			if err != nil {
				return err
			}
			first = false
			if err := sb.ScrollRegionUp(0, 1, 1); err != nil {
				return err
			}
			// width==0 would never shrink cells; bail to avoid a spin.
			if t.lastKnownArea.Width <= 0 {
				break
			}
		}

		width := t.viewportArea.Width
		if width < 0 {
			width = 0
		}
		prev := t.buffers[1-t.current]
		topLine := prev.Content
		if width > len(topLine) {
			width = len(topLine)
		}
		topLine = topLine[:width]
		_, err := t.drawLinesOverCleared(0, 1, topLine)
		return err
	}

	// Room below the viewport: scroll the viewport band down and fill the gap.
	{
		viewportTop := t.viewportArea.Top()
		viewportBottom := t.viewportArea.Bottom()
		screenBottom := t.lastKnownArea.Bottom()
		if viewportBottom < screenBottom && height > 0 {
			toDraw := height
			room := screenBottom - viewportBottom
			if room < toDraw {
				toDraw = room
			}
			if toDraw > 0 {
				if err := sb.ScrollRegionDown(viewportTop, viewportBottom+toDraw, toDraw); err != nil {
					return err
				}
				var err error
				cells, err = t.drawLinesOverCleared(viewportTop, toDraw, cells)
				if err != nil {
					return err
				}
				next := t.viewportArea
				next.Y = viewportTop + toDraw
				t.setViewportArea(next)
				height -= toDraw
			}
		}
	}

	viewportTop := t.viewportArea.Top()
	for height > 0 {
		toDraw := height
		if viewportTop < toDraw {
			toDraw = viewportTop
		}
		if toDraw <= 0 {
			break
		}
		if err := sb.ScrollRegionUp(0, viewportTop, toDraw); err != nil {
			return err
		}
		var err error
		cells, err = t.drawLinesOverCleared(viewportTop-toDraw, toDraw, cells)
		if err != nil {
			return err
		}
		_ = cells
		height -= toDraw
	}
	return nil
}

// drawLines writes lines_to_draw rows of cells at y_offset in screen coords.
// Returns the unused remainder of cells. Safe for zero width/height.
func (t *Terminal) drawLines(yOffset, linesToDraw int, cells []buffer.Cell) ([]buffer.Cell, error) {
	if linesToDraw < 0 {
		linesToDraw = 0
	}
	if yOffset < 0 {
		yOffset = 0
	}
	width := t.lastKnownArea.Width
	if width < 0 {
		width = 0
	}
	need := width * linesToDraw
	if need < 0 {
		need = 0
	}
	if need > len(cells) {
		need = len(cells)
	}
	toDraw := cells[:need]
	remainder := cells[need:]
	if linesToDraw > 0 && width > 0 && len(toDraw) > 0 {
		updates := make([]buffer.PositionedCell, len(toDraw))
		for i := range toDraw {
			updates[i] = buffer.PositionedCell{
				Position: layout.Position{
					X: i % width,
					Y: yOffset + i/width,
				},
				Cell: toDraw[i],
			}
		}
		if err := t.backend.Draw(updates); err != nil {
			return remainder, err
		}
		if err := t.backend.Flush(); err != nil {
			return remainder, err
		}
	}
	return remainder, nil
}

// drawLinesOverCleared is the scrolling-regions helper: the destination rows
// are already cleared, so every cell is written (equivalent to diffing against
// an empty buffer of the same area).
func (t *Terminal) drawLinesOverCleared(yOffset, linesToDraw int, cells []buffer.Cell) ([]buffer.Cell, error) {
	if linesToDraw < 0 {
		linesToDraw = 0
	}
	if yOffset < 0 {
		yOffset = 0
	}
	width := t.lastKnownArea.Width
	if width < 0 {
		width = 0
	}
	need := width * linesToDraw
	if need < 0 {
		need = 0
	}
	if need > len(cells) {
		need = len(cells)
	}
	toDraw := cells[:need]
	remainder := cells[need:]
	if linesToDraw <= 0 || width <= 0 || len(toDraw) == 0 {
		return remainder, nil
	}

	area := layout.Rect{X: 0, Y: yOffset, Width: width, Height: linesToDraw}
	previous := buffer.Empty(area)
	next := &buffer.Buffer{Area: area, Content: toDraw}
	if err := t.backend.Draw(buffer.Diff(previous, next)); err != nil {
		return remainder, err
	}
	if err := t.backend.Flush(); err != nil {
		return remainder, err
	}
	return remainder, nil
}

// scrollUp scrolls the whole screen up by lines by parking the cursor on the
// last row and calling AppendLines (Rust fallback path).
func (t *Terminal) scrollUp(lines int) error {
	if lines <= 0 {
		return nil
	}
	y := t.lastKnownArea.Height - 1
	if y < 0 {
		y = 0
	}
	if err := t.SetCursorPosition(layout.Position{X: 0, Y: y}); err != nil {
		return err
	}
	return t.backend.AppendLines(lines)
}

func (t *Terminal) clearViewport() error {
	switch t.viewport {
	case ViewportFixed:
		if err := t.clearFixedViewport(t.viewportArea); err != nil {
			return err
		}
	case ViewportInline:
		// Clear from the viewport origin through end of display, leaving
		// content above the inline band untouched.
		origin := layout.Position{X: t.viewportArea.X, Y: t.viewportArea.Y}
		if err := t.backend.SetCursorPosition(origin); err != nil {
			return err
		}
		if err := t.backend.ClearRegion(backend.AfterCursor); err != nil {
			return err
		}
	default:
		if err := t.backend.ClearRegion(backend.All); err != nil {
			return err
		}
	}
	// Reset the back buffer so the next update redraws everything.
	t.buffers[1-t.current].Reset()
	return nil
}

func (t *Terminal) clearFixedViewport(area layout.Rect) error {
	if area.IsEmpty() {
		return nil
	}
	sz, err := t.backend.Size()
	if err != nil {
		return err
	}
	isFullWidth := area.X == 0 && area.Width == sz.Width
	endsAtBottom := area.Bottom() == sz.Height
	if isFullWidth && endsAtBottom {
		if err := t.backend.SetCursorPosition(area.AsPosition()); err != nil {
			return err
		}
		return t.backend.ClearRegion(backend.AfterCursor)
	}
	if isFullWidth {
		return t.clearFullWidthRows(area)
	}
	return t.clearRegionCells(area)
}

func (t *Terminal) clearFullWidthRows(area layout.Rect) error {
	for y := area.Top(); y < area.Bottom(); y++ {
		if err := t.backend.SetCursorPosition(layout.Position{X: 0, Y: y}); err != nil {
			return err
		}
		if err := t.backend.ClearRegion(backend.CurrentLine); err != nil {
			return err
		}
	}
	return nil
}

func (t *Terminal) clearRegionCells(area layout.Rect) error {
	clearCell := buffer.NewCell()
	n := area.Area()
	if n < 0 {
		n = 0
	}
	updates := make([]buffer.PositionedCell, 0, n)
	for y := area.Y; y < area.Bottom(); y++ {
		for x := area.X; x < area.Right(); x++ {
			updates = append(updates, buffer.PositionedCell{
				Position: layout.Position{X: x, Y: y},
				Cell:     clearCell,
			})
		}
	}
	return t.backend.Draw(updates)
}

func (t *Terminal) setViewportArea(area layout.Rect) {
	t.buffers[t.current].Resize(area)
	t.buffers[1-t.current].Resize(area)
	t.viewportArea = area
}

// Frame is a consistent view into terminal state for one render pass.
type Frame struct {
	// cursorPosition is set by SetCursorPosition; nil means hide cursor.
	cursorPosition *layout.Position
	viewportArea   layout.Rect
	buffer         *buffer.Buffer
	count          int
}

// Area returns the viewport rectangle for this frame.
func (f *Frame) Area() layout.Rect { return f.viewportArea }

// Buffer returns the frame's buffer for direct cell access.
func (f *Frame) Buffer() *buffer.Buffer { return f.buffer }

// Count returns the frame sequence number (frames completed before this one).
func (f *Frame) Count() int { return f.count }

// RenderWidget renders w into area of the frame buffer.
func (f *Frame) RenderWidget(w widget.Widget, area layout.Rect) {
	if w == nil || f.buffer == nil {
		return
	}
	w.Render(area, f.buffer)
}

// RenderStatefulWidget renders via a callback that closes over widget state.
//
// Frame itself stays non-generic; use the package-level RenderStateful helper
// when you have a StatefulWidget value.
func (f *Frame) RenderStatefulWidget(render func(area layout.Rect, buf *buffer.Buffer), area layout.Rect) {
	if render == nil || f.buffer == nil {
		return
	}
	render(area, f.buffer)
}

// RenderStateful renders a StatefulWidget with the given state pointer.
func RenderStateful[S any](f *Frame, w widget.StatefulWidget[S], area layout.Rect, state *S) {
	if f == nil || w == nil || f.buffer == nil {
		return
	}
	w.RenderStateful(area, f.buffer, state)
}

// SetCursorPosition requests that the cursor be shown at pos after this frame
// is applied. If never called, Draw hides the cursor.
func (f *Frame) SetCursorPosition(pos layout.Position) {
	p := pos
	f.cursorPosition = &p
}

// diffBuffers returns cells in next that differ from previous.
func diffBuffers(previous, next *buffer.Buffer) []buffer.PositionedCell {
	if previous == nil && next == nil {
		return nil
	}
	return buffer.Diff(previous, next)
}

func sizeToRect(sz layout.Size) layout.Rect {
	w, h := sz.Width, sz.Height
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return layout.Rect{X: 0, Y: 0, Width: w, Height: h}
}

func clampRect(r layout.Rect) layout.Rect {
	if r.Width < 0 {
		r.Width = 0
	}
	if r.Height < 0 {
		r.Height = 0
	}
	return r
}

func nonNeg(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// computeInlineSize translates ViewportInline into a concrete Rect.
//
// The viewport always starts at column 0, spans the full terminal width, and is
// anchored to the backend cursor row. Requested height is clamped to the
// terminal height. AppendLines reserves vertical space; if the cursor is near
// the bottom edge and the terminal scrolls, the origin shifts upward so the
// band stays fully visible.
//
// offsetInPreviousViewport keeps the cursor at the same relative row across
// resizes (0 at construction).
//
// Returns the viewport area and the cursor position observed at the start.
// Zero height / zero size terminals are safe and yield an empty rect.
func computeInlineSize(b backend.Backend, height int, size layout.Size, offsetInPreviousViewport int) (layout.Rect, layout.Position, error) {
	if height < 0 {
		height = 0
	}
	if offsetInPreviousViewport < 0 {
		offsetInPreviousViewport = 0
	}
	w, h := size.Width, size.Height
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}

	pos, err := b.GetCursorPosition()
	if err != nil {
		return layout.Rect{}, layout.Position{}, err
	}
	row := pos.Y
	if row < 0 {
		row = 0
	}

	maxHeight := height
	if h < maxHeight {
		maxHeight = h
	}
	if maxHeight < 0 {
		maxHeight = 0
	}

	// Lines to append below the cursor so the band fits.
	// height - offset - 1, saturating at 0.
	linesAfterCursor := height - offsetInPreviousViewport - 1
	if linesAfterCursor < 0 {
		linesAfterCursor = 0
	}

	if err := b.AppendLines(linesAfterCursor); err != nil {
		return layout.Rect{}, layout.Position{}, err
	}

	// If append would have scrolled past the bottom, shift origin up.
	availableLines := h - row - 1
	if availableLines < 0 {
		availableLines = 0
	}
	missingLines := linesAfterCursor - availableLines
	if missingLines > 0 {
		row -= missingLines
		if row < 0 {
			row = 0
		}
	}
	row -= offsetInPreviousViewport
	if row < 0 {
		row = 0
	}

	return layout.Rect{
		X:      0,
		Y:      row,
		Width:  w,
		Height: maxHeight,
	}, pos, nil
}
