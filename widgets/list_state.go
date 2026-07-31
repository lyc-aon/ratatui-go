package widgets

import "math"

// ListState holds selection and scroll offset for a List.
//
// Fields are unexported; use accessors. RenderStateful repairs out-of-range
// values: selection clamps to the last item, offset moves so the selection
// stays visible (with scroll padding).
type ListState struct {
	offset   int
	selected *int
}

// NewListState returns a default state (no selection, offset 0).
func NewListState() ListState {
	return ListState{}
}

// WithOffset returns a copy with the given first-visible index.
func (s ListState) WithOffset(offset int) ListState {
	if offset < 0 {
		offset = 0
	}
	s.offset = offset
	return s
}

// WithSelected returns a copy with the given selection.
// A nil index means no selection (offset is left unchanged here; Select(nil)
// is the mutating path that also resets offset).
func (s ListState) WithSelected(index *int) ListState {
	if index == nil {
		s.selected = nil
		return s
	}
	v := *index
	if v < 0 {
		v = 0
	}
	s.selected = &v
	return s
}

// Offset returns the index of the first visible item.
func (s ListState) Offset() int {
	return s.offset
}

// SetOffset sets the index of the first visible item.
func (s *ListState) SetOffset(offset int) {
	if s == nil {
		return
	}
	if offset < 0 {
		offset = 0
	}
	s.offset = offset
}

// Selected returns the selected item index, or nil if none.
func (s ListState) Selected() *int {
	if s.selected == nil {
		return nil
	}
	v := *s.selected
	return &v
}

// Select sets the selected index. Passing nil clears the selection and resets
// offset to 0 (matching Ratatui ListState::select(None)).
func (s *ListState) Select(index *int) {
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

// SelectNext selects the next item, or 0 when nothing was selected.
// Until render, the item count is unknown so the index is not clamped here.
func (s *ListState) SelectNext() {
	if s == nil {
		return
	}
	next := 0
	if s.selected != nil {
		if *s.selected == math.MaxInt {
			next = math.MaxInt
		} else {
			next = *s.selected + 1
		}
	}
	s.Select(&next)
}

// SelectPrevious selects the previous item, or MaxInt when nothing was selected
// (render clamps to the last real item).
func (s *ListState) SelectPrevious() {
	if s == nil {
		return
	}
	prev := math.MaxInt
	if s.selected != nil {
		prev = *s.selected - 1
		if prev < 0 {
			prev = 0
		}
	}
	s.Select(&prev)
}

// SelectFirst selects index 0.
func (s *ListState) SelectFirst() {
	if s == nil {
		return
	}
	z := 0
	s.Select(&z)
}

// SelectLast selects math.MaxInt; RenderStateful clamps to the last item.
func (s *ListState) SelectLast() {
	if s == nil {
		return
	}
	last := math.MaxInt
	s.Select(&last)
}

// ScrollDownBy moves the selection down by amount (saturating add).
func (s *ListState) ScrollDownBy(amount int) {
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
	// saturating add without wrap
	if amount > math.MaxInt-selected {
		selected = math.MaxInt
	} else {
		selected += amount
	}
	s.Select(&selected)
}

// ScrollUpBy moves the selection up by amount (saturating sub to 0).
func (s *ListState) ScrollUpBy(amount int) {
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
	if amount >= selected {
		selected = 0
	} else {
		selected -= amount
	}
	s.Select(&selected)
}
