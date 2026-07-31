package ratatui

import (
	"github.com/lyc-aon/ratatui-go/backend"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/terminal"
	"github.com/lyc-aon/ratatui-go/text"
	"github.com/lyc-aon/ratatui-go/widget"
)

// Additional aliases that complete the application-facing surface.
// Types already declared in ratatui.go are intentionally not repeated.
type (
	Layout              = layout.Layout
	Offset              = layout.Offset
	VerticalAlignment   = layout.VerticalAlignment
	HorizontalAlignment = layout.Alignment // Rust renamed Alignment → HorizontalAlignment
	RatioPair           = layout.RatioPair
	ClearType           = backend.ClearType
	WindowSize          = backend.WindowSize
	TTYBackend          = backend.TTYBackend

	Viewport        = terminal.Viewport
	TerminalOptions = terminal.Options

	WidgetFunc   = widget.WidgetFunc
	Masked       = text.Masked
	StringWidget = widget.StringWidget
	SpanWidget   = widget.SpanWidget
	LineWidget   = widget.LineWidget
	TextWidget   = widget.TextWidget
)

// Generic widget aliases (Go 1.18+ type aliases to generic types).
type (
	StatefulWidget[S any]     = widget.StatefulWidget[S]
	StatefulWidgetFunc[S any] = widget.StatefulWidgetFunc[S]
)

const (
	// Vertical alignment.
	AlignTop    = layout.AlignTop
	AlignMiddle = layout.AlignMiddle
	AlignBottom = layout.AlignBottom

	// Flex modes (layout.DefaultFlex is FlexStart).
	DefaultFlex      = layout.DefaultFlex
	FlexStart        = layout.FlexStart
	FlexLegacy       = layout.FlexLegacy
	FlexEnd          = layout.FlexEnd
	FlexCenter       = layout.FlexCenter
	FlexSpaceBetween = layout.FlexSpaceBetween
	FlexSpaceAround  = layout.FlexSpaceAround
	FlexSpaceEvenly  = layout.FlexSpaceEvenly

	// Remaining modifiers (subset already in ratatui.go).
	ModSlowBlink  = style.ModSlowBlink
	ModRapidBlink = style.ModRapidBlink
	ModHidden     = style.ModHidden
	ModCrossedOut = style.ModCrossedOut
	ModAll        = style.ModAll

	// Viewport modes.
	ViewportFullscreen = terminal.ViewportFullscreen
	ViewportFixed      = terminal.ViewportFixed
	ViewportInline     = terminal.ViewportInline

	// Backend clear regions.
	ClearAll          = backend.All
	ClearAfterCursor  = backend.AfterCursor
	ClearBeforeCursor = backend.BeforeCursor
	ClearCurrentLine  = backend.CurrentLine
	ClearUntilNewLine = backend.UntilNewLine
)

// --- Geometry / layout constructors -----------------------------------------

var Origin = layout.Origin

func NewPosition(x, y int) Position { return layout.NewPosition(x, y) }

func NewSize(width, height int) Size { return layout.NewSize(width, height) }

func NewRect(x, y, width, height int) Rect { return layout.NewRect(x, y, width, height) }

func NewMargin(horizontal, vertical int) Margin {
	return layout.NewMargin(horizontal, vertical)
}

func UniformMargin(n int) Margin { return layout.UniformMargin(n) }

func NewOffset(x, y int) Offset { return layout.NewOffset(x, y) }

// NewLayout builds a Layout with the given direction and constraints.
func NewLayout(direction Direction, constraints ...Constraint) Layout {
	return layout.New(direction, constraints...)
}

// VerticalLayout builds a top-to-bottom layout.
// Named to avoid colliding with the Vertical direction constant.
func VerticalLayout(constraints ...Constraint) Layout {
	return layout.Vertical(constraints...)
}

// HorizontalLayout builds a left-to-right layout.
// Named to avoid colliding with the Horizontal direction constant.
func HorizontalLayout(constraints ...Constraint) Layout {
	return layout.Horizontal(constraints...)
}

// Constraint batch helpers.

func FromLengths(lengths ...int) []Constraint { return layout.FromLengths(lengths...) }
func FromMins(mins ...int) []Constraint       { return layout.FromMins(mins...) }
func FromMaxes(maxes ...int) []Constraint     { return layout.FromMaxes(maxes...) }
func FromPercentages(percentages ...int) []Constraint {
	return layout.FromPercentages(percentages...)
}
func FromFills(weights ...int) []Constraint       { return layout.FromFills(weights...) }
func FromRatios(ratios ...RatioPair) []Constraint { return layout.FromRatios(ratios...) }

// --- Terminal options -------------------------------------------------------

func WithViewport(v Viewport) TerminalOption { return terminal.WithViewport(v) }

func WithFixedArea(area Rect) TerminalOption { return terminal.WithFixedArea(area) }

func WithInlineHeight(height int) TerminalOption { return terminal.WithInlineHeight(height) }

func NewTerminalWithOptions(b Backend, o TerminalOptions) (*Terminal, error) {
	return terminal.NewWithOptions(b, o)
}

// RenderStateful renders a StatefulWidget with the given state pointer.
func RenderStateful[S any](f *Frame, w StatefulWidget[S], area Rect, state *S) {
	terminal.RenderStateful(f, w, area, state)
}

// --- Style / color constructors ---------------------------------------------

func FromU32(u uint32) Color { return style.FromU32(u) }

func ParseColor(s string) (Color, error) { return style.ParseColor(s) }

func FromHSL(hue, saturation, lightness float64) Color {
	return style.FromHSL(hue, saturation, lightness)
}

func FromHSLuv(hue, saturation, lightness float64) Color {
	return style.FromHSLuv(hue, saturation, lightness)
}

func FromColor(c Color) Style { return style.FromColor(c) }

func FromColors(fg, bg Color) Style { return style.FromColors(fg, bg) }

func FromModifier(m Modifier) Style { return style.FromModifier(m) }

func FromModifiers(add, sub Modifier) Style { return style.FromModifiers(add, sub) }

// --- Text helpers ------------------------------------------------------------

func NewMasked(s string, maskChar rune) Masked { return text.NewMasked(s, maskChar) }

func Spanf(format string, args ...any) Span { return text.Spanf(format, args...) }

func StyledSpanf(format string, value Style, args ...any) Span {
	return text.StyledSpanf(format, value, args...)
}

func Linef(format string, args ...any) Line { return text.Linef(format, args...) }

func StyledLinef(format string, value Style, args ...any) Line {
	return text.StyledLinef(format, value, args...)
}

func Textf(format string, args ...any) Text { return text.Textf(format, args...) }

func StyledTextf(format string, value Style, args ...any) Text {
	return text.StyledTextf(format, value, args...)
}

func FromSpans(spans ...Span) Line { return text.FromSpans(spans...) }

func FromSpanSlice(spans []Span) Line { return text.FromSpanSlice(spans) }

func FromLines(lines ...Line) Text { return text.FromLines(lines...) }

func FromLineSlice(lines []Line) Text { return text.FromLineSlice(lines) }

func FromSpan(span Span) Text { return text.FromSpan(span) }

func FromLine(line Line) Text { return text.FromLine(line) }

// AsWidget adapts a plain function into a Widget (WidgetFunc).
func AsWidget(f func(area Rect, buf *Buffer)) Widget {
	return widget.WidgetFunc(f)
}

// AsStatefulWidget adapts a plain function into a StatefulWidget[S].
func AsStatefulWidget[S any](f func(area Rect, buf *Buffer, state *S)) StatefulWidget[S] {
	return widget.StatefulWidgetFunc[S](f)
}

// AsStringWidget adapts a string to the core Widget contract.
func AsStringWidget(value string) StringWidget { return widget.String(value) }

// AsSpanWidget adapts a Span to the core Widget contract.
func AsSpanWidget(value Span) SpanWidget { return widget.Span(value) }

// AsLineWidget adapts a Line to the core Widget contract.
func AsLineWidget(value Line) LineWidget { return widget.Line(value) }

// AsTextWidget adapts Text to the core Widget contract.
func AsTextWidget(value Text) TextWidget { return widget.Text(value) }
