package renderer

// CSI / paint framing constants matching OMP tui.ts.
const (
	eraseToEndOfLine = "\x1b[K"
	eraseLine        = "\x1b[2K"
	eraseDisplay     = "\x1b[2J"
	eraseScrollback  = "\x1b[3J" // ED3 — single callsite only
	eraseScreenCopy  = "\x1b[22J" // kitty ED22 copy-screen-to-scrollback
	cursorHome       = "\x1b[H"
	hideCursor       = "\x1b[?25l"
	showCursor       = "\x1b[?25h"
	syncOutputBegin  = "\x1b[?2026h"
	syncOutputEnd    = "\x1b[?2026l"
	disableAutowrap  = "\x1b[?7l"
	enableAutowrap   = "\x1b[?7h"
	altScreenEnter   = "\x1b[?1049h"
	altScreenExit    = "\x1b[?1049l"
	mouseTrackingOn  = "\x1b[?1000h\x1b[?1003h\x1b[?1006h"
	mouseTrackingOff = "\x1b[?1006l\x1b[?1003l\x1b[?1000l"

	// Line-fit source clamp (OMP LINE_FIT_*).
	lineFitMinSourceCodeUnits     = 4096
	lineFitMaxSourceCodeUnits     = 65536
	lineFitSourceWidthMultiplier  = 64
)

// DefaultMinRenderIntervalMs is ordinary repaint cadence (~30fps).
const DefaultMinRenderIntervalMs = 1000.0 / 30.0

// DefaultMultiplexerResizeDebounceMs folds SIGWINCH bursts in mux panes.
const DefaultMultiplexerResizeDebounceMs = 50.0

// DefaultResizeViewportSettleMs ends a non-mux resize drag before full replay.
const DefaultResizeViewportSettleMs = 120.0
