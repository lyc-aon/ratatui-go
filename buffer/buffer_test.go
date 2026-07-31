package buffer

import (
	"testing"

	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

func TestWideGraphemeWrite(t *testing.T) {
	b := Empty(layout.NewRect(0, 0, 5, 1))
	endX, endY := b.SetStringN(0, 0, "😀", 5, style.New())

	if endX != 2 || endY != 0 {
		t.Errorf("SetStringN returned (%d, %d), want (2, 0)", endX, endY)
	}

	c0, ok0 := b.Get(0, 0)
	if !ok0 || c0.DisplaySymbol() != "😀" {
		t.Errorf("cell (0,0) = %q, ok = %v; want %q, true", c0.DisplaySymbol(), ok0, "😀")
	}

	c1, ok1 := b.Get(1, 0)
	if !ok1 || c1.DisplaySymbol() != " " {
		t.Errorf("cell (1,0) symbol = %q, ok = %v; want space, true", c1.DisplaySymbol(), ok1)
	}
}

func TestZeroValueCellEqualsCanonicalEmptyCell(t *testing.T) {
	var zero Cell
	canonical := NewCell()
	if !zero.Equal(canonical) || !canonical.Equal(zero) {
		t.Fatalf("zero and canonical empty cells differ: zero=%#v canonical=%#v", zero, canonical)
	}
	if got := zero.StyleValue(); got != canonical.Style {
		t.Fatalf("zero StyleValue = %#v, want %#v", got, canonical.Style)
	}
}

func TestBufferDiff(t *testing.T) {
	prev := Empty(layout.NewRect(0, 0, 3, 1))
	next := Empty(layout.NewRect(0, 0, 3, 1))
	next.SetString(0, 0, "A", style.New())

	diff := prev.Diff(next)
	if len(diff) != 1 {
		t.Fatalf("diff length = %d, want 1", len(diff))
	}
	if diff[0].Position != (layout.Position{X: 0, Y: 0}) {
		t.Errorf("diff position = %+v, want (0, 0)", diff[0].Position)
	}
	if diff[0].Cell.DisplaySymbol() != "A" {
		t.Errorf("diff cell symbol = %q, want %q", diff[0].Cell.DisplaySymbol(), "A")
	}
}

func TestBufferDiffShrink(t *testing.T) {
	// Prev has a wide grapheme (width 2) with reversed style at (0,0), covering (0,0) and (1,0).
	prev := Empty(layout.NewRect(0, 0, 3, 1))
	prev.SetString(0, 0, "😀", style.FromModifier(style.ModReversed))

	// Next replaces (0,0) with a single-width grapheme 'A'.
	next := Empty(layout.NewRect(0, 0, 3, 1))
	next.SetString(0, 0, "A", style.New())

	diff := prev.Diff(next)

	// Diff must contain (0,0) for 'A' AND an update for column 1 to force-refresh/clear the trailing wide cell.
	foundCol0 := false
	foundCol1 := false

	for _, pc := range diff {
		if pc.Position.Y == 0 {
			if pc.Position.X == 0 && pc.Cell.DisplaySymbol() == "A" {
				foundCol0 = true
			}
			if pc.Position.X == 1 {
				foundCol1 = true
			}
		}
	}

	if !foundCol0 {
		t.Errorf("diff missing column 0 update with 'A'")
	}
	if !foundCol1 {
		t.Errorf("diff missing column 1 update to clear wide grapheme continuation cell")
	}
}

func TestBufferNonzeroOrigin(t *testing.T) {
	area := layout.NewRect(10, 20, 5, 2)
	b := Empty(area)

	b.SetString(10, 20, "X", style.New())

	c, ok := b.Get(10, 20)
	if !ok || c.DisplaySymbol() != "X" {
		t.Errorf("Get(10, 20) = %q, ok = %v; want %q, true", c.DisplaySymbol(), ok, "X")
	}

	_, okOut := b.Get(0, 0)
	if okOut {
		t.Errorf("Get(0, 0) returned ok = true for position outside non-zero origin area")
	}

	prev := Empty(area)
	next := Empty(area)
	next.SetString(10, 20, "X", style.New())

	diff := prev.Diff(next)
	if len(diff) != 1 {
		t.Fatalf("diff length = %d, want 1", len(diff))
	}
	if diff[0].Position != (layout.Position{X: 10, Y: 20}) {
		t.Errorf("diff position = %+v, want (10, 20)", diff[0].Position)
	}
}

func TestBufferDiffOptions(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		prev := WithStrings("abc")
		next := WithStrings("xyz")
		next.GetMut(1, 0).SetDiffOption(DiffSkip)

		diff := Diff(prev, next)
		if len(diff) != 2 || diff[0].Cell.DisplaySymbol() != "x" || diff[1].Cell.DisplaySymbol() != "z" {
			t.Fatalf("Diff symbols = %q, %q (len %d), want x, z", diff[0].Cell.DisplaySymbol(), diff[1].Cell.DisplaySymbol(), len(diff))
		}
	})

	t.Run("always update", func(t *testing.T) {
		prev := WithStrings("abc")
		next := WithStrings("abc")
		prev.GetMut(1, 0).SetDiffOption(DiffAlwaysUpdate)
		next.GetMut(1, 0).SetDiffOption(DiffAlwaysUpdate)

		diff := Diff(prev, next)
		if len(diff) != 1 || diff[0].Position != (layout.Position{X: 1, Y: 0}) {
			t.Fatalf("Diff = %#v, want one update at (1,0)", diff)
		}
	})

	t.Run("forced width", func(t *testing.T) {
		prev := WithStrings("abcd")
		next := WithStrings("xbcd")
		next.GetMut(0, 0).SetDiffOption(ForcedWidth(2))

		diff := Diff(prev, next)
		if len(diff) != 1 || diff[0].Cell.DisplaySymbol() != "x" {
			t.Fatalf("Diff = %#v, want only x", diff)
		}
		if got := next.GetMut(0, 0).CellWidth(); got != 2 {
			t.Fatalf("CellWidth = %d, want 2", got)
		}
	})

	t.Run("forced width overrides deprecated skip", func(t *testing.T) {
		prev := WithStrings("abcd")
		next := WithStrings("xbcd")
		next.GetMut(0, 0).SetSkip(true).SetDiffOption(ForcedWidth(2))

		diff := Diff(prev, next)
		if len(diff) != 1 || diff[0].Cell.DisplaySymbol() != "x" {
			t.Fatalf("Diff = %#v, want only x", diff)
		}
	})
}

func TestDiffNilPreviousSkipsWideTrailingCell(t *testing.T) {
	next := WithStrings("😀x")
	diff := Diff(nil, next)
	if len(diff) != 2 {
		t.Fatalf("Diff length = %d, want 2", len(diff))
	}
	if diff[0].Position.X != 0 || diff[0].Cell.DisplaySymbol() != "😀" ||
		diff[1].Position.X != 2 || diff[1].Cell.DisplaySymbol() != "x" {
		t.Fatalf("Diff = %#v, want emoji at 0 and x at 2", diff)
	}
}

func TestWithLinesUsesWidestLine(t *testing.T) {
	b := WithLines(text.RawLine("a"), text.RawLine("界x"))
	if b.Area != (layout.Rect{Width: 3, Height: 2}) {
		t.Fatalf("Area = %#v, want 3x2", b.Area)
	}
	if got, _ := b.Get(2, 1); got.DisplaySymbol() != "x" {
		t.Fatalf("cell (2,1) = %q, want x", got.DisplaySymbol())
	}
}

func TestCellMergeSymbolDistinguishesImplicitAndExplicitSpace(t *testing.T) {
	implicit := NewCell()
	implicit.MergeSymbol("┏", symbols.MergeStrategyExact)
	if implicit.DisplaySymbol() != "┏" {
		t.Fatalf("implicit empty merge = %q, want ┏", implicit.DisplaySymbol())
	}

	explicit := NewCellWithSymbol(" ")
	explicit.MergeSymbol("┏", symbols.MergeStrategyExact)
	if explicit.DisplaySymbol() != " " {
		t.Fatalf("explicit space merge = %q, want space", explicit.DisplaySymbol())
	}

	merged := NewCellWithSymbol("┘")
	merged.MergeSymbol("┏", symbols.MergeStrategyExact)
	if merged.DisplaySymbol() != "╆" {
		t.Fatalf("box merge = %q, want ╆", merged.DisplaySymbol())
	}
}
