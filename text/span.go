package text

import (
	"strings"

	"github.com/michaelkelly/ratatui-go/style"
)

// Span is a contiguous run of text sharing one style.
type Span struct {
	Content string
	Style   style.Style
}

// RawSpan builds an unstyled span.
func RawSpan(content string) Span {
	return Span{Content: content}
}

// StyledSpan builds a span with the given style.
func StyledSpan(content string, sty style.Style) Span {
	return Span{Content: content, Style: sty}
}

// WithContent returns a copy with replaced content.
func (s Span) WithContent(content string) Span {
	s.Content = content
	return s
}

// WithStyle replaces the span style (does not patch).
func (s Span) WithStyle(sty style.Style) Span {
	s.Style = sty
	return s
}

// PatchStyle merges sty onto the span style.
func (s Span) PatchStyle(sty style.Style) Span {
	s.Style = s.Style.Patch(sty)
	return s
}

// ResetStyle patches Style with style.ResetStyle().
func (s Span) ResetStyle() Span {
	return s.PatchStyle(style.ResetStyle())
}

// Width returns the terminal cell width of the content.
func (s Span) Width() int {
	return GraphemeWidth(s.Content)
}

// String returns the content with newline separators removed (Ratatui Display).
// Lone '\r' is kept; only '\n' (and the '\r' of '\r\n') act as line breaks.
func (s Span) String() string {
	if !strings.Contains(s.Content, "\n") {
		return s.Content
	}
	return strings.Join(splitContentLines(s.Content), "")
}

// StyledGraphemes iterates grapheme clusters, patching base with the span style.
// Control-character clusters are omitted.
func (s Span) StyledGraphemes(base style.Style) []StyledGrapheme {
	sty := base.Patch(s.Style)
	gs := Graphemes(s.Content)
	out := make([]StyledGrapheme, len(gs))
	for i, g := range gs {
		out[i] = StyledGrapheme{Symbol: g, Style: sty}
	}
	return out
}

// IntoLeftAlignedLine converts the span into a left-aligned line.
func (s Span) IntoLeftAlignedLine() Line {
	return FromSpans(s).LeftAligned()
}

// IntoCenteredLine converts the span into a center-aligned line.
func (s Span) IntoCenteredLine() Line {
	return FromSpans(s).Centered()
}

// IntoRightAlignedLine converts the span into a right-aligned line.
func (s Span) IntoRightAlignedLine() Line {
	return FromSpans(s).RightAligned()
}
