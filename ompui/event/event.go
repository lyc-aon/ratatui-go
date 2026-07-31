// Package event defines semantic terminal input event value types.
//
// Event is one value struct with Kind plus Key/Mouse/Text/Size/Raw fields.
// Decoders preserve Raw bytes for remote TypeScript components.
package event

// Kind classifies an Event.
type Kind uint8

const (
	// KindNone is the zero value; unused by decoders.
	KindNone Kind = iota
	// KindKey is a keyboard event (legacy, CSI-u, modifyOtherKeys, SS3).
	KindKey
	// KindText is printable text that is not a named/modified key binding.
	KindText
	// KindPaste is a complete bracketed-paste payload.
	KindPaste
	// KindMouse is an SGR or X10 mouse report.
	KindMouse
	// KindResize is a terminal size change (OS or in-band).
	KindResize
	// KindFocus is a focus-in / focus-out report.
	KindFocus
	// KindRaw is an unrecognized complete sequence delivered intact.
	KindRaw
	// KindError is a decoder or stream error.
	KindError
)

// String returns a stable kind name.
func (k Kind) String() string {
	switch k {
	case KindKey:
		return "key"
	case KindText:
		return "text"
	case KindPaste:
		return "paste"
	case KindMouse:
		return "mouse"
	case KindResize:
		return "resize"
	case KindFocus:
		return "focus"
	case KindRaw:
		return "raw"
	case KindError:
		return "error"
	default:
		return "none"
	}
}

// Modifiers is a bitset of key modifiers (Kitty keyboard protocol layout).
// Bits match the wire encoding after subtracting 1 from the CSI modifier field:
//
//	Shift=1, Alt=2, Ctrl=4, Super=8, CapsLock=64, NumLock=128.
type Modifiers uint16

const (
	ModShift    Modifiers = 1
	ModAlt      Modifiers = 2
	ModCtrl     Modifiers = 4
	ModSuper    Modifiers = 8
	ModHyper    Modifiers = 16
	ModMeta     Modifiers = 32
	ModCapsLock Modifiers = 64
	ModNumLock  Modifiers = 128
)

// LockMask is CapsLock|NumLock; stripped when matching binding modifiers.
const LockMask = ModCapsLock | ModNumLock

// Contains reports whether all bits in m are set.
func (mods Modifiers) Contains(m Modifiers) bool { return mods&m == m }

// WithoutLocks returns mods with CapsLock and NumLock cleared.
func (mods Modifiers) WithoutLocks() Modifiers { return mods &^ LockMask }

// Action is the Kitty keyboard event type (protocol flag 2).
type Action uint8

const (
	// ActionPress is a key-down (default when event type is absent).
	ActionPress Action = 1
	// ActionRepeat is an auto-repeat while held.
	ActionRepeat Action = 2
	// ActionRelease is a key-up.
	ActionRelease Action = 3
)

// String returns a stable action name.
func (a Action) String() string {
	switch a {
	case ActionPress:
		return "press"
	case ActionRepeat:
		return "repeat"
	case ActionRelease:
		return "release"
	default:
		return "unknown"
	}
}

// Code identifies a physical / logical key independent of modifiers.
// Named keys use well-known string IDs ("escape", "enter", "f1", "up", …).
// Printable keys use their single-character base form ("a", "1", "[", …).
type Code string

// Well-known named key codes.
const (
	CodeEscape    Code = "escape"
	CodeEnter     Code = "enter"
	CodeTab       Code = "tab"
	CodeBackspace Code = "backspace"
	CodeDelete    Code = "delete"
	CodeInsert    Code = "insert"
	CodeClear     Code = "clear"
	CodeHome      Code = "home"
	CodeEnd       Code = "end"
	CodePageUp    Code = "pageUp"
	CodePageDown  Code = "pageDown"
	CodeUp        Code = "up"
	CodeDown      Code = "down"
	CodeLeft      Code = "left"
	CodeRight     Code = "right"
	CodeSpace     Code = "space"
	CodeF1        Code = "f1"
	CodeF2        Code = "f2"
	CodeF3        Code = "f3"
	CodeF4        Code = "f4"
	CodeF5        Code = "f5"
	CodeF6        Code = "f6"
	CodeF7        Code = "f7"
	CodeF8        Code = "f8"
	CodeF9        Code = "f9"
	CodeF10       Code = "f10"
	CodeF11       Code = "f11"
	CodeF12       Code = "f12"
)

// Key is a decoded keyboard event.
//
// ID is the canonical binding identifier (e.g. "ctrl+shift+p", "escape").
// Code is the base key without modifiers. Text is the printable character
// produced by the key when it inserts text (empty for pure bindings).
// Raw is the original framed byte sequence.
type Key struct {
	// ID is the canonical key identifier used by bindings ("ctrl+c", "shift+tab").
	ID string
	// Code is the base key name without modifiers.
	Code Code
	// Mods is the modifier bitset (lock bits may be present; strip for matching).
	Mods Modifiers
	// Action is press, repeat, or release.
	Action Action
	// Text is the printable insertion text, if any.
	Text string
	// Codepoint is the primary Kitty/Unicode codepoint when known; 0 otherwise.
	Codepoint rune
	// ShiftedKey is the shifted codepoint from CSI-u alternate-key fields; 0 if absent.
	ShiftedKey rune
	// BaseLayoutKey is the PC-101 base layout codepoint; 0 if absent.
	BaseLayoutKey rune
}

// MouseButton encodes the low button bits plus motion/wheel flags from SGR reports.
type MouseButton int

// Common SGR button bit flags.
const (
	MouseButtonLeft   MouseButton = 0
	MouseButtonMiddle MouseButton = 1
	MouseButtonRight  MouseButton = 2
	MouseButtonMotion MouseButton = 32
	MouseButtonWheel  MouseButton = 64
)

// Mouse is a decoded SGR (or X10) mouse report.
// Col and Row are 0-based screen coordinates.
type Mouse struct {
	// Button is the raw SGR button code (bit 32 = motion, bit 64 = wheel).
	Button int
	// Col is the 0-based column.
	Col int
	// Row is the 0-based row.
	Row int
	// Release is true for an SGR release report ('m' terminator).
	Release bool
	// Motion is true when the pointer moved (hover/drag), not a click or wheel.
	Motion bool
	// LeftClick is true for a left-button press (not motion, release, or wheel).
	LeftClick bool
	// Wheel is -1 (up), 1 (down), or 0 when not a wheel event.
	Wheel int
	// Mods carries modifier bits encoded in the high button bits when present.
	// SGR encodes shift/alt/ctrl as button+4/+8/+16 respectively above the base.
	Mods Modifiers
}

// Size is a terminal geometry report.
type Size struct {
	// Cols is the width in cells.
	Cols int
	// Rows is the height in cells.
	Rows int
	// WidthPx is the width in pixels when known; 0 if unknown.
	WidthPx int
	// HeightPx is the height in pixels when known; 0 if unknown.
	HeightPx int
}

// Focus reports terminal focus gain/loss (CSI I / CSI O focus events).
type Focus struct {
	// Gained is true for focus-in, false for focus-out.
	Gained bool
}

// Event is one semantic input value.
//
// Exactly one of the payload fields is meaningful for a given Kind;
// other payload fields are zero. Raw always holds the framed bytes that
// produced the event, including bracketed-paste delimiters (empty for synthetic errors).
type Event struct {
	Kind Kind

	// Key is set for KindKey.
	Key Key
	// Text is set for KindText (and may mirror Key.Text for KindKey).
	Text string
	// Paste is set for KindPaste.
	Paste string
	// Mouse is set for KindMouse.
	Mouse Mouse
	// Size is set for KindResize.
	Size Size
	// Focus is set for KindFocus.
	Focus Focus
	// Raw is the original framed byte sequence, including paste delimiters.
	Raw []byte
	// Err is set for KindError.
	Err error
}

// KeyEvent constructs a KindKey event.
func KeyEvent(k Key, raw []byte) Event {
	return Event{Kind: KindKey, Key: k, Text: k.Text, Raw: raw}
}

// TextEvent constructs a KindText event.
func TextEvent(text string, raw []byte) Event {
	return Event{Kind: KindText, Text: text, Raw: raw}
}

// PasteEvent constructs a KindPaste event.
func PasteEvent(content string, raw []byte) Event {
	if raw == nil {
		raw = []byte(content)
	}
	return Event{Kind: KindPaste, Paste: content, Text: content, Raw: raw}
}

// MouseEvent constructs a KindMouse event.
func MouseEvent(m Mouse, raw []byte) Event {
	return Event{Kind: KindMouse, Mouse: m, Raw: raw}
}

// ResizeEvent constructs a KindResize event.
func ResizeEvent(s Size, raw []byte) Event {
	return Event{Kind: KindResize, Size: s, Raw: raw}
}

// FocusEvent constructs a KindFocus event.
func FocusEvent(gained bool, raw []byte) Event {
	return Event{Kind: KindFocus, Focus: Focus{Gained: gained}, Raw: raw}
}

// RawEvent constructs a KindRaw event for an unrecognized complete sequence.
func RawEvent(raw []byte) Event {
	return Event{Kind: KindRaw, Raw: raw}
}

// ErrorEvent constructs a KindError event.
func ErrorEvent(err error, raw []byte) Event {
	return Event{Kind: KindError, Err: err, Raw: raw}
}
