package widgets

import (
	"testing"

	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/text"
)

func TestTableHeaderAndWidths(t *testing.T) {
	hdr := NewRow(NewCell(text.RawText("H1")), NewCell(text.RawText("H2")))
	row1 := NewRow(NewCell(text.RawText("A1")), NewCell(text.RawText("A2")))

	tbl := NewTable([]Row{row1}, []layout.Constraint{layout.Length(4), layout.Length(4)}).
		Header(hdr).
		ColumnSpacing(1)

	area := layout.NewRect(0, 0, 10, 3)
	buf := buffer.Empty(area)

	tbl.Render(area, buf)

	// Header row y=0
	r0 := getBufferRowString(buf, 0)
	wantR0 := "H1   H2   "
	if r0 != wantR0 {
		t.Errorf("header row 0 = %q, want %q", r0, wantR0)
	}

	// Data row y=1
	r1 := getBufferRowString(buf, 1)
	wantR1 := "A1   A2   "
	if r1 != wantR1 {
		t.Errorf("data row 1 = %q, want %q", r1, wantR1)
	}
}

func TestTableColumnSpan(t *testing.T) {
	// Cell 0 spans 2 columns
	row := NewRow(
		NewCell(text.RawText("SPAN")).WithColumnSpan(2),
		NewCell(text.RawText("END")),
	)

	tbl := NewTable([]Row{row}, []layout.Constraint{layout.Length(3), layout.Length(3), layout.Length(3)}).
		ColumnSpacing(1)

	area := layout.NewRect(0, 0, 11, 2)
	buf := buffer.Empty(area)

	tbl.Render(area, buf)

	// Col 0 (width 3) + Spacing (1) + Col 1 (width 3) = 7 cols total for SPAN cell
	// Col 2 (width 3) starts at col 8 -> "END"
	r0 := getBufferRowString(buf, 0)
	if r0 != "SPAN    END" {
		t.Errorf("spanned row 0 = %q, want %q", r0, "SPAN    END")
	}
}

func TestTableHighlightPrecedence(t *testing.T) {
	row0 := NewRow(NewCell(text.RawText("C0")), NewCell(text.RawText("C1")))
	row1 := NewRow(NewCell(text.RawText("C2")), NewCell(text.RawText("C3")))

	rowSty := style.New().WithFG(style.Red)
	colSty := style.New().WithFG(style.Green)
	cellSty := style.New().WithFG(style.Blue)

	tbl := NewTable([]Row{row0, row1}, []layout.Constraint{layout.Length(3), layout.Length(3)}).
		RowHighlightStyle(rowSty).
		ColumnHighlightStyle(colSty).
		CellHighlightStyle(cellSty)

	area := layout.NewRect(0, 0, 8, 2)
	buf := buffer.Empty(area)

	state := NewTableState()
	cellSel := [2]int{1, 1} // Row 1, Col 1
	state = state.WithSelectedCell(&cellSel)

	tbl.RenderStateful(area, buf, &state)

	// Cell (1,1) is at row 1, col 4 ('C' of 'C3')
	// Row 1 Col 0 ('C' of 'C2', x=0, y=1) should have RowHighlightStyle (Red)
	cRowOnly, _ := buf.Get(0, 1)
	if cRowOnly.Style.FG != style.Red {
		t.Errorf("Row-only cell FG = %v, want Red", cRowOnly.Style.FG)
	}

	// Row 0 Col 1 ('C' of 'C1', x=4, y=0) should have ColumnHighlightStyle (Green)
	cColOnly, _ := buf.Get(4, 0)
	if cColOnly.Style.FG != style.Green {
		t.Errorf("Col-only cell FG = %v, want Green", cColOnly.Style.FG)
	}

	// Row 1 Col 1 ('C' of 'C3', x=4, y=1) should have CellHighlightStyle (Blue - highest precedence)
	cCellHighlight, _ := buf.Get(4, 1)
	if cCellHighlight.Style.FG != style.Blue {
		t.Errorf("Intersection cell FG = %v, want Blue", cCellHighlight.Style.FG)
	}
}

func TestTableStateRepair(t *testing.T) {
	rows := []Row{
		NewRow(NewCell(text.RawText("R0"))),
		NewRow(NewCell(text.RawText("R1"))),
	}

	tbl := NewTable(rows, []layout.Constraint{layout.Length(5)})

	area := layout.NewRect(0, 0, 5, 2)
	buf := buffer.Empty(area)

	state := NewTableState()
	outOfRange := 100
	state = state.WithSelected(&outOfRange)

	tbl.RenderStateful(area, buf, &state)

	// RenderStateful should repair selected index to 1 (last valid row index)
	if state.Selected() == nil || *state.Selected() != 1 {
		t.Errorf("repaired selected = %v, want 1", state.Selected())
	}
}

func TestTableZeroAreaSmoke(t *testing.T) {
	zero := layout.NewRect(0, 0, 0, 0)
	buf := buffer.Empty(zero)

	tbl := NewTable([]Row{NewRow(NewCell(text.RawText("X")))}, []layout.Constraint{layout.Length(5)}).
		Header(NewRow(NewCell(text.RawText("H"))))

	state := NewTableState()
	sel := 0
	state = state.WithSelected(&sel)

	tbl.RenderStateful(zero, buf, &state)
	tbl.Render(zero, buf)
}

func TestTableHighlightSpacingAlwaysNever(t *testing.T) {
	row := NewRow(NewCell(text.RawText("DATA")))
	tbl := NewTable([]Row{row}, []layout.Constraint{layout.Length(4)}).
		HighlightSymbol(text.RawText("> "))

	area := layout.NewRect(0, 0, 8, 1)

	// HighlightAlways without selection should still reserve symbol space (2 cols)
	tblAlways := tbl.WithHighlightSpacing(HighlightAlways)
	bufAlways := buffer.Empty(area)
	stateNoSel := NewTableState()
	tblAlways.RenderStateful(area, bufAlways, &stateNoSel)
	rAlways := getBufferRowString(bufAlways, 0)
	if rAlways != "  DATA  " {
		t.Errorf("HighlightAlways row 0 = %q, want %q", rAlways, "  DATA  ")
	}

	// HighlightNever with selection should NOT draw symbol or reserve space
	tblNever := tbl.WithHighlightSpacing(HighlightNever)
	bufNever := buffer.Empty(area)
	stateSel := NewTableState()
	sel := 0
	stateSel = stateSel.WithSelected(&sel)
	tblNever.RenderStateful(area, bufNever, &stateSel)
	rNever := getBufferRowString(bufNever, 0)
	if rNever != "DATA    " {
		t.Errorf("HighlightNever row 0 = %q, want %q", rNever, "DATA    ")
	}
}

func TestTableSpansPastRemainingColumns(t *testing.T) {
	// Cell with columnSpan=5 when only 2 columns exist
	row := NewRow(NewCell(text.RawText("OVERSPAN")).WithColumnSpan(5))
	tbl := NewTable([]Row{row}, []layout.Constraint{layout.Length(4), layout.Length(4)})

	area := layout.NewRect(0, 0, 8, 1)
	buf := buffer.Empty(area)

	// Must render cleanly spanning remaining columns without indexing out of bounds or crashing
	tbl.Render(area, buf)
	r0 := getBufferRowString(buf, 0)
	if r0 != "OVERSPAN" {
		t.Errorf("overspan row 0 = %q, want %q", r0, "OVERSPAN")
	}
}

func TestTableFlexAndSpacing(t *testing.T) {
	row := NewRow(NewCell(text.RawText("C1")), NewCell(text.RawText("C2")))
	tbl := NewTable([]Row{row}, []layout.Constraint{layout.Fill(1), layout.Fill(1)}).
		ColumnSpacing(2)

	area := layout.NewRect(0, 0, 10, 1)
	buf := buffer.Empty(area)
	tbl.Render(area, buf)

	r0 := getBufferRowString(buf, 0)
	// Width 10 with spacing 2 -> col1 width 4, spacing 2, col2 width 4
	// Row 0: "C1    C2  "
	if r0 != "C1    C2  " {
		t.Errorf("flex+spacing row 0 = %q, want %q", r0, "C1    C2  ")
	}
}

func TestTableZeroWidthLineKeepsTextStyle(t *testing.T) {
	// Rust Line::render returns before set_style when width==0.
	// renderTableText must keep Text-level style on empty lines.
	// Pinned: cell Text style Blue bg + empty Line with Red line style → Blue.
	emptyRed := text.RawLine("").WithStyle(style.New().WithBG(style.Red))
	content := text.FromLines(emptyRed).WithStyle(style.New().WithBG(style.Blue))
	row := NewRow(NewCell(content))
	tbl := NewTable([]Row{row}, []layout.Constraint{layout.Length(4)})

	area := layout.NewRect(0, 0, 4, 1)
	buf := buffer.Empty(area)
	tbl.Render(area, buf)

	cell, ok := buf.Get(0, 0)
	if !ok {
		t.Fatal("cell missing")
	}
	bg, set := cell.Style.Background()
	if !set || bg != style.Blue {
		t.Fatalf("zero-width line cell bg = (%v, %v), want Blue Text style (not Red line style)", bg, set)
	}
}
