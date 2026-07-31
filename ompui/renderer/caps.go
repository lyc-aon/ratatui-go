package renderer

import "github.com/lyc-aon/ratatui-go/ompui/termcaps"

// Caps are terminal capability decisions the emitter needs. The host supplies
// them (from termcaps probes / env); the engine never sniffs the process env
// itself and never probes scroll position.
type Caps struct {
	// SynchronizedOutput wraps paints in DEC 2026 when true.
	SynchronizedOutput bool

	// SupportsScreenToScrollback enables kitty ED22 on non-clearing full paints.
	SupportsScreenToScrollback bool

	// Multiplexer suppresses ED3 and forces resize-in-place semantics when set
	// via termcaps.ResizeRepaintsInPlace. Stored here so callers can pass a
	// frozen decision without re-reading env each frame.
	Multiplexer bool

	// ResizeRepaintsInPlace is true for mux panes and terminals that re-report
	// size on alt-screen toggle (Warp). Geometry frames then freeze commits and
	// rewrite the window instead of ED3+replay.
	ResizeRepaintsInPlace bool

	// ImageProtocol classifies image payload rows (bypass width-fit / coalesce).
	// Zero means no image protocol — IsImageLine always false.
	ImageProtocol termcaps.ImageProtocol

	// ShowHardwareCursor reveals the hardware cursor at the marker when true.
	// OMP defaults this off unless PI_HARDWARE_CURSOR is set; callers decide.
	ShowHardwareCursor bool
}

// CapsFromSnapshot builds Caps from a termcaps.Snapshot plus env used for the
// resize-in-place decision. snap may be nil (safe defaults: no sync, no images).
func CapsFromSnapshot(snap *termcaps.Snapshot, env termcaps.Env) Caps {
	c := Caps{}
	if snap != nil {
		c.SynchronizedOutput = snap.SynchronizedOutput
		c.SupportsScreenToScrollback = snap.SupportsScreenToScrollback
		c.Multiplexer = snap.Multiplexer
		c.ImageProtocol = snap.ImageProtocol
	}
	if env != nil {
		c.ResizeRepaintsInPlace = termcaps.ResizeRepaintsInPlace(env)
		if !c.Multiplexer {
			c.Multiplexer = termcaps.IsMultiplexerSession(env)
		}
	} else if snap != nil {
		c.ResizeRepaintsInPlace = snap.Multiplexer
	}
	return c
}

// IsImageLine reports whether line carries the configured image protocol.
func (c Caps) IsImageLine(line string) bool {
	if c.ImageProtocol == "" {
		return false
	}
	return termcaps.IsImageLine(line, c.ImageProtocol)
}

func (c Caps) paintBegin() string {
	if c.SynchronizedOutput {
		return hideCursor + syncOutputBegin + disableAutowrap
	}
	return hideCursor + disableAutowrap
}

func (c Caps) paintEnd() string {
	if c.SynchronizedOutput {
		return enableAutowrap + syncOutputEnd
	}
	return enableAutowrap
}

func (c Caps) cursorBegin() string {
	if c.SynchronizedOutput {
		return hideCursor + syncOutputBegin
	}
	return hideCursor
}

func (c Caps) cursorEnd() string {
	if c.SynchronizedOutput {
		return syncOutputEnd
	}
	return ""
}
