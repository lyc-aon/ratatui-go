package widgets

import (
	"strconv"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
)

// Mascot art characters (source half-row encoding).
const (
	mascotEmpty      = ' '
	mascotRat        = '█'
	mascotHat        = 'h'
	mascotEye        = 'e'
	mascotTerm       = '░'
	mascotTermBorder = '▒'
	mascotTermCursor = '▓'
)

// Exact upstream RATATUI_MASCOT art after indoc! common-indent strip (4 spaces).
// Pairs of lines → half-block cells. Zip is shortest-line (Rust chars().zip).
const ratatuiMascotArt = "" +
	"               hhh\n" +
	"             hhhhhh\n" +
	"            hhhhhhh\n" +
	"           hhhhhhhh\n" +
	"          hhhhhhhhh\n" +
	"         hhhhhhhhhh\n" +
	"        hhhhhhhhhhhh\n" +
	"        hhhhhhhhhhhhh\n" +
	"        hhhhhhhhhhhhh     ██████\n" +
	"         hhhhhhhhhhh    ████████\n" +
	"              hhhhh ███████████\n" +
	"               hhh ██ee████████\n" +
	"                h █████████████\n" +
	"            ████ █████████████\n" +
	"           █████████████████\n" +
	"           ████████████████\n" +
	"           ████████████████\n" +
	"            ███ ██████████\n" +
	"          ▒▒    █████████\n" +
	"         ▒░░▒   █████████\n" +
	"        ▒░░░░▒ ██████████\n" +
	"       ▒░░▓░░░▒ █████████\n" +
	"      ▒░░▓▓░░░░▒ ████████\n" +
	"     ▒░░░░░░░░░░▒ ██████████\n" +
	"    ▒░░░░░░░░░░░░▒ ██████████\n" +
	"   ▒░░░░░░░▓▓░░░░░▒ █████████\n" +
	"  ▒░░░░░░░░░▓▓░░░░░▒ ████  ███\n" +
	" ▒░░░░░░░░░░░░░░░░░░▒ ██   ███\n" +
	"▒░░░░░░░░░░░░░░░░░░░░▒ █   ███\n" +
	"▒░░░░░░░░░░░░░░░░░░░░░▒   ███\n" +
	" ▒░░░░░░░░░░░░░░░░░░░░░▒ ███\n" +
	"  ▒░░░░░░░░░░░░░░░░░░░░░▒ █"

// MascotEyeColor selects the mascot eye palette.
type MascotEyeColor int

const (
	// MascotEyeDefault is the normal dark eye.
	MascotEyeDefault MascotEyeColor = iota
	// MascotEyeRed is the blinking/red eye.
	MascotEyeRed
)

// String returns a stable name for the eye color state.
func (e MascotEyeColor) String() string {
	switch e {
	case MascotEyeDefault:
		return "Default"
	case MascotEyeRed:
		return "Red"
	default:
		return "MascotEyeColor(" + strconv.Itoa(int(e)) + ")"
	}
}

// RatatuiMascot renders the half-block Ratatui mascot (32×16 cells).
type RatatuiMascot struct {
	eyeState        MascotEyeColor
	ratColor        style.Color
	ratEyeColor     style.Color
	ratEyeBlink     style.Color
	hatColor        style.Color
	termColor       style.Color
	termBorderColor style.Color
	termCursorColor style.Color
}

// NewRatatuiMascot creates a mascot with the default palette.
func NewRatatuiMascot() RatatuiMascot {
	return RatatuiMascot{
		ratColor:        style.Indexed(252), // light_gray #d0d0d0
		hatColor:        style.Indexed(231), // white #ffffff
		ratEyeColor:     style.Indexed(236), // dark_charcoal #303030
		ratEyeBlink:     style.Indexed(196), // red #ff0000
		termColor:       style.Indexed(232), // vampire_black #080808
		termBorderColor: style.Indexed(237), // gray
		termCursorColor: style.Indexed(248), // dark_gray #a8a8a8
		eyeState:        MascotEyeDefault,
	}
}

// EyeColor sets the eye state (open / blinking red).
func (m RatatuiMascot) EyeColor(eye MascotEyeColor) RatatuiMascot {
	m.eyeState = eye
	return m
}

// SetEye is an alias for EyeColor matching upstream naming.
func (m RatatuiMascot) SetEye(eye MascotEyeColor) RatatuiMascot {
	return m.EyeColor(eye)
}

func (m RatatuiMascot) colorFor(c rune) (style.Color, bool) {
	switch c {
	case mascotRat:
		return m.ratColor, true
	case mascotHat:
		return m.hatColor, true
	case mascotEye:
		switch m.eyeState {
		case MascotEyeRed:
			return m.ratEyeBlink, true
		default:
			return m.ratEyeColor, true
		}
	case mascotTerm:
		return m.termColor, true
	case mascotTermCursor:
		return m.termCursorColor, true
	case mascotTermBorder:
		return m.termBorderColor, true
	default:
		return style.Color{}, false
	}
}

// Render draws the half-block mascot into area ∩ buf.Area.
// Zero-sized or clipped areas are a no-op; cells outside the art stay untouched.
func (m RatatuiMascot) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}

	lines := splitMascotLines(ratatuiMascotArt)
	// Pair consecutive source lines into half-block rows.
	row := 0
	for i := 0; i+1 < len(lines); i += 2 {
		line1 := []rune(lines[i])
		line2 := []rune(lines[i+1])
		// Zip shortest like Rust line1.chars().zip(line2.chars()).
		n := len(line1)
		if len(line2) < n {
			n = len(line2)
		}
		y := area.Y + row
		if y >= area.Bottom() {
			break
		}
		for xOff := range n {
			x := area.X + xOff
			if x >= area.Right() {
				break
			}
			ch1 := line1[xOff]
			ch2 := line2[xOff]

			cell := buf.GetMut(x, y)
			if cell == nil {
				continue
			}

			fg, bg := m.halfBlockColors(ch1, ch2)
			sym, ok := halfBlockSymbol(ch1, ch2)
			st := style.New()
			if fg != nil {
				st = st.WithFG(*fg)
			}
			if bg != nil {
				st = st.WithBG(*bg)
			}
			if fg != nil || bg != nil {
				cell.SetStyle(st)
			}
			if ok {
				cell.SetSymbol(string(sym))
			}
		}
		row++
	}
}

func (m RatatuiMascot) halfBlockColors(ch1, ch2 rune) (fg, bg *style.Color) {
	switch {
	case ch1 == mascotEmpty && ch2 == mascotEmpty:
		return nil, nil
	case ch2 == mascotEmpty:
		if c, ok := m.colorFor(ch1); ok {
			return &c, nil
		}
		return nil, nil
	case ch1 == mascotEmpty:
		if c, ok := m.colorFor(ch2); ok {
			return &c, nil
		}
		return nil, nil
	case ch1 == mascotTerm && ch2 == mascotTermBorder:
		cBorder, okB := m.colorFor(mascotTermBorder)
		cTerm, okT := m.colorFor(mascotTerm)
		var pf, pb *style.Color
		if okB {
			pf = &cBorder
		}
		if okT {
			pb = &cTerm
		}
		return pf, pb
	case ch1 == mascotTerm:
		c2, ok2 := m.colorFor(ch2)
		cTerm, okT := m.colorFor(mascotTerm)
		var pf, pb *style.Color
		if ok2 {
			pf = &c2
		}
		if okT {
			pb = &cTerm
		}
		return pf, pb
	case ch2 == mascotTerm:
		c1, ok1 := m.colorFor(ch1)
		cTerm, okT := m.colorFor(mascotTerm)
		var pf, pb *style.Color
		if ok1 {
			pf = &c1
		}
		if okT {
			pb = &cTerm
		}
		return pf, pb
	default:
		c1, ok1 := m.colorFor(ch1)
		c2, ok2 := m.colorFor(ch2)
		var pf, pb *style.Color
		if ok1 {
			pf = &c1
		}
		if ok2 {
			pb = &c2
		}
		return pf, pb
	}
}

func halfBlockSymbol(ch1, ch2 rune) (rune, bool) {
	switch {
	case ch1 == mascotEmpty && ch2 == mascotEmpty:
		return 0, false
	case ch1 == mascotTerm && ch2 == mascotTerm:
		return mascotEmpty, true
	case ch2 == mascotEmpty || ch2 == mascotTerm:
		return symbols.HalfBlockUpper, true
	case ch1 == mascotEmpty || ch1 == mascotTerm:
		return symbols.HalfBlockLower, true
	case ch1 == ch2:
		return symbols.HalfBlockFull, true
	default:
		return symbols.HalfBlockUpper, true
	}
}

func splitMascotLines(s string) []string {
	if s == "" {
		return nil
	}
	// Manual split keeps a final non-empty line and drops a trailing bare newline,
	// matching str::lines() / indoc output.
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
