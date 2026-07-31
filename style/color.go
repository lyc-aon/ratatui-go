package style

import (
	"fmt"
	"strconv"
	"strings"
)

// ColorKind identifies which color model a Color uses.
type ColorKind uint8

// Color model kind constants.
const (
	// KindUnset is reserved; public Color values never use it (zero Color is Reset).
	// Style tracks unset via HasFG/HasBG/HasUnderlineColor instead.
	KindUnset ColorKind = iota
	// KindReset resets the terminal color to the default.
	KindReset
	// KindNamed is one of the 16 ANSI named colors (indices 0-15).
	KindNamed
	// KindIndexed is an 8-bit palette index (0-255).
	KindIndexed
	// KindRGB is a 24-bit truecolor value.
	KindRGB
)

// Named ANSI color identifiers stored in Color.index when Kind is KindNamed.
// Black..Gray = 0..7, DarkGray..White = 8..15 (standard ANSI palette).
const (
	namedIdxBlack uint8 = iota
	namedIdxRed
	namedIdxGreen
	namedIdxYellow
	namedIdxBlue
	namedIdxMagenta
	namedIdxCyan
	namedIdxGray
	namedIdxDarkGray
	namedIdxLightRed
	namedIdxLightGreen
	namedIdxLightYellow
	namedIdxLightBlue
	namedIdxLightMagenta
	namedIdxLightCyan
	namedIdxWhite
)

// Color is a comparable terminal color value.
//
// Zero value is treated as Reset for Kind()/IsReset(); Style tracks presence
// separately so unset stays distinct from Reset.
type Color struct {
	kind    ColorKind
	index   uint8 // named ANSI index (0-15) or palette index (0-255)
	r, g, b uint8
}

// Predeclared named / special colors.
var (
	// Reset restores the terminal default color. Distinct from an unset Style
	// color (Style uses HasFG/HasBG/HasUnderlineColor for presence).
	// Equal to the zero Color value so Color{} == Reset.
	Reset = Color{} // kind KindUnset, treated as KindReset by Kind()

	Black        = Color{kind: KindNamed, index: namedIdxBlack}
	Red          = Color{kind: KindNamed, index: namedIdxRed}
	Green        = Color{kind: KindNamed, index: namedIdxGreen}
	Yellow       = Color{kind: KindNamed, index: namedIdxYellow}
	Blue         = Color{kind: KindNamed, index: namedIdxBlue}
	Magenta      = Color{kind: KindNamed, index: namedIdxMagenta}
	Cyan         = Color{kind: KindNamed, index: namedIdxCyan}
	Gray         = Color{kind: KindNamed, index: namedIdxGray}
	DarkGray     = Color{kind: KindNamed, index: namedIdxDarkGray}
	LightRed     = Color{kind: KindNamed, index: namedIdxLightRed}
	LightGreen   = Color{kind: KindNamed, index: namedIdxLightGreen}
	LightYellow  = Color{kind: KindNamed, index: namedIdxLightYellow}
	LightBlue    = Color{kind: KindNamed, index: namedIdxLightBlue}
	LightMagenta = Color{kind: KindNamed, index: namedIdxLightMagenta}
	LightCyan    = Color{kind: KindNamed, index: namedIdxLightCyan}
	White        = Color{kind: KindNamed, index: namedIdxWhite}
)

// Indexed returns an 8-bit palette color.
func Indexed(index uint8) Color {
	return Color{kind: KindIndexed, index: index}
}

// RGB returns a 24-bit truecolor value.
func RGB(r, g, b uint8) Color {
	return Color{kind: KindRGB, r: r, g: g, b: b}
}

// Kind reports the color model. Zero Color is KindReset.
func (c Color) Kind() ColorKind {
	if c.kind == KindUnset {
		return KindReset
	}
	return c.kind
}

// IsReset reports whether c is the Reset color (including the zero value).
func (c Color) IsReset() bool {
	return c.Kind() == KindReset
}

// IsNamed reports whether c is one of the 16 ANSI named colors.
func (c Color) IsNamed() bool {
	return c.Kind() == KindNamed
}

// IsIndexed reports whether c is an 8-bit palette color.
func (c Color) IsIndexed() bool {
	return c.Kind() == KindIndexed
}

// IsRGB reports whether c is a 24-bit truecolor value.
func (c Color) IsRGB() bool {
	return c.Kind() == KindRGB
}

// Index returns the ANSI/palette index for KindNamed (0-15) and KindIndexed (0-255).
// For other kinds ok is false.
//
// Named mapping: Black..Gray 0..7, DarkGray..White 8..15.
// Backend uses Kind() to choose SGR 30/90 vs 38;5 forms.
func (c Color) Index() (uint8, bool) {
	switch c.Kind() {
	case KindNamed, KindIndexed:
		return c.index, true
	default:
		return 0, false
	}
}

// RGB returns the red/green/blue channels when c is RGB.
func (c Color) RGB() (r, g, b uint8, ok bool) {
	if c.Kind() != KindRGB {
		return 0, 0, 0, false
	}
	return c.r, c.g, c.b, true
}

// FromU32 builds an RGB color from 0x00RRGGBB.
func FromU32(u uint32) Color {
	return RGB(uint8(u>>16), uint8(u>>8), uint8(u))
}

// String renders a stable display form matching Ratatui's Display impl.
func (c Color) String() string {
	switch c.Kind() {
	case KindReset:
		return "Reset"
	case KindNamed:
		switch c.index {
		case namedIdxBlack:
			return "Black"
		case namedIdxRed:
			return "Red"
		case namedIdxGreen:
			return "Green"
		case namedIdxYellow:
			return "Yellow"
		case namedIdxBlue:
			return "Blue"
		case namedIdxMagenta:
			return "Magenta"
		case namedIdxCyan:
			return "Cyan"
		case namedIdxGray:
			return "Gray"
		case namedIdxDarkGray:
			return "DarkGray"
		case namedIdxLightRed:
			return "LightRed"
		case namedIdxLightGreen:
			return "LightGreen"
		case namedIdxLightYellow:
			return "LightYellow"
		case namedIdxLightBlue:
			return "LightBlue"
		case namedIdxLightMagenta:
			return "LightMagenta"
		case namedIdxLightCyan:
			return "LightCyan"
		case namedIdxWhite:
			return "White"
		default:
			return fmt.Sprintf("Named(%d)", c.index)
		}
	case KindIndexed:
		return strconv.FormatUint(uint64(c.index), 10)
	case KindRGB:
		return fmt.Sprintf("#%02X%02X%02X", c.r, c.g, c.b)
	default:
		return "Reset"
	}
}

// ParseColorError is returned when a color string cannot be parsed.
type ParseColorError struct {
	Value string
}

func (e ParseColorError) Error() string {
	if e.Value == "" {
		return "failed to parse color"
	}
	return "failed to parse color: " + e.Value
}

// ParseColor parses named colors, #RRGGBB hex, and decimal indexed colors.
//
// Accepts the same flexible named-color spellings as Ratatui (bright/light,
// grey/gray, silver, separators, etc.).
func ParseColor(s string) (Color, error) {
	normalized := strings.ToLower(s)
	normalized = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(normalized)
	normalized = strings.ReplaceAll(normalized, "bright", "light")
	normalized = strings.ReplaceAll(normalized, "grey", "gray")
	normalized = strings.ReplaceAll(normalized, "silver", "gray")
	normalized = strings.ReplaceAll(normalized, "lightblack", "darkgray")
	normalized = strings.ReplaceAll(normalized, "lightwhite", "white")
	normalized = strings.ReplaceAll(normalized, "lightgray", "white")

	switch normalized {
	case "reset":
		return Reset, nil
	case "black":
		return Black, nil
	case "red":
		return Red, nil
	case "green":
		return Green, nil
	case "yellow":
		return Yellow, nil
	case "blue":
		return Blue, nil
	case "magenta":
		return Magenta, nil
	case "cyan":
		return Cyan, nil
	case "gray":
		return Gray, nil
	case "darkgray":
		return DarkGray, nil
	case "lightred":
		return LightRed, nil
	case "lightgreen":
		return LightGreen, nil
	case "lightyellow":
		return LightYellow, nil
	case "lightblue":
		return LightBlue, nil
	case "lightmagenta":
		return LightMagenta, nil
	case "lightcyan":
		return LightCyan, nil
	case "white":
		return White, nil
	}

	if idx, err := strconv.ParseUint(s, 10, 8); err == nil {
		return Indexed(uint8(idx)), nil
	}
	if r, g, b, ok := parseHexColor(s); ok {
		return RGB(r, g, b), nil
	}
	return Color{}, ParseColorError{Value: s}
}

func parseHexColor(input string) (r, g, b uint8, ok bool) {
	if len(input) != 7 || input[0] != '#' {
		return 0, 0, 0, false
	}
	rv, err1 := strconv.ParseUint(input[1:3], 16, 8)
	gv, err2 := strconv.ParseUint(input[3:5], 16, 8)
	bv, err3 := strconv.ParseUint(input[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return uint8(rv), uint8(gv), uint8(bv), true
}
