package widgets

import "math"

// TableState holds scroll offset and selection for a Table.
//
// Fields are unexported; use accessors. Select(nil) resets offset to 0.
// SelectLast / SelectPrevious use math.MaxInt as a sentinel that render clamps.
type TableState struct {
	offset         int
	selected       *int
	selectedColumn *int
}

// NewTableState creates an empty table state (no selection, offset 0).
func NewTableState() TableState {
	return TableState{}
}

// WithOffset returns a copy with the first visible row index set.
func (s TableState) WithOffset(offset int) TableState {
	if offset < 0 {
		offset = 0
	}
	s.offset = offset
	return s
}

// WithSelected returns a copy with the selected row index set.
// Pass nil for no selection.
func (s TableState) WithSelected(selected *int) TableState {
	if selected == nil {
		s.selected = nil
		return s
	}
	v := *selected
	if v < 0 {
		v = 0
	}
	s.selected = &v
	return s
}

// WithSelectedColumn returns a copy with the selected column index set.
// Pass nil for no column selection.
func (s TableState) WithSelectedColumn(selected *int) TableState {
	if selected == nil {
		s.selectedColumn = nil
		return s
	}
	v := *selected
	if v < 0 {
		v = 0
	}
	s.selectedColumn = &v
	return s
}

// WithSelectedCell returns a copy with both row and column selection set.
// Pass nil to clear both.
func (s TableState) WithSelectedCell(cell *[2]int) TableState {
	if cell == nil {
		s.selected = nil
		s.selectedColumn = nil
		return s
	}
	r, c := cell[0], cell[1]
	if r < 0 {
		r = 0
	}
	if c < 0 {
		c = 0
	}
	s.selected = &r
	s.selectedColumn = &c
	return s
}

// Offset returns the index of the first visible row.
func (s *TableState) Offset() int {
	if s == nil {
		return 0
	}
	return s.offset
}

// SetOffset sets the first visible row index.
func (s *TableState) SetOffset(offset int) {
	if s == nil {
		return
	}
	if offset < 0 {
		offset = 0
	}
	s.offset = offset
}

// Selected returns a copy of the selected row index, or nil if none.
func (s *TableState) Selected() *int {
	if s == nil {
		return nil
	}
	return copyIntPtr(s.selected)
}

// SelectedColumn returns a copy of the selected column index, or nil if none.
func (s *TableState) SelectedColumn() *int {
	if s == nil {
		return nil
	}
	return copyIntPtr(s.selectedColumn)
}

// SelectedCell returns [row, col] when both are selected, else nil.
func (s *TableState) SelectedCell() *[2]int {
	if s == nil || s.selected == nil || s.selectedColumn == nil {
		return nil
	}
	cell := [2]int{*s.selected, *s.selectedColumn}
	return &cell
}

// Select sets the selected row. nil clears selection and resets offset to 0.
func (s *TableState) Select(index *int) {
	if s == nil {
		return
	}
	if index == nil {
		s.selected = nil
		s.offset = 0
		return
	}
	v := *index
	if v < 0 {
		v = 0
	}
	s.selected = &v
}

// SelectColumn sets the selected column. nil clears column selection.
func (s *TableState) SelectColumn(index *int) {
	if s == nil {
		return
	}
	if index == nil {
		s.selectedColumn = nil
		return
	}
	v := *index
	if v < 0 {
		v = 0
	}
	s.selectedColumn = &v
}

// SelectCell sets both row and column. nil clears both and resets offset to 0.
func (s *TableState) SelectCell(indexes *[2]int) {
	if s == nil {
		return
	}
	if indexes == nil {
		s.offset = 0
		s.selected = nil
		s.selectedColumn = nil
		return
	}
	r, c := indexes[0], indexes[1]
	if r < 0 {
		r = 0
	}
	if c < 0 {
		c = 0
	}
	s.selected = &r
	s.selectedColumn = &c
}

// SelectNext selects the next row, or 0 if nothing was selected.
// Saturating add; render clamps past the last row.
func (s *TableState) SelectNext() {
	if s == nil {
		return
	}
	next := 0
	if s.selected != nil {
		next = satAddInt(*s.selected, 1)
	}
	s.Select(&next)
}

// SelectNextColumn selects the next column, or 0 if nothing was selected.
func (s *TableState) SelectNextColumn() {
	if s == nil {
		return
	}
	next := 0
	if s.selectedColumn != nil {
		next = satAddInt(*s.selectedColumn, 1)
	}
	s.SelectColumn(&next)
}

// SelectPrevious selects the previous row, or math.MaxInt if nothing was selected
// (render clamps to the last row).
func (s *TableState) SelectPrevious() {
	if s == nil {
		return
	}
	prev := math.MaxInt
	if s.selected != nil {
		prev = satSubInt(*s.selected, 1)
	}
	s.Select(&prev)
}

// SelectPreviousColumn selects the previous column, or math.MaxInt if none.
func (s *TableState) SelectPreviousColumn() {
	if s == nil {
		return
	}
	prev := math.MaxInt
	if s.selectedColumn != nil {
		prev = satSubInt(*s.selectedColumn, 1)
	}
	s.SelectColumn(&prev)
}

// SelectFirst selects row 0.
func (s *TableState) SelectFirst() {
	if s == nil {
		return
	}
	zero := 0
	s.Select(&zero)
}

// SelectFirstColumn selects column 0.
func (s *TableState) SelectFirstColumn() {
	if s == nil {
		return
	}
	zero := 0
	s.SelectColumn(&zero)
}

// SelectLast selects math.MaxInt; render clamps to the last row.
func (s *TableState) SelectLast() {
	if s == nil {
		return
	}
	last := math.MaxInt
	s.Select(&last)
}

// SelectLastColumn selects math.MaxInt; render clamps to the last column.
func (s *TableState) SelectLastColumn() {
	if s == nil {
		return
	}
	last := math.MaxInt
	s.SelectColumn(&last)
}

// ScrollDownBy moves the row selection down by amount (saturating).
// If nothing is selected, starts from 0.
func (s *TableState) ScrollDownBy(amount int) {
	if s == nil {
		return
	}
	if amount < 0 {
		amount = 0
	}
	selected := 0
	if s.selected != nil {
		selected = *s.selected
	}
	next := satAddInt(selected, amount)
	s.Select(&next)
}

// ScrollUpBy moves the row selection up by amount (saturating).
// If nothing is selected, starts from 0.
func (s *TableState) ScrollUpBy(amount int) {
	if s == nil {
		return
	}
	if amount < 0 {
		amount = 0
	}
	selected := 0
	if s.selected != nil {
		selected = *s.selected
	}
	next := satSubInt(selected, amount)
	s.Select(&next)
}

// ScrollRightBy moves the column selection right by amount (saturating).
func (s *TableState) ScrollRightBy(amount int) {
	if s == nil {
		return
	}
	if amount < 0 {
		amount = 0
	}
	selected := 0
	if s.selectedColumn != nil {
		selected = *s.selectedColumn
	}
	next := satAddInt(selected, amount)
	s.SelectColumn(&next)
}

// ScrollLeftBy moves the column selection left by amount (saturating).
func (s *TableState) ScrollLeftBy(amount int) {
	if s == nil {
		return
	}
	if amount < 0 {
		amount = 0
	}
	selected := 0
	if s.selectedColumn != nil {
		selected = *s.selectedColumn
	}
	next := satSubInt(selected, amount)
	s.SelectColumn(&next)
}

func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func satAddInt(a, b int) int {
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	sum := a + b
	if sum < a {
		return math.MaxInt
	}
	return sum
}

func satSubInt(a, b int) int {
	if b < 0 {
		b = 0
	}
	if a <= b {
		return 0
	}
	return a - b
}
