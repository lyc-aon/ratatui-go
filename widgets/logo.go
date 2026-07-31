package widgets

import (
	"strconv"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/text"
)

// LogoSize selects which Ratatui logo art to render.
type LogoSize int

const (
	// LogoTiny is the default 2×15 logo.
	LogoTiny LogoSize = iota
	// LogoSmall is the 2×27 logo.
	LogoSmall
)

// String returns a stable name for the logo size.
func (s LogoSize) String() string {
	switch s {
	case LogoTiny:
		return "Tiny"
	case LogoSmall:
		return "Small"
	default:
		return "LogoSize(" + strconv.Itoa(int(s)) + ")"
	}
}

// RatatuiLogo renders the Ratatui wordmark as two lines of text art.
type RatatuiLogo struct {
	Size LogoSize
}

// NewRatatuiLogo creates a logo of the given size.
func NewRatatuiLogo(size LogoSize) RatatuiLogo {
	return RatatuiLogo{Size: size}
}

// Tiny returns a tiny (default) Ratatui logo.
func Tiny() RatatuiLogo {
	return NewRatatuiLogo(LogoTiny)
}

// Small returns a small Ratatui logo.
func Small() RatatuiLogo {
	return NewRatatuiLogo(LogoSmall)
}

// WithSize sets the logo size.
func (r RatatuiLogo) WithSize(size LogoSize) RatatuiLogo {
	r.Size = size
	return r
}

// Render draws the logo art into area ∩ buf.Area via the Text paint path.
func (r RatatuiLogo) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	renderText(text.RawText(r.Size.art()), area, buf)
}

func (s LogoSize) art() string {
	switch s {
	case LogoSmall:
		return logoSmall
	default:
		return logoTiny
	}
}

// Exact upstream art (indoc! strips the common leading indent and trailing newline).
const logoTiny = "▛▚▗▀▖▜▘▞▚▝▛▐ ▌▌\n▛▚▐▀▌▐ ▛▜ ▌▝▄▘▌"

const logoSmall = "█▀▀▄ ▄▀▀▄▝▜▛▘▄▀▀▄▝▜▛▘█  █ █\n█▀▀▄ █▀▀█ ▐▌ █▀▀█ ▐▌ ▀▄▄▀ █"

// renderText paints a text.Text the way ratatui Text::render does:
// set base style on area, then each line into successive rows.
func renderText(t text.Text, area layout.Rect, buf *buffer.Buffer) {
	buf.SetStyle(area, t.Style)
	rows := area.Rows()
	n := len(t.Lines)
	if n > len(rows) {
		n = len(rows)
	}
	for i := range n {
		lineArea := rows[i]
		if lineArea.IsEmpty() {
			continue
		}
		spans, _, leftPad, lineStyle, ok := t.LineRenderData(i, lineArea.Width)
		if !ok {
			continue
		}
		x := lineArea.X + leftPad
		remaining := lineArea.Width - leftPad
		if remaining < 0 {
			remaining = 0
		}
		for _, span := range spans {
			if remaining == 0 {
				break
			}
			st := lineStyle.Patch(span.Style)
			nx, _ := buf.SetStringN(x, lineArea.Y, span.Content, remaining, st)
			w := nx - x
			if w < 0 {
				w = 0
			}
			x = nx
			remaining -= w
			if remaining < 0 {
				remaining = 0
			}
		}
	}
}
