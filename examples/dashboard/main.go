package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lyc-aon/ratatui-go/backend"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/terminal"
	"github.com/lyc-aon/ratatui-go/text"
	"github.com/lyc-aon/ratatui-go/widgets"
)

func main() {
	if err := realMain(); err != nil {
		fatal(err)
	}
}

func realMain() error {
	width := flag.Int("width", 86, "terminal width")
	height := flag.Int("height", 28, "terminal height")
	frames := flag.Int("frames", 2, "frames to render")
	ansi := flag.Bool("ansi", false, "write a real ANSI terminal frame")
	flag.Parse()

	if *width < 1 || *height < 1 || *frames < 1 {
		return fmt.Errorf("width, height, and frames must be positive")
	}

	if *ansi {
		b := backend.NewANSIBackend(os.Stdout, *width, *height)
		if err := b.Setup(); err != nil {
			return err
		}
		runErr := run(b, *frames)
		restoreErr := b.Restore()
		if runErr != nil {
			return runErr
		}
		return restoreErr
	}

	b := backend.NewTestBackend(*width, *height)
	if err := run(b, *frames); err != nil {
		return err
	}
	for _, line := range b.BufferLines() {
		fmt.Println(strings.TrimRight(line, " "))
	}
	return nil
}

func run(b backend.Backend, frames int) error {
	t, err := terminal.New(b)
	if err != nil {
		return err
	}

	listState := widgets.NewListState()
	tableState := widgets.NewTableState()
	cell := [2]int{1, 1}
	tableState.SelectCell(&cell)

	for i := range frames {
		selected := (i + 1) % 4
		listState.Select(&selected)
		if _, err := t.Draw(func(f *terminal.Frame) {
			render(f, i, &listState, &tableState)
		}); err != nil {
			return err
		}
	}
	return nil
}

func render(f *terminal.Frame, frame int, listState *widgets.ListState, tableState *widgets.TableState) {
	cyan := style.New().WithFG(style.LightCyan)
	accent := cyan.WithAddModifier(style.ModBold)
	selected := style.New().WithFG(style.Black).WithBG(style.Cyan).WithAddModifier(style.ModBold)
	dim := style.New().WithFG(style.DarkGray)

	vertical := layout.Vertical(
		layout.Length(3),
		layout.Length(5),
		layout.Min(8),
		layout.Length(8),
	).Split(f.Area())
	if len(vertical) != 4 {
		return
	}

	header := widgets.NewParagraph(text.FromLine(text.FromSpans(
		text.StyledSpan("Ratatui", accent),
		text.RawSpan(" → Go  •  immediate-mode terminal UI  •  Unicode ✓"),
	))).Block(widgets.Bordered().Title(text.RawLine(" ratatui-go ")).BorderStyle(cyan))
	f.RenderWidget(header, vertical[0])

	metrics := layout.Horizontal(layout.Percentage(55), layout.Fill(1)).Spacing(1).Split(vertical[1])
	if len(metrics) == 2 {
		progress := 0.68 + float64(frame%4)*0.06
		gauge := widgets.NewGauge().
			Block(widgets.Bordered().Title(text.RawLine(" Port progress "))).
			Ratio(progress).
			GaugeStyle(style.New().WithFG(style.Green).WithBG(style.DarkGray)).
			UseUnicode(true)
		f.RenderWidget(gauge, metrics[0])

		data := []uint64{3, 5, 4, 7, 6, 8, 5, 9, 7, 10, 9, uint64(10 + frame)}
		spark := widgets.NewSparkline().
			Block(widgets.Bordered().Title(text.RawLine(" Frame diff "))).
			Style(cyan).
			DataUint64(data...)
		f.RenderWidget(spark, metrics[1])
	}

	body := layout.Horizontal(layout.Length(27), layout.Fill(1)).Spacing(1).Split(vertical[2])
	if len(body) == 2 {
		items := []widgets.ListItem{
			widgets.NewListItem(text.RawText("Layout + constraints")),
			widgets.NewListItem(text.RawText("Unicode text + styles")),
			widgets.NewListItem(text.RawText("Double-buffer terminal")),
			widgets.NewListItem(text.RawText("Widgets + state")),
		}
		list := widgets.NewList(items...).
			Block(widgets.Bordered().Title(text.RawLine(" Ported systems "))).
			HighlightSymbol(text.RawLine("▶ ")).
			HighlightStyle(selected).
			WithHighlightSpacing(widgets.HighlightAlways)
		list.RenderStateful(body[0], f.Buffer(), listState)

		rows := []widgets.Row{
			widgets.NewRow(
				widgets.NewCell(text.RawText("layout")),
				widgets.NewCell(text.RawText("ready")),
				widgets.NewCell(text.RawText("constraints, flex")),
			),
			widgets.NewRow(
				widgets.NewCell(text.RawText("terminal")),
				widgets.NewCell(text.RawText("ready")),
				widgets.NewCell(text.RawText("ANSI + test backend")),
			),
			widgets.NewRow(
				widgets.NewCell(text.RawText("widgets")),
				widgets.NewCell(text.RawText("ready")),
				widgets.NewCell(text.RawText("stateful rendering")),
			),
		}
		table := widgets.NewTable(rows, []layout.Constraint{
			layout.Length(12), layout.Length(9), layout.Fill(1),
		}).
			Header(widgets.NewRow(
				widgets.NewCell(text.StyledText("Package", accent)),
				widgets.NewCell(text.StyledText("Status", accent)),
				widgets.NewCell(text.StyledText("Proof", accent)),
			)).
			Block(widgets.Bordered().Title(text.RawLine(" Runtime matrix "))).
			RowHighlightStyle(style.New().WithBG(style.DarkGray)).
			CellHighlightStyle(selected).
			HighlightSymbol(text.RawText("› ")).
			WithHighlightSpacing(widgets.HighlightWhenSelected)
		table.RenderStateful(body[1], f.Buffer(), tableState)
	}

	bars := []widgets.Bar{
		widgets.BarWithLabel(text.RawLine("core"), 100).WithStyle(style.New().WithFG(style.Cyan)),
		widgets.BarWithLabel(text.RawLine("text"), 100).WithStyle(style.New().WithFG(style.Green)),
		widgets.BarWithLabel(text.RawLine("term"), 100).WithStyle(style.New().WithFG(style.Yellow)),
		widgets.BarWithLabel(text.RawLine("UI"), 100).WithStyle(style.New().WithFG(style.Magenta)),
	}
	chart := widgets.NewBarChart(bars).
		Block(widgets.Bordered().Title(text.RawLine(" Conversion coverage ")).BorderStyle(dim)).
		Max(100).
		BarWidth(5).
		BarGap(3).
		ValueStyle(style.New().WithFG(style.White).WithAddModifier(style.ModBold))
	f.RenderWidget(chart, vertical[3])
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
