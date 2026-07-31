package widgets

import (
	"testing"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

func TestGaugeUnicodeEighthCell(t *testing.T) {
	// Gauge width 4, ratio 0.375 (3/8 of total width 4 = 1.5 cells => 1 full cell + 1 half cell (4/8 = BlockHalf))
	g := NewGauge().
		Ratio(0.375).
		UseUnicode(true).
		Label(text.RawSpan("")) // empty label to check raw bar cells

	area := layout.NewRect(0, 0, 4, 1)
	buf := buffer.Empty(area)

	g.Render(area, buf)

	c0, _ := buf.Get(0, 0)
	c1, _ := buf.Get(1, 0)
	c2, _ := buf.Get(2, 0)

	// Cell 0 should be full block "█"
	if c0.DisplaySymbol() != symbols.BlockFull {
		t.Errorf("cell 0 = %q, want %q", c0.DisplaySymbol(), symbols.BlockFull)
	}
	// Cell 1 should be half block "▌" (since 0.5 * 8 = 4 -> BlockHalf)
	if c1.DisplaySymbol() != symbols.BlockHalf {
		t.Errorf("cell 1 = %q, want %q", c1.DisplaySymbol(), symbols.BlockHalf)
	}
	// Cell 2 should be space " "
	if c2.DisplaySymbol() != " " {
		t.Errorf("cell 2 = %q, want space", c2.DisplaySymbol())
	}
}

func TestLineGaugeRender(t *testing.T) {
	lg := NewLineGauge().
		Ratio(0.5).
		FilledSymbol("=").
		UnfilledSymbol("-").
		Label(text.RawLine("50%"))

	// Width 10: label "50%" (3) + 1 space = 4 cols for label area, remaining 6 cols for bar
	// Ratio 0.5 of 6 = 3 filled ("="), 3 unfilled ("-")
	area := layout.NewRect(0, 0, 10, 1)
	buf := buffer.Empty(area)

	lg.Render(area, buf)

	row := getBufferRowString(buf, 0)
	wantRow := "50% ===---"
	if row != wantRow {
		t.Errorf("LineGauge row = %q, want %q", row, wantRow)
	}
}

func TestGaugeZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	NewGauge().Percent(50).UseUnicode(true).Render(zero, buf)
	NewLineGauge().Ratio(0.5).Render(zero, buf)
}
