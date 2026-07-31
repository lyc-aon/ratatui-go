package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

// Wrap describes how Paragraph wraps text across lines.
type Wrap struct {
	// Trim removes leading whitespace from wrapped lines.
	Trim bool
}

// Paragraph displays a block of text with optional wrapping, alignment, scroll,
// and an optional surrounding Block.
type Paragraph struct {
	block     *Block
	style     style.Style
	wrap      *Wrap
	text      text.Text
	scrollY   int
	scrollX   int
	alignment layout.Alignment
}

// NewParagraph creates a Paragraph from text. If text carries an alignment it
// becomes the paragraph default; otherwise alignment is left.
//
// The text lines slice is copied so later mutation of the caller's data is safe.
func NewParagraph(t text.Text) Paragraph {
	align := layout.AlignLeft
	if t.Alignment != nil {
		align = *t.Alignment
	}
	copied := text.FromLineSlice(t.Lines)
	copied.Style = t.Style
	if t.Alignment != nil {
		a := *t.Alignment
		copied.Alignment = &a
	}
	return Paragraph{
		text:      copied,
		alignment: align,
	}
}

// Block surrounds the paragraph with a block.
func (p Paragraph) Block(b Block) Paragraph {
	p.block = &b
	return p
}

// Style sets the base style applied to the whole widget area (and block).
func (p Paragraph) Style(sty style.Style) Paragraph {
	p.style = sty
	return p
}

// WithWrap enables wrapping with the given options.
func (p Paragraph) WithWrap(w Wrap) Paragraph {
	p.wrap = &w
	return p
}

// Scroll sets the (vertical, horizontal) scroll offset.
// Note the argument order matches upstream: (y, x).
func (p Paragraph) Scroll(y, x int) Paragraph {
	if y < 0 {
		y = 0
	}
	if x < 0 {
		x = 0
	}
	p.scrollY = y
	p.scrollX = x
	return p
}

// Alignment sets the default horizontal alignment for lines without their own.
func (p Paragraph) Alignment(a layout.Alignment) Paragraph {
	p.alignment = a
	return p
}

// LeftAligned left-aligns the paragraph text.
func (p Paragraph) LeftAligned() Paragraph {
	return p.Alignment(layout.AlignLeft)
}

// Centered center-aligns the paragraph text.
func (p Paragraph) Centered() Paragraph {
	return p.Alignment(layout.AlignCenter)
}

// RightAligned right-aligns the paragraph text.
func (p Paragraph) RightAligned() Paragraph {
	return p.Alignment(layout.AlignRight)
}

// LineCount returns how many rows are needed to fully render at the given width,
// including vertical space taken by an optional block. Width < 1 yields 0.
func (p Paragraph) LineCount(width int) int {
	if width < 1 {
		return 0
	}

	top, bottom := 0, 0
	if p.block != nil {
		top, bottom = p.block.VerticalSpace()
	}

	var count int
	if p.wrap != nil {
		// LineCount uses widget style as grapheme base (upstream paragraph.rs).
		lines := p.styledInputLines(p.style)
		composer := newWordWrapper(lines, width, p.wrap.Trim)
		for {
			_, ok := composer.NextLine()
			if !ok {
				break
			}
			count++
		}
	} else {
		count = p.text.Height()
	}

	return count + top + bottom
}

// LineWidth returns the shortest line width that avoids wrapping/truncating any
// source line, plus horizontal space taken by an optional block.
func (p Paragraph) LineWidth() int {
	width := 0
	for i := range p.text.Lines {
		if w := p.text.Lines[i].Width(); w > width {
			width = w
		}
	}
	left, right := 0, 0
	if p.block != nil {
		left, right = p.block.HorizontalSpace()
	}
	return width + left + right
}

// Render draws the paragraph into area.
func (p Paragraph) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, p.style)
	inner := InnerIfSome(p.block, area, buf)
	p.renderParagraph(inner, buf)
}

func (p Paragraph) renderParagraph(textArea layout.Rect, buf *buffer.Buffer) {
	if textArea.IsEmpty() {
		return
	}
	buf.SetStyle(textArea, p.style)

	// Styled graphemes use text-level style (not the widget style) as the base,
	// matching upstream: line.styled_graphemes(self.text.style).
	lines := p.styledInputLines(p.text.Style)

	if p.wrap != nil {
		composer := newWordWrapper(lines, textArea.Width, p.wrap.Trim)
		// Skip vertical scroll lines.
		for range p.scrollY {
			if _, ok := composer.NextLine(); !ok {
				return
			}
		}
		renderComposerLines(composer, textArea, buf)
		return
	}

	// No wrap: skip directly to scrollY, then truncate with horizontal scroll.
	if p.scrollY > 0 {
		if p.scrollY >= len(lines) {
			return
		}
		lines = lines[p.scrollY:]
	}
	composer := newLineTruncator(lines, textArea.Width, p.scrollX)
	renderComposerLines(composer, textArea, buf)
}

func (p Paragraph) styledInputLines(base style.Style) []inputLine {
	out := make([]inputLine, len(p.text.Lines))
	for i := range p.text.Lines {
		line := p.text.Lines[i]
		align := p.alignment
		if line.Alignment != nil {
			align = *line.Alignment
		}
		out[i] = inputLine{
			Graphemes: line.StyledGraphemes(base),
			Alignment: align,
		}
	}
	return out
}

func renderComposerLines(composer lineComposer, area layout.Rect, buf *buffer.Buffer) {
	y := 0
	for {
		wrapped, ok := composer.NextLine()
		if !ok {
			return
		}
		renderWrappedLine(wrapped, area, buf, y)
		y++
		if y >= area.Height {
			return
		}
	}
}

func renderWrappedLine(wrapped wrappedLine, area layout.Rect, buf *buffer.Buffer, y int) {
	x := getLineOffset(wrapped.Width, area.Width, wrapped.Alignment)
	for i := range wrapped.Graphemes {
		g := wrapped.Graphemes[i]
		width := text.GraphemeWidth(g.Symbol)
		if width == 0 {
			continue
		}
		// Overwrite any previous character with a space rather than zero-width.
		symbol := g.Symbol
		if symbol == "" {
			symbol = " "
		}
		px := area.X + x
		py := area.Y + y
		if cell := buf.GetMut(px, py); cell != nil {
			cell.SetSymbol(symbol)
			cell.SetStyle(g.Style)
		}
		// Wide graphemes occupy multiple cells; do not Reset trailing cells —
		// Rust paragraph.rs only paints the lead cell, so the paragraph base
		// style set earlier stays on the covered columns.
		x += width
	}
}

func getLineOffset(lineWidth, textAreaWidth int, alignment layout.Alignment) int {
	switch alignment {
	case layout.AlignCenter:
		// saturating_sub: (text_area_width / 2).saturating_sub(line_width / 2)
		off := textAreaWidth/2 - lineWidth/2
		if off < 0 {
			return 0
		}
		return off
	case layout.AlignRight:
		off := textAreaWidth - lineWidth
		if off < 0 {
			return 0
		}
		return off
	default:
		return 0
	}
}
