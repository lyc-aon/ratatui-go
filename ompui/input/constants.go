package input

import "time"

const (
	esc byte = 0x1b
	bel byte = 0x07

	// Bracketed paste markers (CSI 200~/201~).
	// pasteStart = "\x1b[200~", pasteEnd = "\x1b[201~"
	// Paste-mode recovery bounds: a lost/corrupted end marker must not hang
	// input forever or grow memory unboundedly.
	defaultPasteInactivity = 1000 * time.Millisecond
	defaultPasteMaxBytes   = 64 * 1024 * 1024

	// A buggy double-report (CSI-u event plus the bare printable for the same
	// keypress) arrives in the same terminal write; a bare char later than this
	// window is a real keystroke and must not be swallowed.
	kittyPrintableDedupWindow = 25 * time.Millisecond

	// Flush timeout for incomplete escape sequences (ProcessTerminal uses 50ms).
	defaultFlushTimeout = 50 * time.Millisecond

	// Upper bound on how long an unambiguous partial is held past the flush
	// timeout before being delivered raw (terminal died mid-sequence).
	defaultPartialHoldMax = 150 * time.Millisecond
)

var (
	pasteStart = []byte{0x1b, '[', '2', '0', '0', '~'}
	pasteEnd   = []byte{0x1b, '[', '2', '0', '1', '~'}
)

// Kitty keyboard modifier bits (after wire-value - 1).
const (
	kittyModShift   = 1
	kittyModAlt     = 2
	kittyModCtrl    = 4
	kittyModSuper   = 8
	kittyModNumLock = 128
	kittyLockMask   = 64 + kittyModNumLock
)

// Internal sentinel codepoints for CSI 1;mod <letter> forms (negative).
const (
	arrowUp    int32 = -1
	arrowDown  int32 = -2
	arrowRight int32 = -3
	arrowLeft  int32 = -4

	funcDelete   int32 = -10
	funcInsert   int32 = -11
	funcPageUp   int32 = -12
	funcPageDown int32 = -13
	funcHome     int32 = -14
	funcEnd      int32 = -15
	funcClear    int32 = -16

	funcF1  int32 = -20
	funcF2  int32 = -21
	funcF3  int32 = -22
	funcF4  int32 = -23
	funcF5  int32 = -24
	funcF6  int32 = -25
	funcF7  int32 = -26
	funcF8  int32 = -27
	funcF9  int32 = -28
	funcF10 int32 = -29
	funcF11 int32 = -30
	funcF12 int32 = -31

	cpEscape    int32 = 27
	cpTab       int32 = 9
	cpEnter     int32 = 13
	cpSpace     int32 = 32
	cpBackspace int32 = 127

	cpKP0        int32 = 57399
	cpKP1        int32 = 57400
	cpKP2        int32 = 57401
	cpKP3        int32 = 57402
	cpKP4        int32 = 57403
	cpKP5        int32 = 57404
	cpKP6        int32 = 57405
	cpKP7        int32 = 57406
	cpKP8        int32 = 57407
	cpKP9        int32 = 57408
	cpKPDecimal  int32 = 57409
	cpKPDivide   int32 = 57410
	cpKPMultiply int32 = 57411
	cpKPSubtract int32 = 57412
	cpKPAdd      int32 = 57413
	cpKPEnter    int32 = 57414
	cpKPEquals   int32 = 57415
)
