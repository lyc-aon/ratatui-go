package widgets

import "github.com/lyc-aon/ratatui-go/text"

// CellFromString creates a cell from a plain string (Ratatui Cell::from(&str)).
func CellFromString(s string) Cell {
	return NewCell(text.RawText(s))
}

// RowFromStrings creates a height-1 row from plain string cells
// (Ratatui Row::new(vec!["a", "b", ...])).
func RowFromStrings(cells ...string) Row {
	out := make([]Cell, len(cells))
	for i, s := range cells {
		out[i] = CellFromString(s)
	}
	return NewRow(out...)
}
