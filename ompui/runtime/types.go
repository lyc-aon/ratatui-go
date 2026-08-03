package runtime

import (
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
)

// Appearance is the terminal-reported light/dark mode from OSC 11 luminance.
type Appearance uint8

const (
	// AppearanceUnknown means no OSC 11 reply has been observed yet.
	AppearanceUnknown Appearance = iota
	// AppearanceDark is a dark background (BT.601 luminance < 0.5).
	AppearanceDark
	// AppearanceLight is a light background.
	AppearanceLight
)

// String returns a stable appearance name.
func (a Appearance) String() string {
	switch a {
	case AppearanceDark:
		return "dark"
	case AppearanceLight:
		return "light"
	default:
		return "unknown"
	}
}

// MouseMode is a Hermes-compatible DEC mouse tracking preset. Each preset
// includes SGR encoding and differs only in the motion reports it requests.
type MouseMode string

const (
	MouseOff     MouseMode = "off"
	MouseWheel   MouseMode = "wheel"
	MouseButtons MouseMode = "buttons"
	MouseAll     MouseMode = "all"
)

// Valid reports whether m names a supported mouse tracking preset.
func (m MouseMode) Valid() bool {
	switch m {
	case MouseOff, MouseWheel, MouseButtons, MouseAll:
		return true
	default:
		return false
	}
}

// CursorPosition is a 0-based cursor coordinate from CPR (CSI 6 n).
type CursorPosition struct {
	// Col is 0-based column.
	Col int
	// Row is 0-based row.
	Row int
}

// Notification is a structured desktop / bell notification.
// Rich fields are honored only after OSC 99 support is confirmed; otherwise
// the payload collapses to a single title:body line (or bare bell).
type Notification struct {
	Title     string
	Body      string
	ID        string
	Type      []string
	Urgency   Urgency
	IconName  string
	Sound     string
	Actions   NotifyActions
	ExpiresMs int // 0 = omit; negative allowed (Kitty w=)
}

// Urgency is OSC 99 urgency.
type Urgency uint8

const (
	UrgencyUnspecified Urgency = iota
	UrgencyLow
	UrgencyNormal
	UrgencyCritical
)

// NotifyActions selects OSC 99 action bits.
type NotifyActions uint8

const (
	NotifyActionsDefault NotifyActions = iota
	NotifyActionsFocus
	NotifyActionsReport
	NotifyActionsFocusReport
	NotifyActionsNone
)

// Options configures Terminal construction. Env and Platform must be explicit
// for deterministic capability resolution; zero values get safe defaults.
type Options struct {
	// Env is the environment map used for capability and session identity.
	// Nil means empty (no process env leak into pure tests).
	Env termcaps.Env

	// Platform is the GOOS string ("darwin", "linux", "windows", …).
	// Empty defaults to runtime.GOOS via termcaps.DefaultPlatform at New time
	// only when UseProcessPlatform is true; otherwise "unknown".
	Platform string

	// UseProcessPlatform copies termcaps.DefaultPlatform() when Platform is empty.
	UseProcessPlatform bool

	// EnterAltScreen enters the alternate screen during Start.
	// Default false — normal screen is the OMP default.
	EnterAltScreen bool

	// EventBuffer is the Events channel capacity (default 256).
	// Full channel applies backpressure on the input loop; events are never dropped.
	EventBuffer int

	// CPRTimeout bounds QueryCursorPosition (default 200ms).
	CPRTimeout time.Duration

	// KittyFallbackTimeout is when modifyOtherKeys engages if no Kitty reply
	// (default 150ms).
	KittyFallbackTimeout time.Duration

	// Osc11PollInterval is the background OSC 11 poll for terminals without
	// Mode 2031 (default 30s). Negative disables polling. WSL always skips the poll.
	Osc11PollInterval time.Duration

	// DisableProbes skips Kitty/OSC11/OSC99/DECRQM probes (tests / headless).
	DisableProbes bool

	// Headless suppresses raw mode, probes, writes, and signal watchers.
	// Start becomes a no-op side-effect path that still opens Events.
	Headless bool

	// WindowsTerminal enables the input decoder's raw-backspace heuristic.
	// When false, derived from Env (WT_SESSION / Windows Terminal markers).
	WindowsTerminal bool

	// ForceWindowsTerminal sets WindowsTerminal explicitly when true.
	ForceWindowsTerminal bool

	// TTYPath overrides device-path discovery for TTYID (tests).
	TTYPath string

	// WriteLog, if non-nil, receives a copy of every successful Write payload.
	WriteLog func([]byte)
}

func (o Options) withDefaults() Options {
	out := o
	if out.EventBuffer <= 0 {
		out.EventBuffer = defaultEventBuffer
	}
	if out.CPRTimeout <= 0 {
		out.CPRTimeout = time.Duration(defaultCPRTimeoutMs) * time.Millisecond
	}
	if out.KittyFallbackTimeout <= 0 {
		out.KittyFallbackTimeout = time.Duration(kittyFallbackTimeoutMs) * time.Millisecond
	}
	// Osc11PollInterval: zero → default 30s; negative → disabled (stay negative).
	if out.Osc11PollInterval == 0 {
		out.Osc11PollInterval = time.Duration(osc11PollIntervalMs) * time.Millisecond
	}
	if out.Env == nil {
		out.Env = termcaps.Env{}
	}
	if out.Platform == "" {
		if out.UseProcessPlatform {
			out.Platform = termcaps.DefaultPlatform()
		} else {
			out.Platform = "unknown"
		}
	}
	if out.ForceWindowsTerminal {
		out.WindowsTerminal = true
	} else if !out.WindowsTerminal {
		out.WindowsTerminal = out.Env.Has("WT_SESSION") || out.Platform == "windows"
	}
	return out
}
