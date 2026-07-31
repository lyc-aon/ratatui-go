package ansitext

// Framing and marker constants matching OMP tui.ts.
const (
	// SegmentReset closes SGR attributes (CSI 0 m).
	SegmentReset = "\x1b[0m"

	// LineTerminator closes SGR and any open OSC 8 hyperlink so styles/links
	// cannot bleed across painted content rows in scrollback.
	LineTerminator = "\x1b[0m\x1b]8;;\x07"

	// CursorMarker is the zero-width APC the editor embeds so the renderer can
	// place the hardware cursor for IME candidate windows.
	CursorMarker = "\x1b_pi:c\x07"

	// DefaultTabWidth is the fixed display width of a tab, matching OMP
	// DEFAULT_TAB_WIDTH / pi-natives.
	DefaultTabWidth = 3
)

// Internal scan / coalesce limits.
const (
	esc = '\x1b'
	bel = '\x07'

	// mergeTokenCap is the max SGR parameter tokens per coalesced CSI.
	// Kept under xterm.js's 32-param cap so long adjacent runs split cleanly.
	mergeTokenCap = 16
)
