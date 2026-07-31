package buffer

import (
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
	"github.com/michaelkelly/ratatui-go/text"
)

// CellDiffKind identifies how a Cell participates in buffer diffing.
type CellDiffKind uint8

const (
	// CellDiffNone uses normal equality and symbol-width handling.
	CellDiffNone CellDiffKind = iota
	// CellDiffSkip prevents the cell from being emitted.
	CellDiffSkip
	// CellDiffAlwaysUpdate emits the cell even when it equals the prior cell.
	CellDiffAlwaysUpdate
	// CellDiffForcedWidth uses an explicit cell width instead of symbol width.
	CellDiffForcedWidth
)

// CellDiffOption controls special buffer-diff behavior for a Cell.
//
// The low byte stores the kind; forced width occupies the remaining bits.
// The zero value is normal diffing.
type CellDiffOption uint32

// Non-width cell diff options.
const (
	DiffNone CellDiffOption = iota
	DiffSkip
	DiffAlwaysUpdate
)

// ForcedWidth returns a diff option that treats a cell as width terminal
// columns. Ratatui stores this as a non-zero uint16, so the same bounds apply.
func ForcedWidth(width int) CellDiffOption {
	if width <= 0 || width > 65535 {
		panic("buffer: forced cell width must be in 1..65535")
	}
	return CellDiffOption(uint32(width)<<8 | uint32(CellDiffForcedWidth))
}

// Kind reports the option kind.
func (o CellDiffOption) Kind() CellDiffKind {
	return CellDiffKind(uint32(o) & 0xff)
}

// Width reports the forced width and whether this is a forced-width option.
func (o CellDiffOption) Width() (int, bool) {
	if o.Kind() != CellDiffForcedWidth {
		return 0, false
	}
	return int(uint32(o) >> 8), true
}

// Cell is one terminal cell: a grapheme cluster, resolved style, and diff option.
//
// The zero Cell displays a single space (empty Symbol is treated as " ").
// NewCell returns the canonical empty cell with Reset colors.
type Cell struct {
	Symbol     string
	Style      style.Style
	DiffOption CellDiffOption
	// Skip is retained for compatibility. It applies only when DiffOption is
	// DiffNone; use SetDiffOption(DiffSkip) in new code.
	Skip bool
}

// NewCell returns an empty cell: implicit space symbol, Reset colors, no
// modifiers, and normal diffing.
func NewCell() Cell {
	return Cell{
		Style: resolvedResetStyle(),
	}
}

// NewCellWithSymbol returns a cell with the given symbol and Reset style.
func NewCellWithSymbol(symbol string) Cell {
	c := NewCell()
	c.Symbol = symbol
	return c
}

// DisplaySymbol returns the symbol drawn for this cell.
// Empty Symbol is treated as a single space so zero cells match NewCell.
func (c Cell) DisplaySymbol() string {
	if c.Symbol == "" {
		return " "
	}
	return c.Symbol
}

// Reset restores the cell to the empty state (space, Reset style, not skipped).
func (c *Cell) Reset() {
	*c = NewCell()
}

// SetSymbol replaces the cell symbol.
func (c *Cell) SetSymbol(symbol string) *Cell {
	c.Symbol = symbol
	return c
}

// SetChar replaces the cell symbol with one Unicode scalar value.
func (c *Cell) SetChar(ch rune) *Cell {
	c.Symbol = string(ch)
	return c
}

// MergeSymbol merges a box-drawing symbol into this cell. An implicit empty
// symbol takes the new value directly; an explicitly set space participates in
// the selected merge strategy, matching Ratatui's Option-backed cell symbol.
func (c *Cell) MergeSymbol(symbol string, strategy symbols.MergeStrategy) *Cell {
	if c.Symbol == "" {
		c.Symbol = symbol
		return c
	}
	c.Symbol = strategy.Merge(c.Symbol, symbol)
	return c
}

// SetFG replaces the resolved foreground color.
func (c *Cell) SetFG(color style.Color) *Cell {
	c.Style.FG = color
	c.Style.HasFG = true
	return c
}

// SetBG replaces the resolved background color.
func (c *Cell) SetBG(color style.Color) *Cell {
	c.Style.BG = color
	c.Style.HasBG = true
	return c
}

// SetUnderlineColor replaces the resolved underline color.
func (c *Cell) SetUnderlineColor(color style.Color) *Cell {
	c.Style.UnderlineColor = color
	c.Style.HasUnderlineColor = true
	return c
}

// StyleValue returns the fully resolved cell style. A zero-value Cell resolves
// unset colors to Reset, matching Ratatui's canonical empty cell.
func (c Cell) StyleValue() style.Style {
	s := c.Style
	if !s.HasFG {
		s.FG, s.HasFG = style.Reset, true
	}
	if !s.HasBG {
		s.BG, s.HasBG = style.Reset, true
	}
	if !s.HasUnderlineColor {
		s.UnderlineColor, s.HasUnderlineColor = style.Reset, true
	}
	s.AddModifier = s.Modifiers()
	s.SubModifier = 0
	return s
}

// SetStyle applies an incremental style patch onto the cell's resolved style.
//
// Matches Ratatui Cell::set_style: present colors overwrite; modifiers are
// inserted/removed on the resolved modifier set (SubModifier stays empty on
// the stored cell style).
func (c *Cell) SetStyle(s style.Style) *Cell {
	if s.HasFG {
		c.Style.FG = s.FG
		c.Style.HasFG = true
	}
	if s.HasBG {
		c.Style.BG = s.BG
		c.Style.HasBG = true
	}
	if s.HasUnderlineColor {
		c.Style.UnderlineColor = s.UnderlineColor
		c.Style.HasUnderlineColor = true
	}
	c.Style.AddModifier = c.Style.AddModifier.Insert(s.AddModifier).Remove(s.SubModifier)
	c.Style.SubModifier = 0
	// Resolved cells always carry present colors so backends see concrete values.
	if !c.Style.HasFG {
		c.Style.FG = style.Reset
		c.Style.HasFG = true
	}
	if !c.Style.HasBG {
		c.Style.BG = style.Reset
		c.Style.HasBG = true
	}
	if !c.Style.HasUnderlineColor {
		c.Style.UnderlineColor = style.Reset
		c.Style.HasUnderlineColor = true
	}
	return c
}

// SetSkip sets whether diffing should ignore this cell.
func (c *Cell) SetSkip(skip bool) *Cell {
	c.Skip = skip
	return c
}

// SetDiffOption sets special handling for this cell during buffer diffing.
func (c *Cell) SetDiffOption(option CellDiffOption) *Cell {
	c.DiffOption = option
	return c
}

// Equal reports whether two cells have equal content and diff directives.
// Empty Symbol and " " are equal.
func (c Cell) Equal(other Cell) bool {
	return c.DisplaySymbol() == other.DisplaySymbol() &&
		c.StyleValue() == other.StyleValue() &&
		c.DiffOption == other.DiffOption &&
		c.Skip == other.Skip
}

// CellWidth returns the terminal column width of this cell.
func (c Cell) CellWidth() int {
	if width, ok := c.DiffOption.Width(); ok {
		return width
	}
	return text.GraphemeWidth(c.DisplaySymbol())
}

// resolvedResetStyle is the concrete style stored on empty cells.
func resolvedResetStyle() style.Style {
	return style.Style{
		FG:                style.Reset,
		BG:                style.Reset,
		UnderlineColor:    style.Reset,
		HasFG:             true,
		HasBG:             true,
		HasUnderlineColor: true,
		AddModifier:       0,
		SubModifier:       0,
	}
}
