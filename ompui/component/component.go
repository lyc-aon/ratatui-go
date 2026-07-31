package component

import "github.com/michaelkelly/ratatui-go/ompui/event"

// Component is the render contract every UI node implements.
//
// Render produces a [Frame] for the given terminal width in cells. Width is
// clamped to at least 1 by retained containers; leaf components may also clamp.
// See package docs for Lines ownership and Generation rules.
type Component interface {
	Render(width int) Frame
}

// InputHandler receives decoded terminal input when the component has focus.
//
// Events carry semantic payloads plus Raw bytes for remote/protocol bridges.
// Key-release events are filtered by the router unless the component also
// implements [KeyReleaseInterest] and returns true.
type InputHandler interface {
	HandleInput(ev event.Event)
}

// KeyReleaseInterest opts a component into Kitty key-release delivery.
// Default when absent: releases are dropped.
type KeyReleaseInterest interface {
	WantsKeyRelease() bool
}

// Focusable is a component that can own keyboard focus and drive a cursor.
//
// When Focused is true the component should expose a cursor via Frame.Cursor
// and/or embed [CursorMarker] in its rendered rows.
type Focusable interface {
	Focused() bool
	SetFocused(focused bool)
}

// TerminalCursorAware receives the host's hardware-cursor preference.
// TUI keeps this in sync when focus or the preference changes.
type TerminalCursorAware interface {
	SetUseTerminalCursor(use bool)
}

// StablePrefix is an opt-in stability report for components that mutate their
// returned Lines in place across frames (instead of allocating a fresh slice
// per change).
//
// The engine reads RenderStablePrefixRows right after Render returns. The
// report counts leading rows of the just-returned Lines that are byte-identical
// to the array state the reader last observed.
//
// Contract:
//   - Reading CONSUMES the report: it re-bases the baseline to the current
//     Lines state. The accumulated count covers every render since the previous
//     read, so out-of-band Render calls between engine frames can only lower
//     the report, never inflate it past what the engine actually has.
//   - An implementer that cannot prove stability for a frame must report 0.
//   - Rows at or beyond the report may have been mutated; rows before it must
//     be the identical string values at the identical indices.
type StablePrefix interface {
	RenderStablePrefixRows() int
}

// ViewportTailProvider is an opt-in fast path for composing only the visible
// tail of a tall component during a terminal resize drag.
//
// Returns the BOTTOM rows of the component's full render at width, in
// top-to-bottom order, capped at maxRows (fewer when shorter). Rows MUST be
// byte-identical to the corresponding tail of Render(width), modulo a
// one-row separator at the very top edge. MUST NOT mutate persistent
// full-compose state: the next Render (settle paint) reconciles as if the tail
// render never happened. Warming pure per-width caches is fine.
type ViewportTailProvider interface {
	RenderViewportTail(width, maxRows int) []string
}

// Disposable tears down timers, subscriptions, and other retained resources.
// Called when the component is permanently removed from the live tree.
// Must be idempotent. Containers propagate Dispose to children.
type Disposable interface {
	Dispose()
}

// Invalidator drops cached rendering state so the next Render rebuilds from
// scratch (theme changes, hard refresh). Containers propagate to children.
type Invalidator interface {
	Invalidate()
}

// OverlayFocusOwner lets an overlay root declare focus targets it owns beyond
// itself (nested widgets inside the overlay).
type OverlayFocusOwner interface {
	OwnsOverlayFocusTarget(component Component) bool
}

// TightLayoutAware receives the host tight-layout flag. Containers propagate
// it to children when set.
type TightLayoutAware interface {
	SetIgnoreTight(ignore bool)
}

// CommittedRowsAware receives how many of this component's local rows the
// engine has already committed to native scrollback (from the previous frame).
// Called immediately before Render so the component can skip re-deriving
// immutable history. rows is clamped to >= 0 by the caller.
type CommittedRowsAware interface {
	SetNativeScrollbackCommittedRows(rows int)
}

// IsFocusable reports whether c implements Focusable.
func IsFocusable(c Component) (Focusable, bool) {
	if c == nil {
		return nil, false
	}
	f, ok := c.(Focusable)
	return f, ok
}

// IsInputHandler reports whether c implements InputHandler.
func IsInputHandler(c Component) (InputHandler, bool) {
	if c == nil {
		return nil, false
	}
	h, ok := c.(InputHandler)
	return h, ok
}

// IsOverlayFocusTarget reports whether component is owner or an owned focus
// target of owner (when owner implements OverlayFocusOwner).
func IsOverlayFocusTarget(owner Component, component Component) bool {
	if owner == nil {
		return false
	}
	if component == owner {
		return true
	}
	if component == nil {
		return false
	}
	if o, ok := owner.(OverlayFocusOwner); ok {
		return o.OwnsOverlayFocusTarget(component)
	}
	return false
}

// RouteInput delivers ev to focused when it implements InputHandler.
//
// Key-release events are dropped unless focused implements KeyReleaseInterest
// and WantsKeyRelease returns true. Returns true when HandleInput was called.
func RouteInput(focused Component, ev event.Event) bool {
	if focused == nil {
		return false
	}
	if ev.Kind == event.KindKey && ev.Key.Action == event.ActionRelease {
		w, ok := focused.(KeyReleaseInterest)
		if !ok || !w.WantsKeyRelease() {
			return false
		}
	}
	h, ok := focused.(InputHandler)
	if !ok {
		return false
	}
	h.HandleInput(ev)
	return true
}

// AsViewportTailProvider returns the ViewportTailProvider if implemented.
func AsViewportTailProvider(c Component) (ViewportTailProvider, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.(ViewportTailProvider)
	return v, ok
}

// StablePrefixRows reads and consumes a StablePrefix report when implemented.
// Returns (0, false) when c does not implement StablePrefix.
func StablePrefixRows(c Component) (int, bool) {
	if c == nil {
		return 0, false
	}
	s, ok := c.(StablePrefix)
	if !ok {
		return 0, false
	}
	return s.RenderStablePrefixRows(), true
}

// NotifyCommittedRows calls SetNativeScrollbackCommittedRows when implemented.
func NotifyCommittedRows(c Component, rows int) {
	if c == nil {
		return
	}
	if rows < 0 {
		rows = 0
	}
	if a, ok := c.(CommittedRowsAware); ok {
		a.SetNativeScrollbackCommittedRows(rows)
	}
}

// InvalidateOne calls Invalidate when implemented.
func InvalidateOne(c Component) {
	if c == nil {
		return
	}
	if inv, ok := c.(Invalidator); ok {
		inv.Invalidate()
	}
}

// DisposeOne calls Dispose when implemented.
func DisposeOne(c Component) {
	if c == nil {
		return
	}
	if d, ok := c.(Disposable); ok {
		d.Dispose()
	}
}
