package widgets

import (
	"testing"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

func TestSparklineAbsentAndDirection(t *testing.T) {
	// 3 bars: val=0 (level 0), absent, val=max (level 8 full)
	sp := NewSparkline().
		Data(NewSparklineBar(0), AbsentSparklineBar(), NewSparklineBar(10)).
		Max(10).
		AbsentValueSymbol("?").
		Direction(RenderLeftToRight)

	area := layout.NewRect(0, 0, 3, 1)
	buf := buffer.Empty(area)

	sp.Render(area, buf)

	c0, _ := buf.Get(0, 0)
	c1, _ := buf.Get(1, 0)
	c2, _ := buf.Get(2, 0)

	// Col 0: val 0 -> " " (empty level)
	if c0.DisplaySymbol() != " " {
		t.Errorf("col 0 = %q, want space", c0.DisplaySymbol())
	}
	// Col 1: absent -> "?"
	if c1.DisplaySymbol() != "?" {
		t.Errorf("col 1 absent = %q, want %q", c1.DisplaySymbol(), "?")
	}
	// Col 2: val 10 -> full block "█"
	if c2.DisplaySymbol() != symbols.BlockFull {
		t.Errorf("col 2 max = %q, want %q", c2.DisplaySymbol(), symbols.BlockFull)
	}

	// Test RightToLeft direction
	spRTL := sp.Direction(RenderRightToLeft)
	bufRTL := buffer.Empty(area)
	spRTL.Render(area, bufRTL)

	// Col 0 (leftmost) in RightToLeft shows data[0] or data[2]?
	// RightToLeft places first data value on the right (col 2)
	c0RTL, _ := bufRTL.Get(0, 0)
	c2RTL, _ := bufRTL.Get(2, 0)

	if c2RTL.DisplaySymbol() != " " {
		t.Errorf("RTL col 2 = %q, want space", c2RTL.DisplaySymbol())
	}
	if c0RTL.DisplaySymbol() != symbols.BlockFull {
		t.Errorf("RTL col 0 = %q, want full block", c0RTL.DisplaySymbol())
	}
}

func TestBarChartLabelsAndScaling(t *testing.T) {
	bar1 := BarWithLabel(text.RawLine("B1"), 5)
	bar2 := BarWithLabel(text.RawLine("B2"), 10)

	bc := VerticalBarChart([]Bar{bar1, bar2}).
		BarWidth(2).
		BarGap(1).
		Max(10)

	// Height 3: row 2 is label row, row 1 is lower bar, row 0 is upper bar
	area := layout.NewRect(0, 0, 5, 3)
	buf := buffer.Empty(area)

	bc.Render(area, buf)

	// Label row (y=2): "B1 B2"
	lblRow := getBufferRowString(buf, 2)
	if lblRow != "B1 B2" {
		t.Errorf("BarChart label row 2 = %q, want %q", lblRow, "B1 B2")
	}

	// Bar 2 (max 10) at cols 3..4 should be full height (rows 0 and 1)
	cBar2Top, _ := buf.Get(3, 0)
	if cBar2Top.DisplaySymbol() == " " {
		t.Errorf("Bar 2 top cell should not be space")
	}
}

func TestBarchartSparklineZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	NewSparkline().
		Data(NewSparklineBar(5), AbsentSparklineBar()).
		Render(zero, buf)

	VerticalBarChart([]Bar{BarWithLabel(text.RawLine("X"), 1)}).
		BarWidth(0).
		Render(zero, buf)
}

func TestBarchartValueLongerThanHorizontalBar(t *testing.T) {
	// Horizontal bar with text value longer than the bar length
	bar := BarWithLabel(text.RawLine("B1"), 2).WithTextValue("LONG_TEXT_VALUE")
	chart := HorizontalBarChart([]Bar{bar}).
		BarWidth(1).
		Max(10)

	area := layout.NewRect(0, 0, 20, 1)
	buf := buffer.Empty(area)
	chart.Render(area, buf)

	// Label size ("B1") = 2, margin = 1, barsArea starts at X=3
	// Text value "LONG_TEXT_VALUE" should be rendered starting in barsArea
	r0 := getBufferRowString(buf, 0)
	if r0 == "" {
		t.Errorf("rendered horizontal barchart row 0 is empty")
	}
}

func TestBarchartMaxBelowData(t *testing.T) {
	// Max set lower than actual bar data value (Max=5, Bar=10)
	bar := BarWithLabel(text.RawLine("B"), 10)
	chart := VerticalBarChart([]Bar{bar}).Max(5)

	area := layout.NewRect(0, 0, 3, 3)
	buf := buffer.Empty(area)
	chart.Render(area, buf)

	// Value 10 with max 5 should saturate to full bar height without overflow or crash
	cTop, _ := buf.Get(0, 0)
	if cTop.DisplaySymbol() == " " {
		t.Errorf("top cell of saturated bar should not be empty space")
	}
}

func TestBarHorizontalValueByteBoundaryOverflow(t *testing.T) {
	// Rust barchart/bar.rs 0.30.2: split value text on UTF-8 *byte* boundary
	// at-or-before bar_length, offset x by first.len() bytes, max_width =
	// area.width - first.len(). Go clamps negative remaining width.
	valStyle := style.New().WithFG(style.Red)
	barStyle := style.New().WithFG(style.Green)
	b := NewBar(1).
		WithTextValue("ABC").
		WithValueStyle(valStyle).
		WithStyle(barStyle)

	area := layout.NewRect(0, 0, 5, 1)
	buf := buffer.Empty(area)
	// barLength=1: first part "A" with value style; overflow "BC" at x=1 with bar style.
	b.renderValueWithDifferentStyles(buf, area, 1, style.New(), style.New())

	c0, ok := buf.Get(0, 0)
	if !ok || c0.DisplaySymbol() != "A" {
		t.Fatalf("cell0 = %q, want A", c0.DisplaySymbol())
	}
	fg0, set0 := c0.Style.Foreground()
	if !set0 || fg0 != style.Red {
		t.Fatalf("cell0 fg = (%v, %v), want Red value style", fg0, set0)
	}
	c1, ok := buf.Get(1, 0)
	if !ok || c1.DisplaySymbol() != "B" {
		t.Fatalf("cell1 = %q, want B (byte-offset overflow start)", c1.DisplaySymbol())
	}
	fg1, set1 := c1.Style.Foreground()
	if !set1 || fg1 != style.Green {
		t.Fatalf("cell1 fg = (%v, %v), want Green bar style", fg1, set1)
	}
	c2, ok := buf.Get(2, 0)
	if !ok || c2.DisplaySymbol() != "C" {
		t.Fatalf("cell2 = %q, want C", c2.DisplaySymbol())
	}

	// Multi-byte: "éAB" (é = 2 bytes). barLength=1 includes char at i=0 →
	// splitAt=2, first="é", second="AB" painted at x+2 with bar style.
	buf2 := buffer.Empty(area)
	b2 := NewBar(1).WithTextValue("éAB").WithValueStyle(valStyle).WithStyle(barStyle)
	b2.renderValueWithDifferentStyles(buf2, area, 1, style.New(), style.New())
	lead, ok := buf2.Get(0, 0)
	if !ok || lead.DisplaySymbol() != "é" {
		t.Fatalf("multibyte cell0 = %q, want é", lead.DisplaySymbol())
	}
	overflow, ok := buf2.Get(2, 0)
	if !ok || overflow.DisplaySymbol() != "A" {
		t.Fatalf("multibyte overflow at x=firstLen(2) = %q, want A", overflow.DisplaySymbol())
	}
	ofg, oset := overflow.Style.Foreground()
	if !oset || ofg != style.Green {
		t.Fatalf("multibyte overflow fg = (%v, %v), want Green bar style", ofg, oset)
	}

	// firstLen > area.Width must clamp remaining width (no panic).
	// barLength=5 → first="ABCDE" (5 bytes), area.Width=1 → maxW clamped to 0.
	buf3 := buffer.Empty(layout.NewRect(0, 0, 1, 1))
	b3 := NewBar(1).WithTextValue("ABCDEFGH").WithValueStyle(valStyle).WithStyle(barStyle)
	b3.renderValueWithDifferentStyles(buf3, layout.NewRect(0, 0, 1, 1), 5, style.New(), style.New())
}
