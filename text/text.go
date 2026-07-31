package text

import (
	"strings"

	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
)

// Text is one or more lines of styled text.
//
// Alignment is a pointer so nil means "inherit from parent widget".
type Text struct {
	Lines     []Line
	Style     style.Style
	Alignment *layout.Alignment
}

// RawText builds text from content, splitting on newlines.
// Empty content yields a single empty line (Ratatui Text::raw("")).
func RawText(content string) Text {
	return Text{Lines: contentToLines(content)}
}

// StyledText builds text with the given text-level style (patched onto default).
func StyledText(content string, sty style.Style) Text {
	t := RawText(content)
	t.Style = t.Style.Patch(sty)
	return t
}

// FromLines builds text from lines. The slice is copied.
func FromLines(lines ...Line) Text {
	return Text{Lines: copyLines(lines)}
}

// FromLineSlice builds text from a line slice. The slice is copied.
func FromLineSlice(lines []Line) Text {
	return Text{Lines: copyLines(lines)}
}

// FromSpan builds a single-line text from one span.
func FromSpan(span Span) Text {
	return FromLines(FromSpans(span))
}

// FromLine builds text from one line.
func FromLine(line Line) Text {
	return FromLines(line)
}

func contentToLines(content string) []Line {
	// Ratatui Text::raw: empty => vec![Line::from("")]; else lines().map(Line::from)
	if content == "" {
		return []Line{{Spans: []Span{{Content: ""}}}}
	}
	parts := splitContentLines(content)
	if parts == nil {
		return []Line{{Spans: []Span{{Content: ""}}}}
	}
	out := make([]Line, len(parts))
	for i, p := range parts {
		out[i] = Line{Spans: []Span{{Content: p}}}
	}
	return out
}

func copyLines(lines []Line) []Line {
	if len(lines) == 0 {
		return nil
	}
	out := make([]Line, len(lines))
	for i := range lines {
		out[i] = Line{
			Spans:     copySpans(lines[i].Spans),
			Style:     lines[i].Style,
			Alignment: copyAlignment(lines[i].Alignment),
		}
	}
	return out
}

func copyAlignment(a *layout.Alignment) *layout.Alignment {
	if a == nil {
		return nil
	}
	v := *a
	return &v
}

// WithStyle replaces the text style.
func (t Text) WithStyle(sty style.Style) Text {
	t.Style = sty
	return t
}

// PatchStyle merges sty onto the text style.
func (t Text) PatchStyle(sty style.Style) Text {
	t.Style = t.Style.Patch(sty)
	return t
}

// ResetStyle patches with style.ResetStyle().
func (t Text) ResetStyle() Text {
	return t.PatchStyle(style.ResetStyle())
}

// WithAlignment sets horizontal alignment for the whole text.
func (t Text) WithAlignment(a layout.Alignment) Text {
	t.Alignment = &a
	return t
}

// LeftAligned sets alignment to AlignLeft.
func (t Text) LeftAligned() Text {
	return t.WithAlignment(layout.AlignLeft)
}

// Centered sets alignment to AlignCenter.
func (t Text) Centered() Text {
	return t.WithAlignment(layout.AlignCenter)
}

// RightAligned sets alignment to AlignRight.
func (t Text) RightAligned() Text {
	return t.WithAlignment(layout.AlignRight)
}

// Width returns the max terminal cell width of all lines.
func (t Text) Width() int {
	max := 0
	for i := range t.Lines {
		if w := t.Lines[i].Width(); w > max {
			max = w
		}
	}
	return max
}

// Height returns the number of lines.
func (t Text) Height() int {
	return len(t.Lines)
}

// PushLine appends a line.
func (t *Text) PushLine(line Line) {
	t.Lines = append(t.Lines, line)
}

// PushSpan appends a span to the last line, or creates a line if empty.
func (t *Text) PushSpan(span Span) {
	if len(t.Lines) == 0 {
		t.Lines = append(t.Lines, FromSpans(span))
		return
	}
	t.Lines[len(t.Lines)-1].PushSpan(span)
}

// String joins lines with newlines.
func (t Text) String() string {
	if len(t.Lines) == 0 {
		return ""
	}
	parts := make([]string, len(t.Lines))
	for i := range t.Lines {
		parts[i] = t.Lines[i].String()
	}
	return strings.Join(parts, "\n")
}

// EffectiveAlignment returns the line's alignment if set, else the text's, else nil.
func (t Text) EffectiveAlignment(line Line) *layout.Alignment {
	if line.Alignment != nil {
		return line.Alignment
	}
	return t.Alignment
}

// LineRenderData returns paint data for line i inside an area of the given width,
// applying text-level alignment inheritance. lineStyle is text.Style.Patch(line.Style).
func (t Text) LineRenderData(i int, areaWidth int) (spans []Span, drawnWidth int, leftPad int, lineStyle style.Style, ok bool) {
	if i < 0 || i >= len(t.Lines) {
		return nil, 0, 0, style.Style{}, false
	}
	line := t.Lines[i]
	if line.Alignment == nil && t.Alignment != nil {
		a := *t.Alignment
		line.Alignment = &a
	}
	lineStyle = t.Style.Patch(line.Style)
	spans, drawnWidth, leftPad = line.RenderData(areaWidth)
	return spans, drawnWidth, leftPad, lineStyle, true
}
