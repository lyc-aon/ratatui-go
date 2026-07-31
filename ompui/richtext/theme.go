package richtext

// StyleFunc styles a string with ANSI and returns the result.
type StyleFunc func(string) string

// Theme holds per-element style hooks and terminal capability flags for
// markdown rendering. Nil StyleFunc values act as identity. There is no package
// global theme — callers pass Theme by value into constructors.
type Theme struct {
	Heading1       StyleFunc
	Heading2       StyleFunc
	Heading3       StyleFunc
	Bold           StyleFunc
	Italic         StyleFunc
	Strikethrough  StyleFunc
	Code           StyleFunc
	CodeBlock      StyleFunc
	CodeBlockBorder StyleFunc
	Quote          StyleFunc
	QuoteBorder    StyleFunc
	HR             StyleFunc
	ListBullet     StyleFunc
	Link           StyleFunc
	LinkURL        StyleFunc
	TableHeader    StyleFunc

	// HighlightCode optionally syntax-highlights a fenced code body. Each
	// returned string is one already-styled source line (no trailing newline).
	// When nil, CodeBlock styles plain lines.
	HighlightCode func(code, lang string) []string

	// ResolveMermaidASCII optionally turns a mermaid fence into preformatted
	// ASCII art. ok=false falls back to normal fenced-code rendering.
	ResolveMermaidASCII func(source string, maxWidth int) (string, bool)

	// Hyperlinks enables OSC 8 wrappers on markdown links.
	Hyperlinks bool
	// TextSizing enables OSC 66 double-height H1 when the plain heading fits.
	TextSizing bool
	// ColorSwatch is the chip glyph painted before inline hex colors (e.g. "■").
	// Empty disables swatches.
	ColorSwatch string

	// QuoteBorderChar is the glyph before quote body lines (default "│").
	QuoteBorderChar string
	// HRChar is the default thematic-break fill (default "─").
	// ASCII "-" selects ASCII fallbacks for unicode source rules.
	HRChar string
	// Table holds box-drawing runes for GFM tables. Zero value uses Unicode defaults.
	Table TableSymbols
}

// TableSymbols are the box-drawing characters used by table borders.
type TableSymbols struct {
	TopLeft, TopRight, BottomLeft, BottomRight string
	Horizontal, Vertical                       string
	TeeDown, TeeUp, TeeLeft, TeeRight, Cross   string
}

// MarkdownOptions configures padding and optional full-line background paint.
type MarkdownOptions struct {
	PaddingX, PaddingY int
	// CodeBlockIndent is spaces before each code body line (default 2).
	CodeBlockIndent int
	// Background, when set, receives (line, width) and must return a line painted
	// to full width (OMP applyBackgroundToLine contract).
	Background func(line string, width int) string
}

func (t Theme) apply(fn StyleFunc, s string) string {
	if fn == nil {
		return s
	}
	return fn(s)
}

func (t Theme) headingStyle(level int) StyleFunc {
	switch level {
	case 1:
		return t.Heading1
	case 2:
		return t.Heading2
	default:
		return t.Heading3
	}
}

func (t Theme) quoteBorderChar() string {
	if t.QuoteBorderChar == "" {
		return "│"
	}
	return t.QuoteBorderChar
}

func (t Theme) hrChar() string {
	if t.HRChar == "" {
		return "─"
	}
	return t.HRChar
}

func (t Theme) tableSymbols() TableSymbols {
	s := t.Table
	if s.TopLeft == "" {
		s.TopLeft = "┌"
	}
	if s.TopRight == "" {
		s.TopRight = "┐"
	}
	if s.BottomLeft == "" {
		s.BottomLeft = "└"
	}
	if s.BottomRight == "" {
		s.BottomRight = "┘"
	}
	if s.Horizontal == "" {
		s.Horizontal = "─"
	}
	if s.Vertical == "" {
		s.Vertical = "│"
	}
	if s.TeeDown == "" {
		s.TeeDown = "┬"
	}
	if s.TeeUp == "" {
		s.TeeUp = "┴"
	}
	if s.TeeLeft == "" {
		s.TeeLeft = "┤"
	}
	if s.TeeRight == "" {
		s.TeeRight = "├"
	}
	if s.Cross == "" {
		s.Cross = "┼"
	}
	return s
}
