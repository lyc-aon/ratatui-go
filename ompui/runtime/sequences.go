package runtime

// Control sequences owned by the TTY lifecycle. Values match oh-my-pi
// packages/tui ProcessTerminal.

const (
	seqBracketedPasteEnable  = "\x1b[?2004h"
	seqBracketedPasteDisable = "\x1b[?2004l"

	seqKittyKeyboardQuery = "\x1b[?u"
	seqKittyPushLevel1    = "\x1b[>1u"
	seqKittyPushLevel7    = "\x1b[>7u"
	seqKittyPop           = "\x1b[<u"

	seqModifyOtherKeysEnable  = "\x1b[>4;2m"
	seqModifyOtherKeysDisable = "\x1b[>4;0m"

	seqDA1 = "\x1b[c"

	seqOSC11Query = "\x1b]11;?\x07"

	seqMode2031Enable  = "\x1b[?2031h"
	seqMode2031Disable = "\x1b[?2031l"

	seqSyncOutputDisable = "\x1b[?2026l"
	seqAutowrapEnable    = "\x1b[?7h"

	seqInBandResizeEnable  = "\x1b[?2048h"
	seqInBandResizeDisable = "\x1b[?2048l"

	seqEnhancedPasteDisable = "\x1b[?5522l"

	seqMouseSGREnable     = "\x1b[?1006h"
	seqMouseSGRDisable    = "\x1b[?1006l"
	seqMouseAnyEnable     = "\x1b[?1003h"
	seqMouseAnyDisable    = "\x1b[?1003l"
	seqMouseButtonEnable  = "\x1b[?1002h"
	seqMouseButtonDisable = "\x1b[?1002l"
	seqMouseBasicEnable   = "\x1b[?1000h"
	seqMouseBasicDisable  = "\x1b[?1000l"

	// Hermes presets always use SGR reports. Reset first on every mode change
	// so a transition cannot leave a more permissive tracking bit armed.
	seqMouseWheelEnable   = seqMouseBasicEnable + seqMouseSGREnable
	seqMouseButtonsEnable = seqMouseBasicEnable + seqMouseButtonEnable + seqMouseSGREnable
	seqMouseAllEnable     = seqMouseBasicEnable + seqMouseButtonEnable + seqMouseAnyEnable + seqMouseSGREnable
	seqMouseDisableAll    = seqMouseSGRDisable + seqMouseAnyDisable + seqMouseButtonDisable + seqMouseBasicDisable

	seqHideCursor = "\x1b[?25l"
	seqShowCursor = "\x1b[?25h"

	seqEnterAltScreen = "\x1b[?1049h"
	seqLeaveAltScreen = "\x1b[?1049l"

	seqCPR = "\x1b[6n"

	seqProgressActive = "\x1b]9;4;3\x07"
	seqProgressClear  = "\x1b]9;4;0;\x07"

	// Mode numbers probed via DECRQM.
	modeSyncOutput           = 2026
	modeInBandResize         = 2048
	modeAppearanceNotif      = 2031
	modeScrollBottomOnOutput = 1010
	modeScrollBottomOnKey    = 1011
)

var xtermScrollToBottomModes = [...]int{modeScrollBottomOnOutput, modeScrollBottomOnKey}

// Timing constants (ProcessTerminal / StdinBuffer).
const (
	progressKeepaliveInterval = 1000 // ms — set in types via time.Duration
	kittyFallbackTimeoutMs    = 150
	osc11PollIntervalMs       = 30_000
	mode2031DebounceMs        = 100
	defaultCPRTimeoutMs       = 200
	defaultDrainMaxMs         = 1000
	defaultDrainIdleMs        = 50
	defaultEventBuffer        = 256
	defaultWriteBuffer        = 64 * 1024
	maxConPTYWriteChunkBytes  = 16 * 1024
	maxProbeReassemblyBytes   = 256
)
