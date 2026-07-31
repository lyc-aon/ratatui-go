package widgets

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
	"github.com/michaelkelly/ratatui-go/text"
)

// Tabs displays a horizontal set of tab titles with one optional selection.
type Tabs struct {
	block          *Block
	titles         []text.Line
	selected       *int
	style          style.Style
	highlightStyle style.Style
	divider        text.Span
	paddingLeft    text.Line
	paddingRight   text.Line
}

// NewTabs creates Tabs from titles. The slice is copied. When titles is
// non-empty the first tab (index 0) is selected by default.
func NewTabs(titles ...text.Line) Tabs {
	copied := copyTitleLines(titles)
	var selected *int
	if len(copied) > 0 {
		z := 0
		selected = &z
	}
	return Tabs{
		titles:         copied,
		selected:       selected,
		highlightStyle: style.New().WithAddModifier(style.ModReversed),
		divider:        text.RawSpan(symbols.LineVertical),
		paddingLeft:    text.RawLine(" "),
		paddingRight:   text.RawLine(" "),
	}
}

// Titles replaces the tab titles (copies the slice). Selection is clamped into
// range, or set to 0 when previously unset and titles become non-empty.
func (t Tabs) Titles(titles ...text.Line) Tabs {
	t.titles = copyTitleLines(titles)
	if len(t.titles) == 0 {
		t.selected = nil
		return t
	}
	if t.selected == nil {
		z := 0
		t.selected = &z
		return t
	}
	s := *t.selected
	if s >= len(t.titles) {
		s = len(t.titles) - 1
	}
	if s < 0 {
		s = 0
	}
	t.selected = &s
	return t
}

// Block surrounds the tabs with a block.
func (t Tabs) Block(b Block) Tabs {
	t.block = &b
	return t
}

// Select sets the selected tab index. Pass nil to deselect.
func (t Tabs) Select(index *int) Tabs {
	if index == nil {
		t.selected = nil
		return t
	}
	v := *index
	t.selected = &v
	return t
}

// SelectIndex selects tab i (convenience when index is known).
func (t Tabs) SelectIndex(i int) Tabs {
	v := i
	t.selected = &v
	return t
}

// Style sets the base style applied to the whole render area.
func (t Tabs) Style(sty style.Style) Tabs {
	t.style = sty
	return t
}

// HighlightStyle sets the style patched onto the selected tab title.
func (t Tabs) HighlightStyle(sty style.Style) Tabs {
	t.highlightStyle = sty
	return t
}

// Divider sets the span drawn between tabs (default "│").
func (t Tabs) Divider(span text.Span) Tabs {
	t.divider = span
	return t
}

// Padding sets left and right padding lines around each title (default one space each).
func (t Tabs) Padding(left, right text.Line) Tabs {
	t.paddingLeft = copyLine(left)
	t.paddingRight = copyLine(right)
	return t
}

// PaddingLeft sets only the left padding.
func (t Tabs) PaddingLeft(left text.Line) Tabs {
	t.paddingLeft = copyLine(left)
	return t
}

// PaddingRight sets only the right padding.
func (t Tabs) PaddingRight(right text.Line) Tabs {
	t.paddingRight = copyLine(right)
	return t
}

// Width returns the rendered content width (titles + padding + dividers),
// not including an optional block border.
func (t Tabs) Width() int {
	titlesWidth := 0
	for i := range t.titles {
		titlesWidth += t.titles[i].Width()
	}
	titleCount := len(t.titles)
	dividerCount := titleCount - 1
	if dividerCount < 0 {
		dividerCount = 0
	}
	return titlesWidth +
		dividerCount*t.divider.Width() +
		titleCount*t.paddingLeft.Width() +
		titleCount*t.paddingRight.Width()
}

// Render draws the tabs into area.
func (t Tabs) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, t.style)
	inner := InnerIfSome(t.block, area, buf)
	t.renderTabs(inner, buf)
}

func (t Tabs) renderTabs(tabsArea layout.Rect, buf *buffer.Buffer) {
	if tabsArea.IsEmpty() {
		return
	}

	x := tabsArea.X
	titlesLength := len(t.titles)
	for i := range t.titles {
		lastTitle := i == titlesLength-1
		remaining := tabsArea.Right() - x
		if remaining <= 0 {
			break
		}

		// Left padding.
		nx, _ := buf.SetLine(x, tabsArea.Y, t.paddingLeft, remaining)
		x = nx
		remaining = tabsArea.Right() - x
		if remaining <= 0 {
			break
		}

		// Title.
		titleStart := x
		nx, _ = buf.SetLine(x, tabsArea.Y, t.titles[i], remaining)
		if t.selected != nil && *t.selected == i {
			w := nx - titleStart
			if w < 0 {
				w = 0
			}
			buf.SetStyle(layout.Rect{
				X:      titleStart,
				Y:      tabsArea.Y,
				Width:  w,
				Height: 1,
			}, t.highlightStyle)
		}
		x = nx
		remaining = tabsArea.Right() - x
		if remaining <= 0 {
			break
		}

		// Right padding.
		nx, _ = buf.SetLine(x, tabsArea.Y, t.paddingRight, remaining)
		x = nx
		remaining = tabsArea.Right() - x
		if remaining <= 0 || lastTitle {
			break
		}

		// Divider (not after last title).
		nx, _ = buf.SetSpan(x, tabsArea.Y, t.divider, remaining)
		x = nx
	}
}

func copyTitleLines(titles []text.Line) []text.Line {
	if len(titles) == 0 {
		return nil
	}
	out := make([]text.Line, len(titles))
	for i := range titles {
		out[i] = copyLine(titles[i])
	}
	return out
}
