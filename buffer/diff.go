package buffer

import (
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
)

// PositionedCell is one cell update for a backend draw call.
type PositionedCell struct {
	Position layout.Position
	Cell     Cell
}

// Modifiers that remain visible on a blank (space) cell. When a wide glyph
// carrying these is replaced by narrower content, trailing columns must be
// force-refreshed even if the buffer cells there look unchanged.
const visibleOnBlank = style.ModReversed |
	style.ModUnderlined |
	style.ModSlowBlink |
	style.ModRapidBlink |
	style.ModCrossedOut

// Diff builds the minimal sequence of positioned cells that updates the UI
// from previous to next.
//
// previous and next should share X, Y, and Width (as the terminal double-buffer
// does). When they do not, every non-skip cell of next is emitted. Heights may
// differ; only the overlapping row count is compared.
//
// Handles multi-width graphemes, VS16 emoji trailing clears, skip cells, and
// force-refresh of trailing columns when a wide glyph with visible-on-blank
// style shrinks away.
func Diff(previous, next *Buffer) []PositionedCell {
	if next == nil {
		return nil
	}
	if previous == nil {
		return emitAll(next)
	}

	// Mismatched origin/width: full redraw of next (no panic on normal input).
	if previous.Area.X != next.Area.X ||
		previous.Area.Y != next.Area.Y ||
		previous.Area.Width != next.Area.Width {
		return emitAll(next)
	}

	area := previous.Area
	if next.Area.Height < area.Height {
		area.Height = next.Area.Height
	}
	if area.Width <= 0 || area.Height <= 0 {
		return nil
	}

	prevContent := previous.Content
	nextContent := next.Content
	lenCmp := area.Area()
	if lenCmp > len(prevContent) {
		lenCmp = len(prevContent)
	}
	if lenCmp > len(nextContent) {
		lenCmp = len(nextContent)
	}

	out := make([]PositionedCell, 0, 32)
	pos := 0
	w := area.Width

	type trailingState struct {
		nextIndex int
		end       int
		force     bool
	}
	var trailing *trailingState

	posOf := func(index int) layout.Position {
		return layout.Position{
			X: index%w + area.X,
			Y: index/w + area.Y,
		}
	}

	emit := func(index int) {
		out = append(out, PositionedCell{
			Position: posOf(index),
			Cell:     nextContent[index],
		})
	}

	for {
		// Drain pending trailing cells first.
		if trailing != nil {
			for trailing.nextIndex < trailing.end {
				j := trailing.nextIndex
				cw := nextContent[j].CellWidth()
				if cw < 1 {
					cw = 1
				}
				trailing.nextIndex += cw
				if trailing.end < trailing.nextIndex {
					trailing.end = trailing.nextIndex
				}
				if trailing.end > lenCmp {
					trailing.end = lenCmp
				}
				if isSkipCell(nextContent[j]) {
					continue
				}
				if trailing.force || prevContent[j].DisplaySymbol() != nextContent[j].DisplaySymbol() {
					emit(j)
				}
			}
			pos = trailing.end
			trailing = nil
		}

		if pos >= lenCmp {
			break
		}

		i := pos
		pos++

		current := nextContent[i]
		previousCell := prevContent[i]
		optionKind := current.DiffOption.Kind()

		if isSkipCell(current) {
			continue
		}

		if optionKind == CellDiffForcedWidth {
			cellWidth := current.CellWidth()
			if cellWidth > 1 {
				pos += cellWidth - 1
			}
			if !current.Equal(previousCell) {
				emit(i)
			}
			continue
		}

		cellWidth := current.CellWidth()
		if cellWidth < 0 {
			cellWidth = 0
		}

		// AlwaysUpdate bypasses equality. Normal equal cells still advance over
		// multi-width trailing columns.
		if optionKind != CellDiffAlwaysUpdate && current.Equal(previousCell) {
			if cellWidth > 1 {
				pos += cellWidth - 1
			}
			continue
		}

		previousWidth := previousCell.CellWidth()
		if previousWidth < 0 {
			previousWidth = 0
		}

		// VS16 emoji: explicitly check trailing columns (symbol-only).
		containsVS16 := cellWidth > 1 && symbolHasVS16(current.DisplaySymbol())
		if containsVS16 {
			end := i + cellWidth
			if end > lenCmp {
				end = lenCmp
			}
			trailing = &trailingState{nextIndex: i + 1, end: end, force: false}
		} else if cellWidth > 1 {
			pos += cellWidth - 1
		} else if previousWidth > cellWidth && styleVisibleOnBlank(previousCell.StyleValue()) {
			// Wide → narrow with style visible on blanks: force-refresh trail.
			end := i + previousWidth
			if end > lenCmp {
				end = lenCmp
			}
			trailing = &trailingState{nextIndex: i + 1, end: end, force: true}
		}

		emit(i)
	}

	return out
}

// Diff is a method alias for package-level Diff(b, next).
func (b *Buffer) Diff(next *Buffer) []PositionedCell {
	return Diff(b, next)
}

func emitAll(next *Buffer) []PositionedCell {
	if next == nil || next.Area.IsEmpty() {
		return nil
	}
	w := next.Area.Width
	if w <= 0 {
		return nil
	}
	n := next.Area.Area()
	if n > len(next.Content) {
		n = len(next.Content)
	}
	out := make([]PositionedCell, 0, n)
	for i := 0; i < n; {
		cell := next.Content[i]
		if !isSkipCell(cell) {
			out = append(out, PositionedCell{
				Position: layout.Position{
					X: i%w + next.Area.X,
					Y: i/w + next.Area.Y,
				},
				Cell: cell,
			})
		}
		width := cell.CellWidth()
		if width < 1 {
			width = 1
		}
		i += width
	}
	return out
}

func isSkipCell(c Cell) bool {
	return c.DiffOption.Kind() == CellDiffSkip ||
		(c.Skip && c.DiffOption.Kind() == CellDiffNone)
}

func styleVisibleOnBlank(s style.Style) bool {
	if s.HasBG && !s.BG.IsReset() {
		return true
	}
	return s.AddModifier.Intersects(visibleOnBlank)
}

func symbolHasVS16(s string) bool {
	for _, r := range s {
		if r == '\uFE0F' {
			return true
		}
	}
	return false
}
