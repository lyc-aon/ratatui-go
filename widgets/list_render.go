package widgets

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

// Render draws the list with a fresh default ListState (no selection).
func (l List) Render(area layout.Rect, buf *buffer.Buffer) {
	state := NewListState()
	l.RenderStateful(area, buf, &state)
}

// RenderStateful draws the list, repairing state offset/selection so the
// selected item stays visible within the inner area.
func (l List) RenderStateful(area layout.Rect, buf *buffer.Buffer, state *ListState) {
	if buf == nil {
		return
	}
	if state == nil {
		tmp := NewListState()
		state = &tmp
	}

	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}

	buf.SetStyle(area, l.style)
	listArea := InnerIfSome(l.block, area, buf)
	if listArea.IsEmpty() {
		return
	}

	if len(l.items) == 0 {
		state.Select(nil)
		return
	}

	// Clamp selection that sits past the last item.
	if state.selected != nil && *state.selected >= len(l.items) {
		last := len(l.items) - 1
		if last < 0 {
			state.Select(nil)
			return
		}
		state.Select(&last)
	}

	listHeight := listArea.Height
	firstVisible, lastVisible := l.getItemsBounds(state.selected, state.offset, listHeight)
	// Important: publish the repaired window start back into state.
	state.offset = firstVisible

	highlightSymbol := text.Line{}
	if l.highlightSymbol != nil {
		highlightSymbol = *l.highlightSymbol
	}
	highlightSymbolWidth := highlightSymbol.Width()
	emptySymbol := text.RawLine(strings.Repeat(" ", highlightSymbolWidth))

	selectionSpacing := l.highlightSpacing.ShouldAdd(state.selected != nil)

	currentHeight := 0
	nVisible := lastVisible - firstVisible
	if nVisible < 0 {
		nVisible = 0
	}
	end := firstVisible + nVisible
	if end > len(l.items) {
		end = len(l.items)
	}
	for i := firstVisible; i < end; i++ {
		item := l.items[i]
		itemHeight := item.Height()
		var x, y int
		if l.direction == ListBottomToTop {
			currentHeight += itemHeight
			x = listArea.X
			y = listArea.Bottom() - currentHeight
		} else {
			x = listArea.X
			y = listArea.Y + currentHeight
			currentHeight += itemHeight
		}

		rowArea := layout.NewRect(x, y, listArea.Width, itemHeight)
		itemStyle := l.style.Patch(item.Style)
		buf.SetStyle(rowArea, itemStyle)

		isSelected := state.selected != nil && *state.selected == i

		itemArea := rowArea
		if selectionSpacing {
			itemArea = layout.Rect{
				X:      rowArea.X + highlightSymbolWidth,
				Y:      rowArea.Y,
				Width:  listMax(0, rowArea.Width-highlightSymbolWidth),
				Height: rowArea.Height,
			}
		}
		listPaintText(item.Content, itemArea, buf)

		// Highlight style patches the whole row (including symbol column) after content.
		if isSelected {
			buf.SetStyle(rowArea, l.highlightStyle)
		}

		if selectionSpacing {
			contentH := item.Content.Height()
			for j := range contentH {
				var line text.Line
				if isSelected && (j == 0 || l.repeatHighlightSymbol) {
					line = highlightSymbol
				} else {
					line = emptySymbol
				}
				highlightArea := layout.NewRect(x, y+j, highlightSymbolWidth, 1)
				listPaintLine(line, highlightArea, buf)
			}
		}
	}
}

// getItemsBounds returns [first, last) visible item indices for the window.
//
// offset is clamped to the last item. The window grows/shifts so the selected
// index (with scroll padding) stays on screen when possible.
func (l List) getItemsBounds(selected *int, offset int, maxHeight int) (int, int) {
	n := len(l.items)
	if n == 0 {
		return 0, 0
	}
	if maxHeight < 0 {
		maxHeight = 0
	}

	if offset > n-1 {
		offset = n - 1
	}
	if offset < 0 {
		offset = 0
	}

	firstVisible := offset
	lastVisible := offset
	heightFromOffset := 0

	for _, item := range l.items[offset:] {
		h := item.Height()
		if heightFromOffset+h > maxHeight {
			break
		}
		heightFromOffset += h
		lastVisible++
	}

	indexToDisplay := offset
	if padded, ok := l.applyScrollPaddingToSelectedIndex(selected, maxHeight, firstVisible, lastVisible); ok {
		indexToDisplay = padded
	}

	// Push window down until indexToDisplay is included.
	for indexToDisplay >= lastVisible {
		if lastVisible >= n {
			break
		}
		heightFromOffset += l.items[lastVisible].Height()
		lastVisible++

		for heightFromOffset > maxHeight && firstVisible < lastVisible {
			heightFromOffset -= l.items[firstVisible].Height()
			if heightFromOffset < 0 {
				heightFromOffset = 0
			}
			firstVisible++
		}
	}

	// Pull window up until indexToDisplay is included.
	for indexToDisplay < firstVisible {
		firstVisible--
		heightFromOffset += l.items[firstVisible].Height()

		for heightFromOffset > maxHeight && lastVisible > firstVisible {
			lastVisible--
			heightFromOffset -= l.items[lastVisible].Height()
			if heightFromOffset < 0 {
				heightFromOffset = 0
			}
		}
	}

	return firstVisible, lastVisible
}

// applyScrollPaddingToSelectedIndex shrinks padding until the neighborhood of
// the selection fits in maxHeight, then returns the index that must stay in
// view (selection ± padding toward the nearest edge).
func (l List) applyScrollPaddingToSelectedIndex(
	selected *int,
	maxHeight int,
	firstVisible int,
	lastVisible int,
) (int, bool) {
	if selected == nil {
		return 0, false
	}
	lastValid := len(l.items) - 1
	if lastValid < 0 {
		return 0, false
	}
	sel := *selected
	if sel > lastValid {
		sel = lastValid
	}
	if sel < 0 {
		sel = 0
	}

	scrollPadding := l.scrollPadding
	if scrollPadding < 0 {
		scrollPadding = 0
	}
	for scrollPadding > 0 {
		heightAround := 0
		start := sel - scrollPadding
		if start < 0 {
			start = 0
		}
		end := sel + scrollPadding
		if end > lastValid {
			end = lastValid
		}
		for index := start; index <= end; index++ {
			heightAround += l.items[index].Height()
		}
		if heightAround <= maxHeight {
			break
		}
		scrollPadding--
	}

	out := sel
	upper := sel + scrollPadding
	if upper > lastValid {
		upper = lastValid
	}
	if upper >= lastVisible {
		out = sel + scrollPadding
	} else if sel-scrollPadding < firstVisible {
		out = sel - scrollPadding
		if out < 0 {
			out = 0
		}
	}
	if out > lastValid {
		out = lastValid
	}
	if out < 0 {
		out = 0
	}
	return out, true
}

// listPaintText draws a Text into area (intersected with the buffer), matching
// Ratatui's Text widget: base style, then each line with inherited alignment.
func listPaintText(t text.Text, area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, t.Style)
	maxLines := area.Height
	if maxLines > len(t.Lines) {
		maxLines = len(t.Lines)
	}
	for i := range maxLines {
		row := layout.Rect{X: area.X, Y: area.Y + i, Width: area.Width, Height: 1}
		line := t.Lines[i]
		if line.Alignment == nil && t.Alignment != nil {
			a := *t.Alignment
			line.Alignment = &a
		}
		lineStyle := t.Style.Patch(line.Style)
		listPaintLineStyled(line, lineStyle, row, buf)
	}
}

// listPaintLine draws a single Line into area.
func listPaintLine(line text.Line, area layout.Rect, buf *buffer.Buffer) {
	listPaintLineStyled(line, line.Style, area, buf)
}

// listPaintLineStyled draws line using lineStyle for the row base patch.
func listPaintLineStyled(line text.Line, lineStyle style.Style, area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	// Line widget always paints a single row.
	area.Height = 1
	if area.IsEmpty() {
		return
	}

	// Rust Line::render returns before set_style when width is 0, so a parent
	// Text-level style already on the row is preserved.
	if line.Width() == 0 {
		return
	}

	buf.SetStyle(area, lineStyle)

	// RenderData wants the line's own Alignment field; style is applied above.
	drawLine := line
	drawLine.Style = style.Style{}
	spans, _, leftPad := drawLine.RenderData(area.Width)
	x := area.X + leftPad
	if x < area.X {
		x = area.X
	}
	remaining := area.Right() - x
	if remaining < 0 {
		remaining = 0
	}
	for i := range spans {
		if remaining <= 0 {
			break
		}
		// Span style patches onto cells that already carry lineStyle.
		nx, _ := buf.SetStringN(x, area.Y, spans[i].Content, remaining, spans[i].Style)
		used := nx - x
		if used < 0 {
			used = 0
		}
		x = nx
		remaining -= used
	}
}

func listMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
