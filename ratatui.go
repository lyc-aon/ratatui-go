package ratatui

import (
	"io"

	"github.com/michaelkelly/ratatui-go/backend"
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/terminal"
	"github.com/michaelkelly/ratatui-go/text"
	"github.com/michaelkelly/ratatui-go/widget"
)

// Core aliases keep the common application path in one package. Lower-level
// integrations can import the focused subpackages directly.
type (
	Position       = layout.Position
	Size           = layout.Size
	Rect           = layout.Rect
	Margin         = layout.Margin
	Alignment      = layout.Alignment
	Direction      = layout.Direction
	Constraint     = layout.Constraint
	Flex           = layout.Flex
	Color          = style.Color
	Modifier       = style.Modifier
	Style          = style.Style
	Span           = text.Span
	Line           = text.Line
	Text           = text.Text
	Cell           = buffer.Cell
	Buffer         = buffer.Buffer
	PositionedCell = buffer.PositionedCell
	Widget         = widget.Widget
	Backend        = backend.Backend
	TestBackend    = backend.TestBackend
	ANSIBackend    = backend.ANSIBackend
	Terminal       = terminal.Terminal
	Frame          = terminal.Frame
	CompletedFrame = terminal.CompletedFrame
	TerminalOption = terminal.Option
)

const (
	AlignLeft   = layout.AlignLeft
	AlignCenter = layout.AlignCenter
	AlignRight  = layout.AlignRight

	Horizontal = layout.HorizontalDir
	Vertical   = layout.VerticalDir

	ModBold       = style.ModBold
	ModDim        = style.ModDim
	ModItalic     = style.ModItalic
	ModUnderlined = style.ModUnderlined
	ModReversed   = style.ModReversed
)

var (
	Reset        = style.Reset
	Black        = style.Black
	Red          = style.Red
	Green        = style.Green
	Yellow       = style.Yellow
	Blue         = style.Blue
	Magenta      = style.Magenta
	Cyan         = style.Cyan
	Gray         = style.Gray
	DarkGray     = style.DarkGray
	LightRed     = style.LightRed
	LightGreen   = style.LightGreen
	LightYellow  = style.LightYellow
	LightBlue    = style.LightBlue
	LightMagenta = style.LightMagenta
	LightCyan    = style.LightCyan
	White        = style.White
)

func NewTerminal(b Backend, opts ...TerminalOption) (*Terminal, error) {
	return terminal.New(b, opts...)
}

func NewTestBackend(width, height int) *TestBackend {
	return backend.NewTestBackend(width, height)
}

func NewANSIBackend(w io.Writer, width, height int) *ANSIBackend {
	return backend.NewANSIBackend(w, width, height)
}

func NewStyle() Style { return style.New() }

func ResetStyle() Style { return style.ResetStyle() }

func Indexed(index uint8) Color { return style.Indexed(index) }

func RGB(red, green, blue uint8) Color { return style.RGB(red, green, blue) }

func RawSpan(content string) Span { return text.RawSpan(content) }

func StyledSpan(content string, value Style) Span { return text.StyledSpan(content, value) }

func RawLine(content string) Line { return text.RawLine(content) }

func StyledLine(content string, value Style) Line { return text.StyledLine(content, value) }

func RawText(content string) Text { return text.RawText(content) }

func StyledText(content string, value Style) Text { return text.StyledText(content, value) }

func Length(value int) Constraint { return layout.Length(value) }

func Min(value int) Constraint { return layout.Min(value) }

func Max(value int) Constraint { return layout.Max(value) }

func Percentage(value int) Constraint { return layout.Percentage(value) }

func Ratio(numerator, denominator int) Constraint { return layout.Ratio(numerator, denominator) }

func Fill(weight int) Constraint { return layout.Fill(weight) }
