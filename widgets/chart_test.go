package widgets

import (
	"testing"

	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

func TestChartRenderAxisDatasetLegend(t *testing.T) {
	ds := NewDataset().
		NameString("Data1").
		Data([][2]float64{{0, 0}, {5, 5}, {10, 10}}).
		GraphType(GraphLine)

	xAxis := NewAxis().
		Title(text.RawLine("X-Axis")).
		Bounds([2]float64{0, 10}).
		Labels(text.RawLine("0"), text.RawLine("10"))

	yAxis := NewAxis().
		Title(text.RawLine("Y-Axis")).
		Bounds([2]float64{0, 10}).
		Labels(text.RawLine("0"), text.RawLine("10"))

	legendPos := LegendTopRight
	ch := NewChart(ds).
		XAxis(xAxis).
		YAxis(yAxis).
		LegendPosition(&legendPos)

	area := layout.NewRect(0, 0, 30, 10)
	buf := buffer.Empty(area)

	ch.Render(area, buf)
}

func TestChartZeroAndMinimalAreaSmoke(t *testing.T) {
	ds := NewDataset().Data([][2]float64{{1, 1}}).NameString("S")
	ch := NewChart(ds).
		XAxis(NewAxis().Bounds([2]float64{0, 10})).
		YAxis(NewAxis().Bounds([2]float64{0, 10}))

	// Zero area
	zero := layout.NewRect(0, 0, 0, 0)
	bufZero := buffer.Empty(zero)
	ch.Render(zero, bufZero)

	// Minimal area 1x1
	minArea := layout.NewRect(0, 0, 1, 1)
	bufMin := buffer.Empty(minArea)
	ch.Render(minArea, bufMin)
}

func TestChartLegendStylesWholeNameRow(t *testing.T) {
	legendPos := LegendTopRight
	ch := NewChart(
		NewDataset().NameString("A").Style(style.New().WithBG(style.Blue)),
		NewDataset().NameString("Longer"),
	).
		LegendPosition(&legendPos).
		HiddenLegendConstraints(layout.Min(0), layout.Min(0))
	area := layout.NewRect(0, 0, 30, 10)
	buf := buffer.Empty(area)

	ch.Render(area, buf)
	resolved, ok := ch.resolveLayout(area)
	if !ok || resolved.legendArea == nil {
		t.Fatal("legend was not laid out")
	}
	legend := *resolved.legendArea
	cell, ok := buf.Get(legend.Right()-2, legend.Y+1)
	if !ok {
		t.Fatal("legend name-row trailing cell missing")
	}
	bg, set := cell.Style.Background()
	if !set || bg != style.Blue {
		t.Fatalf("legend name-row background = (%v, %v), want Blue", bg, set)
	}
}

func TestChartEdgeStates(t *testing.T) {
	// Chart with empty datasets
	chEmpty := NewChart()
	area := layout.NewRect(0, 0, 20, 10)
	buf := buffer.Empty(area)
	chEmpty.Render(area, buf)

	// Chart with axis bounds min == max
	ds := NewDataset().Data([][2]float64{{5, 5}}).NameString("D")
	chEqualBounds := NewChart(ds).
		XAxis(NewAxis().Bounds([2]float64{5, 5})).
		YAxis(NewAxis().Bounds([2]float64{5, 5}))
	bufEqual := buffer.Empty(area)
	chEqualBounds.Render(area, bufEqual)

	// Legend positions: TopLeft, BottomRight, BottomLeft
	positions := []LegendPosition{
		LegendTopLeft,
		LegendBottomRight,
		LegendBottomLeft,
		LegendLeft,
		LegendRight,
		LegendTop,
		LegendBottom,
	}
	for _, pos := range positions {
		p := pos
		chLegend := NewChart(ds).LegendPosition(&p)
		bufL := buffer.Empty(area)
		chLegend.Render(area, bufL)
	}
}
