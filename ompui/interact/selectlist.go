package interact

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/fuzzy"
)

const (
	defaultPrimaryColumnWidth = 32
	primaryColumnGap          = 2
	minDescriptionWidth       = 10
)

// SelectItem is one row in a SelectList.
type SelectItem struct {
	Value       string
	Label       string
	Description string
	// Hint is dim text shown inline when selected (optional).
	Hint string
	// Disabled rows are skipped by keyboard navigation and cannot be confirmed.
	Disabled bool
}

// SelectListTheme styles the picker.
type SelectListTheme struct {
	SelectedPrefix Style
	SelectedText   Style
	Description    Style
	ScrollInfo     Style
	NoMatch        Style
	// Cursor is the selection glyph (default "❯").
	Cursor string
	// Hovered paints the full row under the pointer.
	Hovered Style
}

// SelectListTruncatePrimaryContext is passed to a custom primary truncator.
type SelectListTruncatePrimaryContext struct {
	Text        string
	MaxWidth    int
	ColumnWidth int
	Item        SelectItem
	IsSelected  bool
}

// SelectListLayoutOptions tunes column widths and search.
type SelectListLayoutOptions struct {
	MinPrimaryColumnWidth int
	MaxPrimaryColumnWidth int
	TruncatePrimary       func(SelectListTruncatePrimaryContext) string
	// OverflowSearch enables type-to-filter when item count exceeds MaxVisible.
	// Default true (zero value means enabled; set OverflowSearchOff to disable).
	OverflowSearchOff bool
	// WrapDescription wraps long descriptions onto continuation rows.
	WrapDescription bool
	// VimKeys enables j/k navigation.
	VimKeys bool
}

// SelectList is a generic filterable picker.
type SelectList[T any] struct {
	keyBindings

	items       []T
	toItem      func(T) SelectItem
	filtered    []T
	filterQuery string
	selected    int
	hovered     int // -1 = none
	maxVisible  int
	theme       SelectListTheme
	layout      SelectListLayoutOptions

	OnSelect           func(item T)
	OnCancel           func()
	OnSelectionChange  func(item T)
	hitRows            []int // line → filtered index; -1 empty
	gen                component.Gen
	cached             component.Frame
	cacheW             int
	dirty              bool
}

// NewSelectList constructs a typed select list. toItem maps T → display fields.
// For the simple SelectItem case use NewSelectItemList.
func NewSelectList[T any](items []T, maxVisible int, theme SelectListTheme, toItem func(T) SelectItem, layout SelectListLayoutOptions) *SelectList[T] {
	if theme.Cursor == "" {
		theme.Cursor = "❯"
	}
	if maxVisible < 1 {
		maxVisible = 1
	}
	sl := &SelectList[T]{
		items:      append([]T(nil), items...),
		toItem:     toItem,
		maxVisible: maxVisible,
		theme:      theme,
		layout:     layout,
		hovered:    -1,
		dirty:      true,
	}
	if sl.toItem == nil {
		sl.toItem = func(v T) SelectItem {
			if it, ok := any(v).(SelectItem); ok {
				return it
			}
			return SelectItem{}
		}
	}
	sl.filtered = append([]T(nil), sl.items...)
	return sl
}

// NewSelectItemList is the non-generic convenience constructor.
func NewSelectItemList(items []SelectItem, maxVisible int, theme SelectListTheme, layout SelectListLayoutOptions) *SelectList[SelectItem] {
	return NewSelectList(items, maxVisible, theme, func(it SelectItem) SelectItem { return it }, layout)
}

// SetItems replaces the item set and resets the filter.
func (sl *SelectList[T]) SetItems(items []T) {
	sl.items = append([]T(nil), items...)
	sl.setFilter(sl.filterQuery, false)
	sl.dirty = true
}

// SetFilter applies a fuzzy filter query.
func (sl *SelectList[T]) SetFilter(filter string) {
	sl.setFilter(filter, true)
}

// FilterQuery returns the current search string.
func (sl *SelectList[T]) FilterQuery() string { return sl.filterQuery }

// SetSelectedIndex clamps the selection.
func (sl *SelectList[T]) SetSelectedIndex(index int) {
	if len(sl.filtered) == 0 {
		sl.selected = 0
		return
	}
	sl.selected = clampInt(index, 0, len(sl.filtered)-1)
	sl.dirty = true
}

// SelectedIndex returns the filtered selection index.
func (sl *SelectList[T]) SelectedIndex() int { return sl.selected }

// SelectedItem returns the selected filtered item and true when present.
func (sl *SelectList[T]) SelectedItem() (T, bool) {
	var zero T
	if len(sl.filtered) == 0 || sl.selected < 0 || sl.selected >= len(sl.filtered) {
		return zero, false
	}
	return sl.filtered[sl.selected], true
}

// HitTest maps a rendered line to a filtered-item index.
func (sl *SelectList[T]) HitTest(line int) (int, bool) {
	if line < 0 || line >= len(sl.hitRows) {
		return 0, false
	}
	idx := sl.hitRows[line]
	if idx < 0 {
		return 0, false
	}
	return idx, true
}

// SetHoverIndex highlights an item (-1 clears).
func (sl *SelectList[T]) SetHoverIndex(index int) {
	sl.hovered = index
	sl.dirty = true
}

// HandleWheel moves selection by one step without wrap.
func (sl *SelectList[T]) HandleWheel(delta int) {
	if len(sl.filtered) == 0 {
		return
	}
	next := clampInt(sl.selected+delta, 0, len(sl.filtered)-1)
	if next == sl.selected {
		return
	}
	sl.selected = next
	sl.dirty = true
	sl.notifySelectionChange()
}

// ClickItem selects and confirms the item at filtered index.
func (sl *SelectList[T]) ClickItem(index int) {
	if index < 0 || index >= len(sl.filtered) {
		return
	}
	item := sl.toItem(sl.filtered[index])
	if item.Disabled {
		return
	}
	if index != sl.selected {
		sl.selected = index
		sl.notifySelectionChange()
	}
	if sl.OnSelect != nil {
		sl.OnSelect(sl.filtered[index])
	}
	sl.dirty = true
}

// HandleInput implements component.InputHandler.
func (sl *SelectList[T]) HandleInput(ev event.Event) {
	if sl.isCancel(ev) {
		if sl.OnCancel != nil {
			sl.OnCancel()
		}
		return
	}
	if sl.handleSearchInput(ev) {
		return
	}
	if len(sl.filtered) == 0 {
		return
	}
	if sl.match(ev, actSelectUp) || (sl.layout.VimKeys && matchKeys(ev, keysVimUp)) {
		sl.moveSel(-1, true)
		return
	}
	if sl.match(ev, actSelectDown) || (sl.layout.VimKeys && matchKeys(ev, keysVimDown)) {
		sl.moveSel(1, true)
		return
	}
	if sl.match(ev, actSelectPageUp) {
		sl.selected = maxInt(0, sl.selected-sl.maxVisible)
		sl.skipDisabled(-1)
		sl.dirty = true
		sl.notifySelectionChange()
		return
	}
	if sl.match(ev, actSelectPageDown) {
		sl.selected = minInt(len(sl.filtered)-1, sl.selected+sl.maxVisible)
		sl.skipDisabled(1)
		sl.dirty = true
		sl.notifySelectionChange()
		return
	}
	if sl.isConfirm(ev) {
		item, ok := sl.SelectedItem()
		if !ok {
			return
		}
		if sl.toItem(item).Disabled {
			return
		}
		if sl.OnSelect != nil {
			sl.OnSelect(item)
		}
	}
}

func (sl *SelectList[T]) moveSel(delta int, wrap bool) {
	n := len(sl.filtered)
	if n == 0 {
		return
	}
	for step := 0; step < n; step++ {
		next := sl.selected + delta
		if next < 0 || next >= n {
			if !wrap {
				return
			}
			if next < 0 {
				next = n - 1
			} else {
				next = 0
			}
		}
		sl.selected = next
		if !sl.toItem(sl.filtered[sl.selected]).Disabled {
			sl.dirty = true
			sl.notifySelectionChange()
			return
		}
	}
}

func (sl *SelectList[T]) skipDisabled(dir int) {
	n := len(sl.filtered)
	if n == 0 {
		return
	}
	for step := 0; step < n; step++ {
		if !sl.toItem(sl.filtered[sl.selected]).Disabled {
			return
		}
		sl.selected = (sl.selected + dir + n) % n
	}
}

// Invalidate implements component.Invalidator.
func (sl *SelectList[T]) Invalidate() {
	sl.dirty = true
	sl.cached = component.Frame{}
	sl.cacheW = -1
}

// Dispose implements component.Disposable.
func (sl *SelectList[T]) Dispose() {
	sl.OnSelect = nil
	sl.OnCancel = nil
	sl.OnSelectionChange = nil
}

// Render implements component.Component.
func (sl *SelectList[T]) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	lines := make([]string, 0, sl.maxVisible+2)
	sl.hitRows = sl.hitRows[:0]
	showSearch := sl.shouldRenderSearchStatus()

	if len(sl.filtered) == 0 {
		if showSearch {
			lines = append(lines, sl.renderStatusLine(width))
		}
		lines = append(lines, applyStyle(sl.theme.NoMatch, "  No matching items"))
		return sl.finish(width, lines)
	}

	primaryCol := sl.primaryColumnWidth()
	wrapEnabled := sl.layout.WrapDescription
	visualBudget := sl.maxVisible

	conservativeW := maxInt(0, width-1)
	rowCounts := make([]int, len(sl.filtered))
	visualTotal := 0
	for i, it := range sl.filtered {
		item := sl.toItem(it)
		if wrapEnabled {
			rowCounts[i] = sl.computeItemRowCount(item, conservativeW, primaryCol)
		} else {
			rowCounts[i] = 1
		}
		visualTotal += rowCounts[i]
	}
	overflow := visualTotal > visualBudget
	rowWidth := width
	if overflow {
		rowWidth = maxInt(0, width-1)
	}

	start, end, visualOffset := sl.pickWindow(rowCounts, visualBudget)
	rows := make([]string, 0, visualBudget)
	hitForRow := make([]int, 0, visualBudget)
	for i := start; i < end && len(rows) < visualBudget; i++ {
		item := sl.toItem(sl.filtered[i])
		hovered := sl.theme.Hovered != nil && i == sl.hovered && i != sl.selected
		itemRows := sl.renderItem(item, i == sl.selected, rowWidth, primaryCol)
		for _, row := range itemRows {
			if len(rows) >= visualBudget {
				break
			}
			if hovered {
				row = applyStyle(sl.theme.Hovered, row)
			}
			hitForRow = append(hitForRow, i)
			rows = append(rows, row)
		}
	}

	sv := NewScrollView(rows, ScrollViewOptions{
		Height:       len(rows),
		Scrollbar:    ScrollbarAuto,
		HasTotalRows: true,
		TotalRows:    visualTotal,
		Theme: ScrollViewTheme{
			Track: sl.theme.ScrollInfo,
			Thumb: sl.theme.SelectedPrefix,
		},
		Ellipsis: "…",
	})
	// OMP sets ellipsis default unicode; for list rows we already truncated.
	sv.SetEllipsis("…")
	sv.SetScrollOffset(visualOffset)
	// Re-enable follow false — SetScrollOffset may clear follow; fine.
	fr := sv.Render(width)
	for _, ln := range fr.Lines {
		sl.hitRows = append(sl.hitRows, -1)
		lines = append(lines, ln)
	}
	// Map hit rows: ScrollView with HasTotalRows uses sourceIndex=row, so line i
	// corresponds to rows[i].
	for i := 0; i < len(fr.Lines) && i < len(hitForRow); i++ {
		sl.hitRows[i] = hitForRow[i]
	}

	if showSearch {
		lines = append(lines, sl.renderStatusLine(width))
		sl.hitRows = append(sl.hitRows, -1)
	}
	return sl.finish(width, lines)
}

func (sl *SelectList[T]) finish(width int, lines []string) component.Frame {
	changed := sl.dirty || sl.cacheW != width || !sameLines(sl.cached.Lines, lines)
	gen := sl.gen.Touch(changed)
	sl.dirty = false
	sl.cacheW = width
	if !changed && sl.cached.Lines != nil {
		return sl.cached
	}
	sl.cached = component.NewFrame(lines, gen)
	return sl.cached
}

type selectItemLayout struct {
	kind                 string // "description" | "primary"
	prefix               string
	truncatedValue       string
	spacing              string
	descriptionSingleLine string
	descriptionStart     int
	remainingWidth       int
}

func (sl *SelectList[T]) renderItem(item SelectItem, isSelected bool, width, primaryCol int) []string {
	layout := sl.computeItemLayout(item, isSelected, width, primaryCol)
	if layout.kind == "description" {
		if sl.layout.WrapDescription {
			wrapped := ansitext.WrapANSI(layout.descriptionSingleLine, layout.remainingWidth)
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}
			indent := padSpaces(layout.descriptionStart)
			first := wrapped[0]
			if isSelected {
				rows := []string{applyStyle(sl.theme.SelectedText, layout.prefix+layout.truncatedValue+layout.spacing+first)}
				for i := 1; i < len(wrapped); i++ {
					rows = append(rows, applyStyle(sl.theme.SelectedText, indent+wrapped[i]))
				}
				return rows
			}
			rows := []string{layout.prefix + layout.truncatedValue + applyStyle(sl.theme.Description, layout.spacing+first)}
			for i := 1; i < len(wrapped); i++ {
				rows = append(rows, applyStyle(sl.theme.Description, indent+wrapped[i]))
			}
			return rows
		}
		truncDesc := ansitext.TruncateToWidth(layout.descriptionSingleLine, layout.remainingWidth, "")
		if isSelected {
			return []string{applyStyle(sl.theme.SelectedText, layout.prefix+layout.truncatedValue+layout.spacing+truncDesc)}
		}
		return []string{layout.prefix + layout.truncatedValue + applyStyle(sl.theme.Description, layout.spacing+truncDesc)}
	}
	if isSelected {
		return []string{applyStyle(sl.theme.SelectedText, layout.prefix+layout.truncatedValue)}
	}
	return []string{layout.prefix + layout.truncatedValue}
}

func (sl *SelectList[T]) computeItemRowCount(item SelectItem, width, primaryCol int) int {
	layout := sl.computeItemLayout(item, false, width, primaryCol)
	if layout.kind != "description" {
		return 1
	}
	wrapped := ansitext.WrapANSI(layout.descriptionSingleLine, layout.remainingWidth)
	return maxInt(1, len(wrapped))
}

func (sl *SelectList[T]) pickWindow(rowCounts []int, budget int) (start, end, visualOffset int) {
	n := len(rowCounts)
	if n == 0 {
		return 0, 0, 0
	}
	selected := clampInt(sl.selected, 0, n-1)
	half := budget / 2
	lo := selected
	rowsAbove := 0
	for lo > 0 && rowsAbove+rowCounts[lo-1] <= half {
		lo--
		rowsAbove += rowCounts[lo]
	}
	hi := selected + 1
	used := rowsAbove + rowCounts[selected]
	for hi < n && used+rowCounts[hi] <= budget {
		used += rowCounts[hi]
		hi++
	}
	for lo > 0 && used+rowCounts[lo-1] <= budget {
		lo--
		used += rowCounts[lo]
	}
	for i := 0; i < lo; i++ {
		visualOffset += rowCounts[i]
	}
	return lo, hi, visualOffset
}

func (sl *SelectList[T]) computeItemLayout(item SelectItem, isSelected bool, width, primaryCol int) selectItemLayout {
	cursor := sl.theme.Cursor
	var prefix string
	if isSelected {
		prefix = applyStyle(sl.theme.SelectedPrefix, cursor) + " "
	} else {
		prefix = padSpaces(ansitext.VisibleWidth(cursor) + 1)
	}
	prefixW := ansitext.VisibleWidth(prefix)
	var desc string
	if item.Description != "" {
		desc = sanitizeSingleLine(item.Description)
	}
	if desc != "" && width > 40 {
		effPrimary := maxInt(1, minInt(primaryCol, width-prefixW-4))
		maxPrimary := maxInt(1, effPrimary-primaryColumnGap)
		truncVal := sl.truncatePrimary(item, isSelected, maxPrimary, effPrimary)
		truncW := ansitext.VisibleWidth(truncVal)
		spacing := padSpaces(maxInt(1, effPrimary-truncW))
		descStart := prefixW + truncW + len(spacing)
		remaining := width - descStart - 2
		if remaining > minDescriptionWidth {
			return selectItemLayout{
				kind:                  "description",
				prefix:                prefix,
				truncatedValue:        truncVal,
				spacing:               spacing,
				descriptionSingleLine: desc,
				descriptionStart:      descStart,
				remainingWidth:        remaining,
			}
		}
	}
	fallbackMax := width - prefixW - 2
	truncVal := sl.truncatePrimary(item, isSelected, fallbackMax, fallbackMax)
	return selectItemLayout{
		kind:            "primary",
		prefix:          prefix,
		truncatedValue:  truncVal,
		spacing:         "",
	}
}

func (sl *SelectList[T]) primaryColumnWidth() int {
	minW, maxW := sl.primaryColumnBounds()
	widest := 0
	for _, it := range sl.filtered {
		item := sl.toItem(it)
		w := ansitext.VisibleWidth(sl.displayValue(item)) + primaryColumnGap
		if w > widest {
			widest = w
		}
	}
	return clampInt(widest, minW, maxW)
}

func (sl *SelectList[T]) primaryColumnBounds() (minW, maxW int) {
	rawMin := sl.layout.MinPrimaryColumnWidth
	rawMax := sl.layout.MaxPrimaryColumnWidth
	if rawMin == 0 && rawMax == 0 {
		return defaultPrimaryColumnWidth, defaultPrimaryColumnWidth
	}
	if rawMin == 0 {
		rawMin = rawMax
	}
	if rawMax == 0 {
		rawMax = rawMin
	}
	minW = maxInt(1, minInt(rawMin, rawMax))
	maxW = maxInt(1, maxInt(rawMin, rawMax))
	return minW, maxW
}

func (sl *SelectList[T]) truncatePrimary(item SelectItem, isSelected bool, maxWidth, columnWidth int) string {
	display := sl.displayValue(item)
	var truncated string
	if sl.layout.TruncatePrimary != nil {
		truncated = sl.layout.TruncatePrimary(SelectListTruncatePrimaryContext{
			Text: display, MaxWidth: maxWidth, ColumnWidth: columnWidth, Item: item, IsSelected: isSelected,
		})
	} else {
		truncated = ansitext.TruncateToWidth(display, maxWidth, "")
	}
	return ansitext.TruncateToWidth(truncated, maxWidth, "")
}

func (sl *SelectList[T]) displayValue(item SelectItem) string {
	if item.Label != "" {
		return sanitizeSingleLine(item.Label)
	}
	return sanitizeSingleLine(item.Value)
}

func (sl *SelectList[T]) renderStatusLine(width int) string {
	query := sanitizeSingleLine(sl.filterQuery)
	status := "  Type to search"
	if query != "" {
		status = "  Search: " + query
	}
	return applyStyle(sl.theme.ScrollInfo, ansitext.TruncateToWidth(status, maxInt(1, width-2), ""))
}

func (sl *SelectList[T]) shouldRenderSearchStatus() bool {
	if sl.layout.OverflowSearchOff {
		return false
	}
	return len(sl.items) > sl.maxVisible || len(sl.filterQuery) > 0
}

func (sl *SelectList[T]) canEditSearch() bool {
	return !sl.layout.OverflowSearchOff && len(sl.items) > sl.maxVisible
}

func (sl *SelectList[T]) handleSearchInput(ev event.Event) bool {
	if !sl.canEditSearch() {
		return false
	}
	if sl.match(ev, actDeleteCharBackward) {
		if sl.filterQuery == "" {
			return false
		}
		sl.setFilter(dropLastGrapheme(sl.filterQuery), true)
		return true
	}
	t := PrintableText(ev)
	if t == "" {
		return false
	}
	if sl.filterQuery == "" && strings.TrimSpace(t) == "" {
		return false
	}
	sl.setFilter(sl.filterQuery+t, true)
	return true
}

func (sl *SelectList[T]) setFilter(filter string, notify bool) {
	sl.filterQuery = filter
	if sanitizeSingleLine(filter) != "" {
		sl.filtered = fuzzy.Filter(sl.items, filter, func(it T) string {
			return sl.filterText(sl.toItem(it))
		})
	} else {
		sl.filtered = append([]T(nil), sl.items...)
	}
	sl.selected = 0
	sl.skipDisabled(1)
	sl.dirty = true
	if notify {
		sl.notifySelectionChange()
	}
}

func (sl *SelectList[T]) filterText(item SelectItem) string {
	text := item.Label + " " + item.Value
	if item.Description != "" {
		text += " " + item.Description
	}
	if item.Hint != "" {
		text += " " + item.Hint
	}
	return sanitizeSingleLine(text)
}

func (sl *SelectList[T]) notifySelectionChange() {
	item, ok := sl.SelectedItem()
	if !ok || sl.OnSelectionChange == nil {
		return
	}
	sl.OnSelectionChange(item)
}
