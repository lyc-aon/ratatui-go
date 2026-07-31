package text

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
)

// Line is one row of text made of styled spans.
//
// Alignment is a pointer so nil means "inherit from parent Text/widget".
type Line struct {
	Spans     []Span
	Style     style.Style
	Alignment *layout.Alignment
}

// RawLine builds a line from content. Embedded newlines become separate spans
// (Ratatui strips newlines by splitting into spans).
func RawLine(content string) Line {
	return Line{Spans: contentToSpans(content)}
}

// StyledLine builds a line with the given line-level style.
func StyledLine(content string, sty style.Style) Line {
	return Line{Spans: contentToSpans(content), Style: sty}
}

// FromSpans builds a line from spans. The slice is copied.
func FromSpans(spans ...Span) Line {
	return Line{Spans: copySpans(spans)}
}

// FromSpanSlice builds a line from a span slice. The slice is copied.
func FromSpanSlice(spans []Span) Line {
	return Line{Spans: copySpans(spans)}
}

func contentToSpans(content string) []Span {
	// Rust str::lines() omits trailing empty after final newline; empty => no lines.
	// Line::raw("") therefore has empty spans (Default).
	if content == "" {
		return nil
	}
	parts := splitContentLines(content)
	out := make([]Span, len(parts))
	for i, p := range parts {
		out[i] = Span{Content: p}
	}
	return out
}

// splitContentLines matches Rust str::lines():
// splits on '\n' only, strips a preceding '\r' from a '\r\n' terminator,
// leaves lone '\r' as content, drops a trailing empty segment after a final
// newline, and yields nothing for "".
func splitContentLines(s string) []string {
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "\n")
	// strings.Split keeps a trailing empty after a final '\n'; drop it.
	if raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	// A part was newline-terminated if it is not the last part, or the source
	// ended with '\n' (in which case every retained part was terminated).
	endedWithNL := strings.HasSuffix(s, "\n")
	for i := range raw {
		terminated := endedWithNL || i < len(raw)-1
		if terminated && strings.HasSuffix(raw[i], "\r") {
			raw[i] = raw[i][:len(raw[i])-1]
		}
	}
	return raw
}

func copySpans(spans []Span) []Span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]Span, len(spans))
	copy(out, spans)
	return out
}

// WithSpans replaces spans (copies the input slice).
func (l Line) WithSpans(spans ...Span) Line {
	l.Spans = copySpans(spans)
	return l
}

// WithStyle replaces the line style.
func (l Line) WithStyle(sty style.Style) Line {
	l.Style = sty
	return l
}

// PatchStyle merges sty onto the line style.
func (l Line) PatchStyle(sty style.Style) Line {
	l.Style = l.Style.Patch(sty)
	return l
}

// ResetStyle patches with style.ResetStyle().
func (l Line) ResetStyle() Line {
	return l.PatchStyle(style.ResetStyle())
}

// WithAlignment sets horizontal alignment.
func (l Line) WithAlignment(a layout.Alignment) Line {
	l.Alignment = &a
	return l
}

// LeftAligned sets alignment to AlignLeft.
func (l Line) LeftAligned() Line {
	return l.WithAlignment(layout.AlignLeft)
}

// Centered sets alignment to AlignCenter.
func (l Line) Centered() Line {
	return l.WithAlignment(layout.AlignCenter)
}

// RightAligned sets alignment to AlignRight.
func (l Line) RightAligned() Line {
	return l.WithAlignment(layout.AlignRight)
}

// Width returns the total terminal cell width of all spans.
func (l Line) Width() int {
	w := 0
	for i := range l.Spans {
		w += l.Spans[i].Width()
	}
	return w
}

// StyledGraphemes yields graphemes with base patched by the line style, then
// each span style.
func (l Line) StyledGraphemes(base style.Style) []StyledGrapheme {
	sty := base.Patch(l.Style)
	var out []StyledGrapheme
	for i := range l.Spans {
		out = append(out, l.Spans[i].StyledGraphemes(sty)...)
	}
	return out
}

// PushSpan appends a span.
func (l *Line) PushSpan(span Span) {
	l.Spans = append(l.Spans, span)
}

// String concatenates span contents.
func (l Line) String() string {
	if len(l.Spans) == 0 {
		return ""
	}
	if len(l.Spans) == 1 {
		return l.Spans[0].String()
	}
	var b strings.Builder
	for i := range l.Spans {
		b.WriteString(l.Spans[i].String())
	}
	return b.String()
}

// RenderData computes placement for a single-row area of the given width.
//
// Returns the spans to draw (possibly start-truncated), total content width
// actually drawn, and left padding in cells. Does not write to a buffer —
// buffer package owns cell writes.
//
// When the line fits the area, the returned spans slice aliases l.Spans and
// must be treated as read-only. Truncation paths return a fresh slice.
func (l Line) RenderData(areaWidth int) (spans []Span, drawnWidth int, leftPad int) {
	if areaWidth <= 0 {
		return nil, 0, 0
	}
	lineWidth := l.Width()
	if lineWidth == 0 {
		return nil, 0, 0
	}

	align := layout.AlignLeft
	if l.Alignment != nil {
		align = *l.Alignment
	}

	if lineWidth <= areaWidth {
		pad := 0
		switch align {
		case layout.AlignCenter:
			pad = (areaWidth - lineWidth) / 2
		case layout.AlignRight:
			pad = areaWidth - lineWidth
		}
		// Alias: callers must not mutate. Constructors already copy inbound slices.
		return l.Spans, lineWidth, pad
	}

	// Truncate from the left for center/right; left truncates only the right edge at paint.
	skip := 0
	switch align {
	case layout.AlignCenter:
		skip = (lineWidth - areaWidth) / 2
	case layout.AlignRight:
		skip = lineWidth - areaWidth
	}
	return spansAfterWidth(l.Spans, skip, areaWidth)
}

// spansAfterWidth skips skip cells from the start, then keeps content up to
// maxWidth cells. Partially visible first span is start-truncated.
//
// When a wide grapheme straddles the left cut, first_grapheme_offset is
// preserved as leftPad (and consumes area budget) even if that span contributes
// no drawable content — matching ratatui spans_after_width.
func spansAfterWidth(spans []Span, skip, maxWidth int) ([]Span, int, int) {
	if maxWidth <= 0 {
		return nil, 0, 0
	}
	var out []Span
	drawn := 0
	leftPad := 0
	remainingSkip := skip

	for i := range spans {
		if leftPad+drawn >= maxWidth {
			break
		}
		sw := spans[i].Width()
		if remainingSkip >= sw {
			remainingSkip -= sw
			continue
		}
		content := spans[i].Content
		if remainingSkip > 0 {
			// Visible sliver starts inside this span.
			available := sw - remainingSkip
			remainingSkip = 0
			trimmed, actual := TruncateStart(content, available)
			firstOff := available - actual

			// Record left-edge gap once, even when the sliver is empty (half a
			// wide grapheme). Gap consumes area budget for following spans.
			if len(out) == 0 {
				leftPad = firstOff
			}

			budget := maxWidth - leftPad - drawn
			if budget <= 0 {
				break
			}
			if actual > budget {
				// Keep firstOff/leftPad; only shrink content to the remaining cells.
				trimmed, actual = Truncate(trimmed, budget)
			}
			if actual > 0 {
				out = append(out, Span{Content: trimmed, Style: spans[i].Style})
				drawn += actual
			}
			continue
		}

		// Fully at start of span; may need end-truncate.
		budget := maxWidth - leftPad - drawn
		if budget <= 0 {
			break
		}
		if sw <= budget {
			out = append(out, spans[i])
			drawn += sw
			continue
		}
		trimmed, actual := Truncate(content, budget)
		if actual > 0 {
			out = append(out, Span{Content: trimmed, Style: spans[i].Style})
			drawn += actual
		}
		break
	}
	return out, drawn, leftPad
}
