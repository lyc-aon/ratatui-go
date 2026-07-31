package widgets

import (
	"strconv"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

// Bar is one bar shown by a BarChart.
//
// Vertical charts draw the value inside the bar (when it fits) and the label
// under the bar. Horizontal charts draw the label to the left and the value
// inside the bar start.
type Bar struct {
	Value      uint64
	Label      *text.Line
	Style      style.Style
	ValueStyle style.Style
	// TextValue overrides the printed value text when non-nil.
	TextValue *string
}

// NewBar creates a bar with the given value.
func NewBar(value uint64) Bar {
	return Bar{Value: value}
}

// BarWithLabel creates a bar with label and value.
func BarWithLabel(label text.Line, value uint64) Bar {
	return Bar{Value: value, Label: copyLinePtr(&label)}
}

// WithValue sets the numeric value.
func (b Bar) WithValue(value uint64) Bar {
	b.Value = value
	return b
}

// WithLabel sets the bar label (copied).
func (b Bar) WithLabel(label text.Line) Bar {
	b.Label = copyLinePtr(&label)
	return b
}

// WithStyle sets the default bar style (patched by chart bar style).
func (b Bar) WithStyle(s style.Style) Bar {
	b.Style = s
	return b
}

// WithValueStyle sets the style of the value text printed on the bar.
func (b Bar) WithValueStyle(s style.Style) Bar {
	b.ValueStyle = s
	return b
}

// WithTextValue sets an optional display override for the value text.
func (b Bar) WithTextValue(v string) Bar {
	b.TextValue = copyStringPtr(&v)
	return b
}

func (b Bar) valueText() string {
	if b.TextValue != nil {
		return *b.TextValue
	}
	return strconv.FormatUint(b.Value, 10)
}

// renderValueWithDifferentStyles prints the value text starting at area.
// The first barLength cells use value style; any overflow uses bar style.
// Matches Ratatui Bar::render_value_with_different_styles (horizontal bars).
func (b Bar) renderValueWithDifferentStyles(
	buf *buffer.Buffer,
	area layout.Rect,
	barLength int,
	defaultValueStyle style.Style,
	barStyle style.Style,
) {
	if buf == nil || area.IsEmpty() {
		return
	}
	txt := b.valueText()
	if txt == "" {
		return
	}
	if barLength < 0 {
		barLength = 0
	}

	valStyle := defaultValueStyle.Patch(b.ValueStyle)
	buf.SetStringN(area.X, area.Y, txt, barLength, valStyle)

	// Overflow beyond the filled bar length is painted with the bar style.
	// Rust barchart/bar.rs splits on a UTF-8 char boundary at-or-before
	// bar_length *bytes* (not cells), then offsets x by first.len() bytes and
	// uses area.width - first.len() as the remaining max width.
	if len(txt) > barLength {
		splitAt := 0
		for i, r := range txt {
			if i >= barLength {
				break
			}
			splitAt = i + utf8.RuneLen(r)
		}
		// splitAt is a valid UTF-8 boundary (0 or end of a rune).
		firstLen := splitAt
		second := txt[splitAt:]
		if second == "" {
			return
		}
		restStyle := barStyle.Patch(b.Style)
		// Clamp negative remaining width (Rust would underflow/panic on usize).
		maxW := area.Width - firstLen
		if maxW < 0 {
			maxW = 0
		}
		if maxW > 0 {
			buf.SetStringN(area.X+firstLen, area.Y, second, maxW, restStyle)
		}
	}
}

// renderValue prints a centered value inside a vertical bar when it fits.
// ticks is the bar height in eighth-blocks; a full cell is 8 ticks.
func (b Bar) renderValue(
	buf *buffer.Buffer,
	maxWidth int,
	x, y int,
	defaultValueStyle style.Style,
	ticks uint64,
) {
	if buf == nil || b.Value == 0 || maxWidth <= 0 {
		return
	}
	const ticksPerLine uint64 = 8
	valueLabel := b.valueText()
	width := text.GraphemeWidth(valueLabel)
	// Print when the label is narrower than the bar, or exactly as wide and
	// the bar is at least one full cell tall.
	if width < maxWidth || (width == maxWidth && ticks >= ticksPerLine) {
		pad := (maxWidth - width) >> 1
		if pad < 0 {
			pad = 0
		}
		buf.SetString(x+pad, y, valueLabel, defaultValueStyle.Patch(b.ValueStyle))
	}
}

// renderLabel centers the bar label under a vertical bar.
func (b Bar) renderLabel(
	buf *buffer.Buffer,
	maxWidth int,
	x, y int,
	defaultLabelStyle style.Style,
) {
	if buf == nil || maxWidth <= 0 {
		return
	}
	width := 0
	if b.Label != nil {
		width = b.Label.Width()
	}
	if width > maxWidth {
		width = maxWidth
	}
	area := layout.NewRect(x+(maxWidth-width)/2, y, width, 1)
	buf.SetStyle(area, defaultLabelStyle)
	if b.Label != nil {
		renderLineInArea(buf, area, *b.Label)
	}
}

func copyLinePtr(l *text.Line) *text.Line {
	if l == nil {
		return nil
	}
	cp := text.FromSpanSlice(l.Spans)
	cp.Style = l.Style
	if l.Alignment != nil {
		a := *l.Alignment
		cp.Alignment = &a
	}
	return &cp
}

func copyStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

// renderLineInArea paints a line into area using the line's alignment.
func renderLineInArea(buf *buffer.Buffer, area layout.Rect, line text.Line) {
	if buf == nil || area.IsEmpty() {
		return
	}
	spans, _, leftPad := line.RenderData(area.Width)
	x := area.X + leftPad
	right := area.Right()
	for i := range spans {
		if x >= right {
			break
		}
		st := line.Style.Patch(spans[i].Style)
		nx, _ := buf.SetStringN(x, area.Y, spans[i].Content, right-x, st)
		if nx <= x {
			break
		}
		x = nx
	}
}
