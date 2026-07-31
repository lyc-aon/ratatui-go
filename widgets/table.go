package widgets

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/text"
)

// Cell is one table cell: styled text that may span multiple columns.
type Cell struct {
	content    text.Text
	style      style.Style
	columnSpan int
}

// NewCell creates a cell with the given content and column span 1.
func NewCell(content text.Text) Cell {
	return Cell{
		content:    copyText(content),
		columnSpan: 1,
	}
}

// Content replaces the cell content (value builder; copies text).
func (c Cell) Content(content text.Text) Cell {
	c.content = copyText(content)
	return c
}

// Style sets the cell base style (applied before content styles).
func (c Cell) Style(st style.Style) Cell {
	c.style = st
	return c
}

// WithColumnSpan sets how many columns this cell occupies (≥1 effective for layout;
// span 0 skips the cell entirely during body row render).
func (c Cell) WithColumnSpan(span int) Cell {
	c.columnSpan = span
	return c
}

// ColumnSpan returns the configured column span.
func (c Cell) ColumnSpan() int {
	return c.columnSpan
}

// ContentText returns a copy of the cell content.
func (c Cell) ContentText() text.Text {
	return copyText(c.content)
}

// CellStyle returns the cell style.
func (c Cell) CellStyle() style.Style {
	return c.style
}

func (c Cell) render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil || area.IsEmpty() {
		return
	}
	buf.SetStyle(area, c.style)
	renderTableText(c.content, area, buf)
}

// Row is one table row: cells plus height and vertical margins.
type Row struct {
	cells        []Cell
	height       int
	topMargin    int
	bottomMargin int
	style        style.Style
}

// NewRow creates a row with height 1 from the given cells (slice is copied).
func NewRow(cells ...Cell) Row {
	return Row{
		cells:  copyCells(cells),
		height: 1,
	}
}

// Cells replaces the row's cells (slice is copied).
func (r Row) Cells(cells ...Cell) Row {
	r.cells = copyCells(cells)
	return r
}

// WithHeight sets the fixed row content height (default 1). Content taller
// than this is clipped.
func (r Row) WithHeight(height int) Row {
	if height < 0 {
		height = 0
	}
	r.height = height
	return r
}

// TopMargin sets blank lines drawn above the row content (default 0).
func (r Row) TopMargin(margin int) Row {
	if margin < 0 {
		margin = 0
	}
	r.topMargin = margin
	return r
}

// BottomMargin sets blank lines drawn below the row content (default 0).
func (r Row) BottomMargin(margin int) Row {
	if margin < 0 {
		margin = 0
	}
	r.bottomMargin = margin
	return r
}

// Style sets the row base style (overridden by cell / highlight styles).
func (r Row) Style(st style.Style) Row {
	r.style = st
	return r
}

// Height returns the content height.
func (r Row) Height() int { return r.height }

// HeightWithMargin returns content height plus top and bottom margins (saturating).
func (r Row) HeightWithMargin() int {
	return satAdd(satAdd(r.height, r.topMargin), r.bottomMargin)
}

// Table renders tabular data with optional header/footer, selection, and flex widths.
//
// Value builder: every setter returns a modified copy. Render never panics on
// zero-size or empty data. Constructors and builders copy caller slices.
type Table struct {
	rows                 []Row
	header               *Row
	footer               *Row
	widths               []layout.Constraint
	columnSpacing        int
	block                *Block
	style                style.Style
	rowHighlightStyle    style.Style
	columnHighlightStyle style.Style
	cellHighlightStyle   style.Style
	highlightSymbol      text.Text
	highlightSpacing     HighlightSpacing
	flex                 layout.Flex
}

// NewTable creates a table from rows and column width constraints.
// Both slices are copied. Empty widths default at render time to even columns
// based on the max cell count across header/rows/footer.
func NewTable(rows []Row, widths []layout.Constraint) Table {
	return Table{
		rows:          copyRows(rows),
		widths:        copyConstraints(widths),
		columnSpacing: 1,
		flex:          layout.FlexStart,
		// highlightSpacing zero value = HighlightWhenSelected
	}
}

// Rows replaces the data rows (slice is copied).
func (t Table) Rows(rows []Row) Table {
	t.rows = copyRows(rows)
	return t
}

// Header sets the optional header row.
func (t Table) Header(header Row) Table {
	h := copyRow(header)
	t.header = &h
	return t
}

// Footer sets the optional footer row.
func (t Table) Footer(footer Row) Table {
	f := copyRow(footer)
	t.footer = &f
	return t
}

// Widths replaces column width constraints (slice is copied).
func (t Table) Widths(widths []layout.Constraint) Table {
	t.widths = copyConstraints(widths)
	return t
}

// ColumnSpacing sets the gap between columns (default 1).
func (t Table) ColumnSpacing(spacing int) Table {
	if spacing < 0 {
		spacing = 0
	}
	t.columnSpacing = spacing
	return t
}

// Block wraps the table in an optional block (copied by value into a pointer).
func (t Table) Block(block Block) Table {
	b := block
	t.block = &b
	return t
}

// Style sets the base widget style applied before the block and content.
func (t Table) Style(st style.Style) Table {
	t.style = st
	return t
}

// RowHighlightStyle sets the style patched onto the selected row.
func (t Table) RowHighlightStyle(st style.Style) Table {
	t.rowHighlightStyle = st
	return t
}

// ColumnHighlightStyle sets the style patched onto the selected column.
func (t Table) ColumnHighlightStyle(st style.Style) Table {
	t.columnHighlightStyle = st
	return t
}

// CellHighlightStyle sets the style patched onto the selected cell
// (intersection of selected row and column). Applied after row then column.
func (t Table) CellHighlightStyle(st style.Style) Table {
	t.cellHighlightStyle = st
	return t
}

// HighlightSymbol sets the text drawn in the selection column for the selected row.
func (t Table) HighlightSymbol(symbol text.Text) Table {
	t.highlightSymbol = copyText(symbol)
	return t
}

// WithHighlightSpacing configures when the selection column is reserved.
func (t Table) WithHighlightSpacing(spacing HighlightSpacing) Table {
	t.highlightSpacing = spacing
	return t
}

// Flex sets how leftover horizontal space is distributed among columns.
func (t Table) Flex(flex layout.Flex) Table {
	t.flex = flex
	return t
}

// Render draws the table with a fresh default TableState.
func (t Table) Render(area layout.Rect, buf *buffer.Buffer) {
	state := NewTableState()
	t.RenderStateful(area, buf, &state)
}

// RenderStateful draws the table and repairs selection / scroll offset on state.
func (t Table) RenderStateful(area layout.Rect, buf *buffer.Buffer, state *TableState) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	if state == nil {
		tmp := NewTableState()
		state = &tmp
	}

	buf.SetStyle(area, t.style)
	tableArea := InnerIfSome(t.block, area, buf)
	if tableArea.IsEmpty() {
		return
	}

	// Clamp / clear row selection.
	if state.selected != nil && *state.selected >= len(t.rows) {
		last := len(t.rows) - 1
		if last < 0 {
			state.Select(nil)
		} else {
			state.Select(&last)
		}
	}
	if len(t.rows) == 0 {
		state.Select(nil)
	}

	columnCount := t.columnCount()
	if state.selectedColumn != nil && *state.selectedColumn >= columnCount {
		last := columnCount - 1
		if last < 0 {
			state.SelectColumn(nil)
		} else {
			state.SelectColumn(&last)
		}
	}
	if columnCount == 0 {
		state.SelectColumn(nil)
	}

	selectionWidth := t.selectionWidth(state)
	columnWidths := t.getColumnWidths(tableArea.Width, selectionWidth, columnCount)
	headerArea, rowsArea, footerArea := t.layout(tableArea)

	t.renderHeader(headerArea, buf, columnWidths)
	t.renderRows(rowsArea, buf, selectionWidth, state, columnWidths)
	t.renderFooter(footerArea, buf, columnWidths)
}

// layout splits the table area into header content, body rows, and footer content.
func (t Table) layout(area layout.Rect) (headerArea, rowsArea, footerArea layout.Rect) {
	headerTop, headerH, headerBottom := 0, 0, 0
	if t.header != nil {
		headerTop = t.header.topMargin
		headerH = t.header.height
		headerBottom = t.header.bottomMargin
	}
	footerTop, footerH, footerBottom := 0, 0, 0
	if t.footer != nil {
		footerTop = t.footer.topMargin
		footerH = t.footer.height
		footerBottom = t.footer.bottomMargin
	}
	parts := layout.Vertical(
		layout.Length(headerTop),
		layout.Length(headerH),
		layout.Length(headerBottom),
		layout.Min(0),
		layout.Length(footerTop),
		layout.Length(footerH),
		layout.Length(footerBottom),
	).Split(area)
	// Defensive: empty split shouldn't happen, but never panic.
	if len(parts) < 7 {
		return layout.Rect{}, area, layout.Rect{}
	}
	return parts[1], parts[3], parts[5]
}

func (t Table) renderHeader(area layout.Rect, buf *buffer.Buffer, columnWidths []layout.Rect) {
	if t.header == nil || area.IsEmpty() {
		return
	}
	buf.SetStyle(area, t.header.style)
	// Header zips cells 1:1 with columns (no column_span), matching upstream.
	n := len(t.header.cells)
	if len(columnWidths) < n {
		n = len(columnWidths)
	}
	for i := range n {
		cellArea := columnWidths[i]
		renderArea := layout.NewRect(
			satAdd(area.X, cellArea.X),
			area.Y,
			cellArea.Width,
			area.Height,
		)
		t.header.cells[i].render(renderArea, buf)
	}
}

func (t Table) renderFooter(area layout.Rect, buf *buffer.Buffer, columnWidths []layout.Rect) {
	if t.footer == nil || area.IsEmpty() {
		return
	}
	buf.SetStyle(area, t.footer.style)
	n := len(t.footer.cells)
	if len(columnWidths) < n {
		n = len(columnWidths)
	}
	for i := range n {
		cellArea := columnWidths[i]
		renderArea := layout.NewRect(
			satAdd(area.X, cellArea.X),
			area.Y,
			cellArea.Width,
			area.Height,
		)
		t.footer.cells[i].render(renderArea, buf)
	}
}

func (t Table) renderRows(
	area layout.Rect,
	buf *buffer.Buffer,
	selectionWidth int,
	state *TableState,
	columnWidths []layout.Rect,
) {
	if len(t.rows) == 0 || area.IsEmpty() {
		return
	}

	startIndex, endIndex := t.visibleRows(state, area)
	state.offset = startIndex

	yOffset := 0
	var selectedRowArea *layout.Rect

	if endIndex > len(t.rows) {
		endIndex = len(t.rows)
	}
	if startIndex > endIndex {
		startIndex = endIndex
	}

	for i := startIndex; i < endIndex; i++ {
		row := t.rows[i]
		y := satAdd(satAdd(area.Y, yOffset), row.topMargin)
		// height = min(y+row.height, area.bottom) - y, saturating
		bottom := area.Bottom()
		rowBottom := satAdd(y, row.height)
		if rowBottom > bottom {
			rowBottom = bottom
		}
		height := satSub(rowBottom, y)
		rowArea := layout.Rect{X: area.X, Y: y, Width: area.Width, Height: height}
		buf.SetStyle(rowArea, row.style)

		isSelected := state.selected != nil && *state.selected == i
		if selectionWidth > 0 && isSelected {
			t.setSelectionStyle(buf, selectionWidth, rowArea, row)
		}
		t.renderRowCells(buf, columnWidths, row.cells, rowArea)
		if isSelected {
			ra := rowArea
			selectedRowArea = &ra
		}
		yOffset = satAdd(yOffset, row.HeightWithMargin())
	}

	var selectedColumnArea *layout.Rect
	if state.selectedColumn != nil {
		s := *state.selectedColumn
		if s >= 0 && s < len(columnWidths) {
			cellArea := columnWidths[s]
			ca := layout.Rect{
				X:      satAdd(cellArea.X, area.X),
				Y:      area.Y,
				Width:  cellArea.Width,
				Height: area.Height,
			}
			selectedColumnArea = &ca
		}
	}

	// Style precedence: row highlight → column highlight → cell highlight.
	switch {
	case selectedRowArea != nil && selectedColumnArea != nil:
		buf.SetStyle(*selectedRowArea, t.rowHighlightStyle)
		buf.SetStyle(*selectedColumnArea, t.columnHighlightStyle)
		cellArea := selectedRowArea.Intersection(*selectedColumnArea)
		buf.SetStyle(cellArea, t.cellHighlightStyle)
	case selectedRowArea != nil:
		buf.SetStyle(*selectedRowArea, t.rowHighlightStyle)
	case selectedColumnArea != nil:
		buf.SetStyle(*selectedColumnArea, t.columnHighlightStyle)
	}
}

func (t Table) renderRowCells(
	buf *buffer.Buffer,
	columnWidths []layout.Rect,
	cells []Cell,
	rowArea layout.Rect,
) {
	colIdx := 0
	for ci := range cells {
		cell := cells[ci]
		cellArea, next := getCellArea(columnWidths, colIdx, cell.columnSpan, t.columnSpacing)
		colIdx = next
		if cellArea == nil {
			continue
		}
		renderArea := layout.NewRect(
			satAdd(rowArea.X, cellArea.X),
			rowArea.Y,
			cellArea.Width,
			rowArea.Height,
		)
		cell.render(renderArea, buf)
	}
}

func (t Table) setSelectionStyle(buf *buffer.Buffer, selectionWidth int, rowArea layout.Rect, row Row) {
	selectionArea := layout.Rect{
		X:      rowArea.X,
		Y:      rowArea.Y,
		Width:  selectionWidth,
		Height: rowArea.Height,
	}
	buf.SetStyle(selectionArea, row.style)
	renderTableText(t.highlightSymbol, selectionArea, buf)
}

// getCellArea consumes columnSpan columns starting at colIdx and returns the
// merged area (x/width only matter) plus the next column index.
// Returns nil area when span is 0 or no columns remain.
func getCellArea(columnWidths []layout.Rect, colIdx, columnSpan, columnSpacing int) (*layout.Rect, int) {
	if columnSpan <= 0 || colIdx >= len(columnWidths) {
		return nil, colIdx
	}
	first := columnWidths[colIdx]
	nTaken := 1
	totalWidth := first.Width
	colIdx++
	for nTaken < columnSpan && colIdx < len(columnWidths) {
		totalWidth = satAdd(totalWidth, columnWidths[colIdx].Width)
		nTaken++
		colIdx++
	}
	// Add gaps between consumed columns.
	if nTaken > 1 {
		totalWidth = satAdd(totalWidth, (nTaken-1)*max0(columnSpacing))
	}
	area := layout.NewRect(first.X, first.Y, totalWidth, 1)
	return &area, colIdx
}

// visibleRows returns [start, end) indexes of rows to draw and scrolls so the
// selection stays in view. end may point one past a partial trailing row.
func (t Table) visibleRows(state *TableState, area layout.Rect) (start, end int) {
	if len(t.rows) == 0 {
		return 0, 0
	}
	lastRow := len(t.rows) - 1
	start = state.offset
	if start > lastRow {
		start = lastRow
	}
	if start < 0 {
		start = 0
	}
	if state.selected != nil {
		sel := *state.selected
		if sel < start {
			start = sel
		}
		if start < 0 {
			start = 0
		}
	}

	end = start
	height := 0
	for _, item := range t.rows[start:] {
		if height+item.height > area.Height {
			break
		}
		height = satAdd(height, item.HeightWithMargin())
		end++
	}

	if state.selected != nil {
		selected := *state.selected
		if selected > lastRow {
			selected = lastRow
		}
		// Scroll down until the selected row is inside [start, end).
		for selected >= end && end <= lastRow {
			height = satAdd(height, t.rows[end].HeightWithMargin())
			end++
			for height > area.Height && start < end {
				height = satSub(height, t.rows[start].HeightWithMargin())
				start++
			}
		}
	}

	// Include a partial row if there is leftover vertical space.
	if height < area.Height && end < len(t.rows) {
		end++
	}
	return start, end
}

// getColumnWidths computes per-column rects (x/width relative to columns area origin 0).
// Empty widths → even Length split by max cell count over full maxWidth (upstream).
func (t Table) getColumnWidths(maxWidth, selectionWidth, colCount int) []layout.Rect {
	if maxWidth < 0 {
		maxWidth = 0
	}
	if selectionWidth < 0 {
		selectionWidth = 0
	}
	if colCount < 0 {
		colCount = 0
	}

	var widths []layout.Constraint
	if len(t.widths) == 0 {
		// Even split across columns using the full table width (including selection).
		div := colCount
		if div < 1 {
			div = 1
		}
		each := maxWidth / div
		widths = make([]layout.Constraint, colCount)
		for i := range widths {
			widths[i] = layout.Length(each)
		}
	} else {
		widths = t.widths
	}

	// Always reserve selection column then fill the rest for data columns.
	splitSel := layout.Horizontal(
		layout.Length(selectionWidth),
		layout.Fill(0),
	).Split(layout.NewRect(0, 0, maxWidth, 1))
	columnsArea := layout.Rect{}
	if len(splitSel) >= 2 {
		columnsArea = splitSel[1]
	}

	if colCount == 0 || len(widths) == 0 {
		return nil
	}

	// If caller provided fewer/more widths than colCount, layout uses what it got.
	rects := layout.Horizontal(widths...).
		Flex(t.flex).
		Spacing(t.columnSpacing).
		Split(columnsArea)

	out := make([]layout.Rect, len(rects))
	for i, c := range rects {
		out[i] = layout.NewRect(c.X, 0, c.Width, 1)
	}
	return out
}

func (t Table) columnCount() int {
	maxCells := 0
	for _, r := range t.rows {
		if n := len(r.cells); n > maxCells {
			maxCells = n
		}
	}
	if t.header != nil {
		if n := len(t.header.cells); n > maxCells {
			maxCells = n
		}
	}
	if t.footer != nil {
		if n := len(t.footer.cells); n > maxCells {
			maxCells = n
		}
	}
	return maxCells
}

func (t Table) selectionWidth(state *TableState) int {
	hasSelection := state != nil && state.selected != nil
	if t.highlightSpacing.ShouldAdd(hasSelection) {
		return t.highlightSymbol.Width()
	}
	return 0
}

// renderTableText paints multi-line text into area with per-line style.
// Clips by terminal cell width; safe on empty/nil.
func renderTableText(t text.Text, area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, t.Style)
	rows := area.Rows()
	n := len(t.Lines)
	if len(rows) < n {
		n = len(rows)
	}
	for i := range n {
		lineArea := rows[i]
		if lineArea.IsEmpty() {
			continue
		}
		spans, _, leftPad, lineStyle, ok := t.LineRenderData(i, lineArea.Width)
		if !ok {
			continue
		}
		// Rust Line::render skips set_style when the line width is 0, leaving
		// the Text-level style applied above.
		if t.Lines[i].Width() == 0 {
			continue
		}
		buf.SetStyle(lineArea, lineStyle)
		x := satAdd(lineArea.X, leftPad)
		remaining := satSub(lineArea.Width, leftPad)
		for si := range spans {
			if remaining <= 0 {
				break
			}
			st := lineStyle.Patch(spans[si].Style)
			nx, _ := buf.SetStringN(x, lineArea.Y, spans[si].Content, remaining, st)
			w := nx - x
			if w < 0 {
				w = 0
			}
			x = nx
			remaining = satSub(remaining, w)
		}
	}
}

func copyText(t text.Text) text.Text {
	// text.FromLineSlice already copies lines; preserve style + alignment.
	out := text.FromLineSlice(t.Lines).WithStyle(t.Style)
	if t.Alignment != nil {
		out = out.WithAlignment(*t.Alignment)
	}
	return out
}

func copyCells(in []Cell) []Cell {
	if len(in) == 0 {
		return nil
	}
	out := make([]Cell, len(in))
	for i := range in {
		out[i] = Cell{
			content:    copyText(in[i].content),
			style:      in[i].style,
			columnSpan: in[i].columnSpan,
		}
	}
	return out
}

func copyRow(r Row) Row {
	return Row{
		cells:        copyCells(r.cells),
		height:       r.height,
		topMargin:    r.topMargin,
		bottomMargin: r.bottomMargin,
		style:        r.style,
	}
}

func copyRows(in []Row) []Row {
	if len(in) == 0 {
		return nil
	}
	out := make([]Row, len(in))
	for i := range in {
		out[i] = copyRow(in[i])
	}
	return out
}

func copyConstraints(in []layout.Constraint) []layout.Constraint {
	if len(in) == 0 {
		return nil
	}
	out := make([]layout.Constraint, len(in))
	copy(out, in)
	return out
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
