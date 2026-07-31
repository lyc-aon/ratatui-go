package component

import "github.com/lyc-aon/ratatui-go/ompui/event"

// Remote is a leaf component whose rows are supplied by a protocol peer
// (TypeScript / Bun). Go owns width notification and input routing; the peer
// pushes cached line frames and invalidations over the wire.
//
// Typical bridge flow:
//  1. Host calls Render(width); Remote returns the last applied frame and
//     records the width so the bridge can send a resize if it changed.
//  2. Peer sends a new line frame; bridge calls SetLines / SetFrame.
//  3. Host routes focus input via HandleInput; Remote forwards to OnInput
//     (bridge encodes Raw/semantic event to the peer).
//
// Remote is safe for single-threaded UI use (same model as Container).
type Remote struct {
	// id is an opaque session/component id for the protocol bridge.
	id string

	frame Frame
	gen   Gen

	lastWidth int
	widthSeen bool

	// OnInput is invoked from HandleInput when non-nil.
	OnInput func(ev event.Event)

	// OnWidth is invoked from Render when the width changes and OnWidth != nil.
	OnWidth func(width int)

	// WantsReleases, when true, opts into key-release delivery.
	WantsReleases bool

	FocusState

	disposed bool

	// committed is the last SetNativeScrollbackCommittedRows claim
	// (informational for the bridge via CommittedRows).
	committed int
}

// NewRemote constructs a remote leaf with the given protocol id.
// The initial frame is empty with generation 0.
func NewRemote(id string) *Remote {
	return &Remote{
		id:    id,
		frame: EmptyFrame(0),
	}
}

// ID returns the protocol component id.
func (r *Remote) ID() string {
	if r == nil {
		return ""
	}
	return r.id
}

// LastWidth returns the width most recently passed to Render.
// ok is false until Render has been called at least once.
func (r *Remote) LastWidth() (width int, ok bool) {
	if r == nil {
		return 0, false
	}
	return r.lastWidth, r.widthSeen
}

// CommittedRows returns the last native-scrollback committed-row claim.
func (r *Remote) CommittedRows() int {
	if r == nil {
		return 0
	}
	return r.committed
}

// Frame returns the current cached frame without rendering.
// The Lines slice is component-owned; callers must not mutate it.
func (r *Remote) Frame() Frame {
	if r == nil {
		return EmptyFrame(0)
	}
	return r.frame
}

// Generation returns the current frame generation.
func (r *Remote) Generation() uint64 {
	if r == nil {
		return 0
	}
	return r.frame.Generation
}

// SetLines replaces the cached rows and bumps generation.
//
// lines is cloned (shallow) so the caller may reuse its buffer. A nil lines
// becomes an empty frame. Seams and cursor are cleared.
func (r *Remote) SetLines(lines []string) {
	if r == nil || r.disposed {
		return
	}
	cloned := CloneLines(lines)
	gen := r.gen.Next()
	r.frame = NewFrame(cloned, gen)
}

// SetFrame replaces the entire cached frame.
//
// Lines are cloned. Generation is bumped (the Generation field on src is
// ignored so the local counter stays monotonic for Container memoization).
// Seams and cursor are taken from src (cursor is copied).
func (r *Remote) SetFrame(src Frame) {
	if r == nil || r.disposed {
		return
	}
	cloned := CloneLines(src.Lines)
	gen := r.gen.Next()
	var cursor *Cursor
	if src.Cursor != nil {
		cc := *src.Cursor
		cursor = &cc
	}
	r.frame = Frame{
		Lines:           cloned,
		Generation:      gen,
		Cursor:          cursor,
		LiveRegionStart: src.LiveRegionStart,
		CommitSafeEnd:   src.CommitSafeEnd,
		SnapshotSafeEnd: src.SnapshotSafeEnd,
	}
}

// Apply updates the cached frame from protocol fields in one shot.
//
// lines is cloned. Generation is always bumped locally so Container
// memoization never sees a peer rewind. Pass BoundaryUnset for any seam that
// should clear. cursor may be nil.
func (r *Remote) Apply(lines []string, cursor *Cursor, liveStart, commitEnd, snapshotEnd int) {
	if r == nil || r.disposed {
		return
	}
	cloned := CloneLines(lines)
	gen := r.gen.Next()
	var c *Cursor
	if cursor != nil {
		cc := *cursor
		c = &cc
	}
	r.frame = Frame{
		Lines:           cloned,
		Generation:      gen,
		Cursor:          c,
		LiveRegionStart: liveStart,
		CommitSafeEnd:   commitEnd,
		SnapshotSafeEnd: snapshotEnd,
	}
}

// Clear drops cached rows to empty and bumps generation.
func (r *Remote) Clear() {
	if r == nil || r.disposed {
		return
	}
	r.frame = EmptyFrame(r.gen.Next())
}

// Render implements Component.
//
// Returns the cached frame. Records width and notifies OnWidth on change.
// Does not reflow locally — the peer is responsible for width-correct rows.
// width is clamped to at least 1 for the notification path; the cached lines
// are returned as-is even when empty (zero-row / empty peer content preserved).
func (r *Remote) Render(width int) Frame {
	if r == nil {
		return EmptyFrame(0)
	}
	if r.disposed {
		return EmptyFrame(r.frame.Generation)
	}
	if width < 1 {
		width = 1
	}
	if !r.widthSeen || r.lastWidth != width {
		r.lastWidth = width
		r.widthSeen = true
		if r.OnWidth != nil {
			r.OnWidth(width)
		}
	}
	// Return cached frame by value; Lines alias is intentional (immutable contract).
	return r.frame
}

// HandleInput implements InputHandler. Forwards to OnInput when set and not disposed.
func (r *Remote) HandleInput(ev event.Event) {
	if r == nil || r.disposed || r.OnInput == nil {
		return
	}
	r.OnInput(ev)
}

// WantsKeyRelease implements KeyReleaseInterest.
func (r *Remote) WantsKeyRelease() bool {
	if r == nil {
		return false
	}
	return r.WantsReleases
}

// Invalidate implements Invalidator.
//
// Drops cached lines to empty and bumps generation so the next compose cannot
// reuse stale peer rows. Width memory is kept so the bridge can re-request.
func (r *Remote) Invalidate() {
	if r == nil || r.disposed {
		return
	}
	r.frame = EmptyFrame(r.gen.Next())
}

// SetNativeScrollbackCommittedRows implements CommittedRowsAware.
func (r *Remote) SetNativeScrollbackCommittedRows(rows int) {
	if r == nil {
		return
	}
	if rows < 0 {
		rows = 0
	}
	r.committed = rows
}

// Dispose implements Disposable. Idempotent. Clears callbacks and cache.
func (r *Remote) Dispose() {
	if r == nil || r.disposed {
		return
	}
	r.disposed = true
	r.OnInput = nil
	r.OnWidth = nil
	r.frame = EmptyFrame(r.gen.Next())
	r.FocusState.SetFocused(false)
}

// Disposed reports whether Dispose has been called.
func (r *Remote) Disposed() bool {
	if r == nil {
		return true
	}
	return r.disposed
}

// RenderViewportTail implements ViewportTailProvider against the cached lines.
// Does not notify OnWidth and does not mutate full-compose state beyond a
// pure slice of the cache (safe for resize-drag).
func (r *Remote) RenderViewportTail(width, maxRows int) []string {
	if r == nil || r.disposed || maxRows <= 0 {
		return emptyLines
	}
	_ = width // peer owns reflow; tail is of the cached frame at last applied width
	lines := r.frame.Lines
	n := len(lines)
	if n == 0 {
		return emptyLines
	}
	if maxRows >= n {
		// Return the cached slice directly — immutable contract, no copy.
		return lines
	}
	// Bottom maxRows: new slice header into the same array (no element copy).
	return lines[n-maxRows:]
}
