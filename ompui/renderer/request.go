package renderer

import (
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/ledger"
)

// Reason classifies why a frame is being drawn. It drives force/full/resize
// paths without exposing a free-form ClearScrollback bool at every callsite.
type Reason uint8

const (
	// ReasonUpdate is an ordinary incremental frame (throttled by scheduler).
	ReasonUpdate Reason = iota
	// ReasonForce forces a full window rewrite (no scrollback clear by itself).
	ReasonForce
	// ReasonReplace is session replace / branch / resume — full paint + ED3
	// outside multiplexers.
	ReasonReplace
	// ReasonReset is Ctrl+L style display reset — full paint + ED3 outside mux.
	ReasonReset
	// ReasonResize marks a geometry change (including net-zero dimension events
	// that still reflowed the terminal buffer).
	ReasonResize
	// ReasonFlush bypasses the ordinary throttle (immediate scheduler path)
	// without implying force-window-rewrite or scrollback clear.
	ReasonFlush
)

// String returns a stable reason name for traces.
func (r Reason) String() string {
	switch r {
	case ReasonUpdate:
		return "update"
	case ReasonForce:
		return "force"
	case ReasonReplace:
		return "replace"
	case ReasonReset:
		return "reset"
	case ReasonResize:
		return "resize"
	case ReasonFlush:
		return "flush"
	default:
		return "unknown"
	}
}

// Request is one composed frame plus semantic metadata for the engine.
//
// Prefer setting Frame (component.Frame) — Lines, seams, and Cursor come from
// it zero-copy. The legacy Fields (Lines/LiveStart/…) remain as an override
// path when Frame is the zero value with nil Lines; when Frame.Lines != nil
// (including empty non-nil from component constructors), Frame wins.
//
// Committed prefix rows are immutable after a successful draw — callers must
// not mutate Frame.Lines[i] for i < CommittedRows.
type Request struct {
	// Frame is the preferred component contract input (lines + seams + cursor).
	Frame component.Frame

	// Width / Height are the terminal viewport dimensions in cells/rows.
	Width  int
	Height int

	// Lines is a fallback row slice used only when Frame.Lines is nil.
	// Prefer Request{Frame: component.NewFrame(...)}.
	Lines []string

	// LiveStart / CommitSafe / SnapshotSafe override Frame seams when SetSeams
	// is true. Otherwise seams come from Frame (BoundaryUnset = omitted).
	LiveStart    int
	CommitSafe   int
	SnapshotSafe int
	SetSeams     bool

	// StablePrefixRows is the compose-reported leading unchanged row count.
	// Gates the committed-prefix audit and prepared-line cache invalidation.
	// Callers typically take this from component.StablePrefixRows / Container.
	StablePrefixRows int

	// Overlays are composited into the window slice only (never the ledger).
	// A visible overlay freezes commits for the frame.
	Overlays []Overlay

	// Cursor overrides Frame.Cursor when non-nil. Markers embedded as
	// ansitext.CursorMarker in row strings are also honored; visible-window
	// markers win after overlay compose.
	Cursor *CursorPos

	// Reason drives force/full/resize classification.
	Reason Reason

	// ImageTransmit is optional Kitty/iTerm transmit sequences to flush before
	// placements in this frame (full paint places them after the clear).
	ImageTransmit string

	// ImagePurge is optional Kitty delete sequences for demoted images.
	ImagePurge string

	// Notify is an optional component told of the post-emit committed row count
	// via component.NotifyCommittedRows (Container fan-out).
	Notify component.Component

	// ResizeViewportOnly paints a throwaway alt-screen viewport during a
	// non-mux drag without advancing the ledger. Host sets this while a
	// resize settle timer is armed; authoritative rebuild uses ReasonResize.
	ResizeViewportOnly bool
}

// clearScrollbackIntent is true for replace/reset reasons.
func (r Request) clearScrollbackIntent() bool {
	return r.Reason == ReasonReplace || r.Reason == ReasonReset
}

// forceWindowIntent is true when the window must be fully rewritten.
func (r Request) forceWindowIntent() bool {
	return r.Reason == ReasonForce || r.Reason == ReasonReplace ||
		r.Reason == ReasonReset || r.Reason == ReasonResize
}

// resolvedRows returns the row slice without copying.
func (r Request) resolvedRows() []string {
	if r.Frame.Lines != nil {
		return r.Frame.Lines
	}
	if r.Lines != nil {
		return r.Lines
	}
	return []string{}
}

// resolvedSeams returns absolute seam indices for the ledger (Unset = omitted).
func (r Request) resolvedSeams() (live, commit, snapshot int) {
	if r.SetSeams {
		return r.LiveStart, r.CommitSafe, r.SnapshotSafe
	}
	// Frame seams: BoundaryUnset == ledger.Unset == -1.
	live = r.Frame.LiveRegionStart
	commit = r.Frame.CommitSafeEnd
	snapshot = r.Frame.SnapshotSafeEnd
	if live == 0 && commit == 0 && snapshot == 0 && r.Frame.Lines == nil && r.Lines != nil {
		// Zero Frame with legacy Lines only — shell semantics.
		return ledger.Unset, ledger.Unset, ledger.Unset
	}
	// Map component.BoundaryUnset to ledger.Unset (same value, explicit).
	if live == component.BoundaryUnset {
		live = ledger.Unset
	}
	if commit == component.BoundaryUnset {
		commit = ledger.Unset
	}
	if snapshot == component.BoundaryUnset {
		snapshot = ledger.Unset
	}
	return live, commit, snapshot
}

// resolvedCursor returns an explicit cursor from Request or Frame.
func (r Request) resolvedCursor() *CursorPos {
	if r.Cursor != nil {
		return r.Cursor
	}
	if r.Frame.Cursor != nil {
		return &CursorPos{Row: r.Frame.Cursor.Row, Col: r.Frame.Cursor.Column}
	}
	return nil
}

// RequestFromFrame builds a Request from a component.Frame and viewport size.
// Zero-copy on Lines. Seams and cursor come from the frame.
func RequestFromFrame(f component.Frame, width, height int, reason Reason) Request {
	return Request{
		Frame:  f,
		Width:  width,
		Height: height,
		Reason: reason,
	}
}
