package interact

import (
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/event"
	"github.com/michaelkelly/ratatui-go/ompui/fuzzy"
)

// SettingItem is one settings row.
type SettingItem struct {
	ID           string
	Label        string
	Description  string
	CurrentValue string
	// Values when set: Enter/Space cycles through them.
	Values []string
	// Submenu when set: Enter opens this component. done(selected) closes it.
	// Prefer SetSubmenuFactory on the list for retained wiring; this field is
	// the per-item factory matching OMP.
	Submenu func(currentValue string, done func(selectedValue string, ok bool)) component.Component
	Changed bool
	// Heading rows are non-interactive section titles.
	Heading bool
}

// SettingsListTheme styles the settings panel.
type SettingsListTheme struct {
	Label       func(text string, selected, changed bool) string
	Value       func(text string, selected, changed bool) string
	Description Style
	Cursor      string // default "❯ "
	Hint        Style
	Heading     func(text string, dimmed bool) string // falls back to Hint
	Section     func(text string, active bool) string
	Hovered     Style
}

// SettingsListOptions tunes layout and search.
type SettingsListOptions struct {
	// Layout "auto" (default) uses split sidebar when headings fit; "flat" forces inline.
	Layout string
	// TypeToSearch when false disables internal filter and status line. Default true.
	TypeToSearch *bool
	EmptyText    string
	// Hint footer; empty string removes the hint row entirely. nil → default text.
	Hint *string
	// SidebarWidth fixed split width; 0 derives from section names.
	SidebarWidth int
	// VimKeys enables j/k.
	VimKeys bool
}

func (o SettingsListOptions) typeToSearchEnabled() bool {
	if o.TypeToSearch == nil {
		return true
	}
	return *o.TypeToSearch
}

// SettingItemFilterText is the fuzzy corpus for one item.
func SettingItemFilterText(item SettingItem) string {
	text := item.Label + " " + item.ID + " " + item.CurrentValue
	if item.Description != "" {
		text += " " + item.Description
	}
	if len(item.Values) > 0 {
		text += " " + strings.Join(item.Values, " ")
	}
	return sanitizeSingleLine(text)
}

type settingSection struct {
	name          string
	firstItemIndex int
	lastItemIndex  int
}

// SettingsList is a searchable settings panel with optional split sidebar,
// section focus, value cycling, and submenu hosting.
type SettingsList struct {
	items          []SettingItem
	filtered       []SettingItem
	theme          SettingsListTheme
	selected       int
	maxVisible     int
	onChange       func(id, newValue string)
	onCancel       func()
	options        SettingsListOptions
	filterQuery    string
	sectionFocus   bool
	lastNotifiedID string

	OnSelectionChange func(item *SettingItem)

	submenu     component.Component
	submenuID   string
	hoveredID   string
	hitRows     []string // line → item id; "" empty
	sidebarHits []string
	sidebarCol  int

	gen    component.Gen
	cached component.Frame
	cacheW int
	dirty  bool
}

// NewSettingsList constructs a settings list.
func NewSettingsList(
	items []SettingItem,
	maxVisible int,
	theme SettingsListTheme,
	onChange func(id, newValue string),
	onCancel func(),
	options SettingsListOptions,
) *SettingsList {
	if maxVisible < 3 {
		maxVisible = 3
	}
	if theme.Cursor == "" {
		theme.Cursor = "❯ "
	}
	sl := &SettingsList{
		items:      append([]SettingItem(nil), items...),
		maxVisible: maxVisible,
		theme:      theme,
		onChange:   onChange,
		onCancel:   onCancel,
		options:    options,
		dirty:      true,
	}
	sl.filtered = append([]SettingItem(nil), sl.items...)
	sl.selected = sl.firstSelectableIndex()
	if it := sl.SelectedItem(); it != nil {
		sl.lastNotifiedID = it.ID
	}
	return sl
}

// SelectedItem returns the selected non-heading item, or nil.
func (sl *SettingsList) SelectedItem() *SettingItem {
	if sl.selected < 0 || sl.selected >= len(sl.filtered) {
		return nil
	}
	it := &sl.filtered[sl.selected]
	if it.Heading {
		return nil
	}
	return it
}

// SelectItem moves selection to id. Returns false when not visible.
func (sl *SettingsList) SelectItem(id string) bool {
	for i, it := range sl.filtered {
		if !it.Heading && it.ID == id {
			sl.sectionFocus = false
			sl.selected = i
			sl.notifySelection()
			sl.dirty = true
			return true
		}
	}
	return false
}

// SectionFocused reports section-heading keyboard focus.
func (sl *SettingsList) SectionFocused() bool { return sl.sectionFocus }

// HasSectionFocusTargets reports whether section focus is meaningful.
func (sl *SettingsList) HasSectionFocusTargets() bool {
	return len(sl.sections()) >= 2
}

// ToggleSectionFocus toggles section vs row focus.
func (sl *SettingsList) ToggleSectionFocus() bool {
	sl.sectionFocus = !sl.sectionFocus && sl.HasSectionFocusTargets()
	sl.dirty = true
	return sl.sectionFocus
}

// HasOpenSubmenu reports submenu ownership of input.
func (sl *SettingsList) HasOpenSubmenu() bool { return sl.submenu != nil }

// SetMaxVisible resizes the viewport (min 3).
func (sl *SettingsList) SetMaxVisible(rows int) {
	next := maxInt(3, rows)
	if next == sl.maxVisible {
		return
	}
	sl.maxVisible = next
	sl.clampSelectedIndex()
	sl.dirty = true
}

// HandleWheel moves selection without wrap; clears section focus.
func (sl *SettingsList) HandleWheel(delta int) {
	if sl.submenu != nil {
		return
	}
	sl.sectionFocus = false
	sl.moveSelection(delta, false)
}

// SetHoverItem highlights by id ("" clears).
func (sl *SettingsList) SetHoverItem(id string) {
	sl.hoveredID = id
	sl.dirty = true
}

// HitTest resolves pointer line/col to an item id.
func (sl *SettingsList) HitTest(line, col int) string {
	if sl.submenu != nil {
		return ""
	}
	if sl.sidebarCol > 0 && col < sl.sidebarCol {
		if line >= 0 && line < len(sl.sidebarHits) {
			return sl.sidebarHits[line]
		}
		return ""
	}
	if line >= 0 && line < len(sl.hitRows) {
		return sl.hitRows[line]
	}
	return ""
}

// HoverTest is HitTest but ignores sidebar jump targets.
func (sl *SettingsList) HoverTest(line, col int) string {
	if sl.submenu != nil {
		return ""
	}
	if sl.sidebarCol > 0 && col < sl.sidebarCol {
		return ""
	}
	if line >= 0 && line < len(sl.hitRows) {
		return sl.hitRows[line]
	}
	return ""
}

// SearchQuery returns the filter string.
func (sl *SettingsList) SearchQuery() string { return sl.filterQuery }

// HasSearchQuery reports a non-empty filter.
func (sl *SettingsList) HasSearchQuery() bool { return len(sl.filterQuery) > 0 }

// ClearSearch drops the filter.
func (sl *SettingsList) ClearSearch() {
	if sl.filterQuery == "" {
		return
	}
	sl.setFilter("")
}

// UpdateValue sets currentValue for id.
func (sl *SettingsList) UpdateValue(id, newValue string) {
	for i := range sl.items {
		if sl.items[i].ID == id {
			sl.items[i].CurrentValue = newValue
			break
		}
	}
	if strings.TrimSpace(sl.filterQuery) != "" {
		sl.applyFilter()
		sl.clampSelectedIndex()
	}
	sl.dirty = true
}

// SetItems replaces the items array, preserving selection by id when possible.
func (sl *SettingsList) SetItems(items []SettingItem) {
	var selectedID string
	if sl.selected >= 0 && sl.selected < len(sl.filtered) {
		selectedID = sl.filtered[sl.selected].ID
	}
	sl.items = append([]SettingItem(nil), items...)
	sl.applyFilter()
	if sl.sectionFocus && !sl.HasSectionFocusTargets() {
		sl.sectionFocus = false
	}
	next := -1
	if selectedID != "" {
		for i, it := range sl.filtered {
			if it.ID == selectedID {
				next = i
				break
			}
		}
	}
	if next >= 0 {
		sl.selected = next
	} else {
		sl.clampSelectedIndex()
	}
	sl.notifySelection()
	sl.dirty = true
}

// HandleInput implements component.InputHandler.
func (sl *SettingsList) HandleInput(ev event.Event) {
	if sl.submenu != nil {
		component.RouteInput(sl.submenu, ev)
		return
	}
	if IsCancel(ev) {
		if sl.filterQuery != "" {
			sl.ClearSearch()
			return
		}
		if sl.sectionFocus {
			sl.sectionFocus = false
			sl.dirty = true
			return
		}
		if sl.onCancel != nil {
			sl.onCancel()
		}
		return
	}
	if sl.handleSearchInput(ev) {
		return
	}
	if len(sl.filtered) == 0 {
		return
	}
	up := matchKeys(ev, keysSelectUp) || (sl.options.VimKeys && matchKeys(ev, keysVimUp))
	down := matchKeys(ev, keysSelectDown) || (sl.options.VimKeys && matchKeys(ev, keysVimDown))
	switch {
	case up:
		if sl.sectionFocus {
			sl.jumpSection(-1)
		} else {
			sl.moveSelection(-1, true)
		}
	case down:
		if sl.sectionFocus {
			sl.jumpSection(1)
		} else {
			sl.moveSelection(1, true)
		}
	case matchKeys(ev, keysSelectPageDown):
		sl.jumpSection(1)
	case matchKeys(ev, keysSelectPageUp):
		sl.jumpSection(-1)
	case IsEnter(ev) || IsSpace(ev):
		if sl.sectionFocus {
			sl.sectionFocus = false
			sl.dirty = true
		} else {
			sl.activateItem()
		}
	}
}

// OwnsOverlayFocusTarget reports submenu ownership.
func (sl *SettingsList) OwnsOverlayFocusTarget(c component.Component) bool {
	if c == nil {
		return false
	}
	if c == sl {
		return true
	}
	if sl.submenu == nil {
		return false
	}
	return component.IsOverlayFocusTarget(sl.submenu, c) || sl.submenu == c
}

// Invalidate implements component.Invalidator.
func (sl *SettingsList) Invalidate() {
	sl.dirty = true
	sl.cached = component.Frame{}
	sl.cacheW = -1
	component.InvalidateOne(sl.submenu)
}

// Dispose implements component.Disposable.
func (sl *SettingsList) Dispose() {
	component.DisposeOne(sl.submenu)
	sl.submenu = nil
	sl.onChange = nil
	sl.onCancel = nil
	sl.OnSelectionChange = nil
}

// Render implements component.Component.
func (sl *SettingsList) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	sl.hitRows = sl.hitRows[:0]
	sl.sidebarHits = sl.sidebarHits[:0]
	sl.sidebarCol = 0

	var lines []string
	if sl.submenu != nil {
		fr := sl.submenu.Render(width)
		lines = append([]string(nil), fr.Lines...)
		lines = sl.padLines(lines)
		return sl.finish(width, lines, fr.Cursor)
	}
	lines = sl.padLines(sl.renderMainList(width))
	return sl.finish(width, lines, nil)
}

func (sl *SettingsList) finish(width int, lines []string, cur *component.Cursor) component.Frame {
	changed := sl.dirty || sl.cacheW != width || !sameLines(sl.cached.Lines, lines) || !cursorEqual(sl.cached.Cursor, cur)
	gen := sl.gen.Touch(changed)
	sl.dirty = false
	sl.cacheW = width
	if !changed && sl.cached.Lines != nil {
		return sl.cached
	}
	fr := component.NewFrame(lines, gen).WithCursor(cur)
	sl.cached = fr
	return fr
}

func (sl *SettingsList) stableHeight() int {
	h := sl.maxVisible + 4 // viewport + blank + 3 desc
	if sl.options.typeToSearchEnabled() {
		h++
	}
	if sl.options.Hint == nil || *sl.options.Hint != "" {
		h += 2
	}
	return h
}

func (sl *SettingsList) padLines(lines []string) []string {
	target := sl.stableHeight()
	for len(lines) < target {
		lines = append(lines, "")
	}
	return lines
}

func (sl *SettingsList) renderMainList(width int) []string {
	lines := make([]string, 0, sl.stableHeight())
	if len(sl.items) == 0 {
		empty := sl.options.EmptyText
		if empty == "" {
			empty = "No settings available"
		}
		lines = append(lines, applyStyle(sl.theme.Hint, "  "+empty))
		return lines
	}
	if len(sl.filtered) == 0 {
		if sl.shouldRenderSearchStatus() {
			lines = append(lines, sl.renderSearchStatus(width))
		}
		lines = append(lines, applyStyle(sl.theme.Hint, "  No matching settings"))
		lines = append(lines, "")
		lines = append(lines, ansitext.TruncateToWidth(applyStyle(sl.theme.Hint, "  Backspace to edit search · Esc to cancel"), width, ""))
		return lines
	}

	sections := sl.sections()
	var split []string
	if sl.options.Layout != "flat" && strings.TrimSpace(sl.filterQuery) == "" && len(sections) >= 2 {
		split = sl.renderSplitList(width, sections)
	}
	if split != nil {
		lines = append(lines, split...)
	} else {
		viewportHeight := minInt(sl.maxVisible, len(sl.filtered))
		startIndex := clampInt(sl.selected-viewportHeight/2, 0, maxInt(0, len(sl.filtered)-viewportHeight))
		maxLabel := sl.maxLabelWidth()
		overflow := len(sl.filtered) > viewportHeight
		rowWidth := width
		if overflow {
			rowWidth = maxInt(0, width-1)
		}
		active := sections[sl.activeSectionIndex(sections)]
		focusedHeading := -1
		if sl.sectionFocus && active.name != "" {
			focusedHeading = active.firstItemIndex - 1
		}
		itemRows := make([]string, 0, viewportHeight)
		for r := 0; r < viewportHeight; r++ {
			idx := startIndex + r
			if idx >= len(sl.filtered) {
				break
			}
			item := sl.filtered[idx]
			itemRows = append(itemRows, sl.renderItemRow(item, idx, maxLabel, rowWidth, false, idx == focusedHeading))
			if item.Heading {
				sl.hitRows = append(sl.hitRows, "")
			} else {
				sl.hitRows = append(sl.hitRows, item.ID)
			}
		}
		sv := NewScrollView(itemRows, ScrollViewOptions{
			Height:       viewportHeight,
			Scrollbar:    ScrollbarAuto,
			HasTotalRows: true,
			TotalRows:    len(sl.filtered),
			Theme: ScrollViewTheme{
				Track: sl.theme.Hint,
				Thumb: func(t string) string {
					if sl.theme.Label != nil {
						return sl.theme.Label(t, true, false)
					}
					return t
				},
			},
		})
		sv.SetScrollOffset(startIndex)
		fr := sv.Render(width)
		lines = append(lines, fr.Lines...)
		for len(lines) < sl.maxVisible {
			lines = append(lines, "")
			sl.hitRows = append(sl.hitRows, "")
		}
	}

	// Description: 1 blank + exactly 3 rows.
	lines = append(lines, "")
	descLines := make([]string, 0, 3)
	sel := sl.filtered[sl.selected]
	if sel.Description != "" && !sel.Heading {
		wrapped := ansitext.WrapANSI(sel.Description, maxInt(1, width-4))
		for i, ln := range wrapped {
			if i >= 3 {
				break
			}
			descLines = append(descLines, applyStyle(sl.theme.Description, "  "+ln))
		}
		if len(wrapped) > 3 && len(descLines) == 3 {
			descLines[2] = ansitext.TruncateToWidth(descLines[2]+"…", width, "")
		}
	}
	for len(descLines) < 3 {
		descLines = append(descLines, "")
	}
	lines = append(lines, descLines...)

	if sl.options.typeToSearchEnabled() {
		lines = append(lines, sl.renderSearchStatus(width))
	}
	if sl.options.Hint == nil || *sl.options.Hint != "" {
		lines = append(lines, "")
		jump := ""
		if len(sections) >= 2 {
			jump = "PgUp/PgDn to jump sections · "
		}
		hintText := "Enter/Space to change · " + jump + "Type to search · Esc to cancel"
		if sl.options.Hint != nil {
			hintText = *sl.options.Hint
		}
		lines = append(lines, ansitext.TruncateToWidth(applyStyle(sl.theme.Hint, "  "+hintText), width, ""))
	}
	return lines
}

func (sl *SettingsList) maxLabelWidth() int {
	maxL := 0
	for _, it := range sl.filtered {
		if it.Heading {
			continue
		}
		w := ansitext.VisibleWidth(it.Label)
		if w > maxL {
			maxL = w
		}
	}
	return minInt(30, maxL)
}

func (sl *SettingsList) renderItemRow(item SettingItem, index, maxLabel, rowWidth int, dimmed, headingCursor bool) string {
	if item.Heading {
		headingStyle := sl.theme.Heading
		prefix := "  "
		if headingCursor {
			prefix = sl.theme.Cursor
		}
		var body string
		if headingStyle != nil {
			body = headingStyle(item.Label, dimmed)
		} else {
			body = applyStyle(sl.theme.Hint, item.Label)
		}
		return ansitext.TruncateToWidth(prefix+body, maxInt(0, rowWidth), "")
	}
	isSelected := index == sl.selected && !sl.sectionFocus
	prefix := "  "
	if isSelected {
		prefix = sl.theme.Cursor
	}
	prefixW := ansitext.VisibleWidth(prefix)
	labelPadded := item.Label + padSpaces(maxInt(0, maxLabel-ansitext.VisibleWidth(item.Label)))
	sep := "  "
	valueMax := rowWidth - prefixW - maxLabel - ansitext.VisibleWidth(sep) - 2
	valuePlain := ansitext.TruncateToWidth(item.CurrentValue, maxInt(0, valueMax), "")
	hovered := !isSelected && sl.theme.Hovered != nil && item.ID == sl.hoveredID

	if dimmed && !isSelected {
		text := applyStyle(sl.theme.Hint, ansitext.TruncateToWidth("  "+labelPadded+sep+valuePlain, maxInt(0, rowWidth), ""))
		if hovered {
			return applyStyle(sl.theme.Hovered, text)
		}
		return text
	}
	var labelText, valueText string
	if sl.theme.Label != nil {
		labelText = sl.theme.Label(labelPadded, isSelected, item.Changed)
	} else {
		labelText = labelPadded
	}
	if sl.theme.Value != nil {
		valueText = sl.theme.Value(valuePlain, isSelected, item.Changed)
	} else {
		valueText = valuePlain
	}
	text := ansitext.TruncateToWidth(prefix+labelText+sep+valueText, maxInt(0, rowWidth), "")
	if hovered {
		return applyStyle(sl.theme.Hovered, text)
	}
	return text
}

func (sl *SettingsList) renderSplitList(width int, sections []settingSection) []string {
	nameWidth := 0
	names := make([]string, len(sections))
	for i, s := range sections {
		n := s.name
		if n == "" {
			n = "Other"
		}
		names[i] = n
		w := ansitext.VisibleWidth(n)
		if w > nameWidth {
			nameWidth = w
		}
	}
	sidebarWidth := sl.options.SidebarWidth
	if sidebarWidth <= 0 {
		sidebarWidth = minInt(22, nameWidth) + 4
	}
	paneWidth := width - sidebarWidth - 2
	if paneWidth < 60 {
		return nil
	}
	activeIndex := sl.activeSectionIndex(sections)
	active := sections[activeIndex]

	sectionStyle := sl.theme.Section
	if sectionStyle == nil {
		sectionStyle = func(text string, active bool) string {
			if active && sl.theme.Label != nil {
				return sl.theme.Label(text, true, false)
			}
			return applyStyle(sl.theme.Hint, text)
		}
	}
	sidebarRows := make([]string, len(names))
	for i, name := range names {
		label := ansitext.TruncateToWidth(name, maxInt(1, sidebarWidth-4), "")
		prefix := "  "
		if sl.sectionFocus && i == activeIndex {
			prefix = sl.theme.Cursor
		}
		pad := padSpaces(sidebarWidth - ansitext.VisibleWidth(prefix) - ansitext.VisibleWidth(label))
		sidebarRows[i] = prefix + sectionStyle(label, i == activeIndex) + pad
	}

	activeStart := active.firstItemIndex
	if active.name != "" {
		activeStart = active.firstItemIndex - 1
	}
	viewportHeight := minInt(sl.maxVisible, len(sl.filtered))
	startRow := clampInt(sl.selected-viewportHeight/2, 0, maxInt(0, len(sl.filtered)-viewportHeight))
	maxLabel := sl.maxLabelWidth()
	overflow := len(sl.filtered) > viewportHeight
	rowWidth := paneWidth
	if overflow {
		rowWidth = maxInt(0, paneWidth-1)
	}
	itemRows := make([]string, 0, viewportHeight)
	for r := 0; r < viewportHeight; r++ {
		idx := startRow + r
		if idx >= len(sl.filtered) {
			break
		}
		item := sl.filtered[idx]
		dimmed := idx < activeStart || idx > active.lastItemIndex
		itemRows = append(itemRows, sl.renderItemRow(item, idx, maxLabel, rowWidth, dimmed, false))
	}
	sv := NewScrollView(itemRows, ScrollViewOptions{
		Height:       viewportHeight,
		Scrollbar:    ScrollbarAuto,
		HasTotalRows: true,
		TotalRows:    len(sl.filtered),
		Theme: ScrollViewTheme{
			Track: sl.theme.Hint,
			Thumb: func(t string) string {
				if sl.theme.Label != nil {
					return sl.theme.Label(t, true, false)
				}
				return t
			},
		},
	})
	sv.SetScrollOffset(startRow)
	paneRows := sv.Render(paneWidth).Lines

	sl.sidebarCol = sidebarWidth
	sl.sidebarHits = make([]string, len(names))
	for i := range names {
		fi := sections[i].firstItemIndex
		if fi >= 0 && fi < len(sl.filtered) {
			sl.sidebarHits[i] = sl.filtered[fi].ID
		}
	}
	sl.hitRows = make([]string, viewportHeight)
	for r := 0; r < viewportHeight; r++ {
		idx := startRow + r
		if idx < len(sl.filtered) && !sl.filtered[idx].Heading {
			sl.hitRows[r] = sl.filtered[idx].ID
		}
	}

	sep := applyStyle(sl.theme.Hint, "│ ")
	height := maxInt(sl.maxVisible, len(sidebarRows))
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		left := padSpaces(sidebarWidth)
		if i < len(sidebarRows) {
			left = sidebarRows[i]
		}
		right := ""
		if i < len(paneRows) {
			right = paneRows[i]
		}
		lines = append(lines, ansitext.TruncateToWidth(left+sep+right, width, ""))
	}
	return lines
}

func (sl *SettingsList) notifySelection() {
	it := sl.SelectedItem()
	var id string
	if it != nil {
		id = it.ID
	}
	if id == sl.lastNotifiedID {
		return
	}
	sl.lastNotifiedID = id
	if sl.OnSelectionChange != nil {
		sl.OnSelectionChange(it)
	}
}

func (sl *SettingsList) setFilter(filter string) {
	sl.filterQuery = filter
	if strings.TrimSpace(filter) != "" {
		sl.sectionFocus = false
	}
	sl.applyFilter()
	sl.selected = sl.firstSelectableIndex()
	sl.notifySelection()
	sl.dirty = true
}

func (sl *SettingsList) applyFilter() {
	if strings.TrimSpace(sl.filterQuery) != "" {
		src := make([]SettingItem, 0, len(sl.items))
		for _, it := range sl.items {
			if !it.Heading {
				src = append(src, it)
			}
		}
		sl.filtered = fuzzy.Filter(src, sl.filterQuery, SettingItemFilterText)
	} else {
		sl.filtered = append([]SettingItem(nil), sl.items...)
	}
}

func (sl *SettingsList) firstSelectableIndex() int {
	for i, it := range sl.filtered {
		if !it.Heading {
			return i
		}
	}
	return 0
}

func (sl *SettingsList) moveSelection(delta int, wrap bool) {
	n := len(sl.filtered)
	if n == 0 {
		return
	}
	index := sl.selected
	for step := 0; step < n*2; step++ {
		next := index + delta
		if next < 0 || next >= n {
			if !wrap {
				return
			}
			next = (next + n) % n
		}
		index = next
		if !sl.filtered[index].Heading {
			sl.selected = index
			sl.notifySelection()
			sl.dirty = true
			return
		}
	}
}

func (sl *SettingsList) sections() []settingSection {
	var sections []settingSection
	var current *settingSection
	for i, item := range sl.filtered {
		if item.Heading {
			sections = append(sections, settingSection{name: item.Label, firstItemIndex: -1, lastItemIndex: -1})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil {
			sections = append(sections, settingSection{name: "", firstItemIndex: i, lastItemIndex: i})
			current = &sections[len(sections)-1]
		}
		if current.firstItemIndex < 0 {
			current.firstItemIndex = i
		}
		current.lastItemIndex = i
	}
	out := sections[:0]
	for _, s := range sections {
		if s.firstItemIndex >= 0 {
			out = append(out, s)
		}
	}
	return out
}

func (sl *SettingsList) activeSectionIndex(sections []settingSection) int {
	for i := len(sections) - 1; i >= 0; i-- {
		if sections[i].firstItemIndex <= sl.selected {
			return i
		}
	}
	return 0
}

func (sl *SettingsList) jumpSection(delta int) {
	sections := sl.sections()
	if len(sections) < 2 {
		n := len(sl.filtered)
		if n == 0 {
			return
		}
		sl.selected = clampInt(sl.selected+delta*sl.maxVisible, 0, n-1)
		sl.clampSelectedIndex()
	} else {
		next := (sl.activeSectionIndex(sections) + delta + len(sections)) % len(sections)
		sl.selected = sections[next].firstItemIndex
	}
	sl.notifySelection()
	sl.dirty = true
}

func (sl *SettingsList) clampSelectedIndex() {
	if len(sl.filtered) == 0 {
		sl.selected = 0
		return
	}
	sl.selected = clampInt(sl.selected, 0, len(sl.filtered)-1)
	if !sl.filtered[sl.selected].Heading {
		return
	}
	for i := sl.selected + 1; i < len(sl.filtered); i++ {
		if !sl.filtered[i].Heading {
			sl.selected = i
			return
		}
	}
	for i := sl.selected - 1; i >= 0; i-- {
		if !sl.filtered[i].Heading {
			sl.selected = i
			return
		}
	}
}

func (sl *SettingsList) renderSearchStatus(width int) string {
	query := sanitizeSingleLine(sl.filterQuery)
	status := "  Type to search"
	if query != "" {
		status = "  Search: " + query
	}
	return applyStyle(sl.theme.Hint, ansitext.TruncateToWidth(status, width, ""))
}

func (sl *SettingsList) shouldRenderSearchStatus() bool {
	if !sl.options.typeToSearchEnabled() {
		return false
	}
	return len(sl.items) > sl.maxVisible || len(sl.filterQuery) > 0
}

func (sl *SettingsList) handleSearchInput(ev event.Event) bool {
	if !sl.options.typeToSearchEnabled() || len(sl.items) == 0 {
		return false
	}
	if matchKeys(ev, keysDeleteCharBackward) {
		if sl.filterQuery == "" {
			return false
		}
		sl.setFilter(dropLastGrapheme(sl.filterQuery))
		return true
	}
	t := PrintableText(ev)
	if t == "" {
		return false
	}
	if sl.filterQuery == "" && strings.TrimSpace(t) == "" {
		return false
	}
	sl.setFilter(sl.filterQuery + t)
	return true
}

func (sl *SettingsList) activateItem() {
	if sl.selected < 0 || sl.selected >= len(sl.filtered) {
		return
	}
	item := &sl.filtered[sl.selected]
	if item.Heading {
		return
	}
	// Keep items[] in sync for CurrentValue mutations.
	src := sl.findItem(item.ID)
	if src == nil {
		src = item
	}
	if src.Submenu != nil {
		sl.submenuID = src.ID
		sl.submenu = src.Submenu(src.CurrentValue, func(selected string, ok bool) {
			if ok {
				src.CurrentValue = selected
				// Mirror into filtered copy.
				if it := sl.findFiltered(src.ID); it != nil {
					it.CurrentValue = selected
				}
				if sl.onChange != nil {
					sl.onChange(src.ID, selected)
				}
			}
			sl.closeSubmenu()
		})
		sl.dirty = true
		return
	}
	if len(src.Values) > 0 {
		cur := indexOf(src.Values, src.CurrentValue)
		next := (cur + 1) % len(src.Values)
		newVal := src.Values[next]
		src.CurrentValue = newVal
		if it := sl.findFiltered(src.ID); it != nil {
			it.CurrentValue = newVal
		}
		if sl.onChange != nil {
			sl.onChange(src.ID, newVal)
		}
		sl.dirty = true
	}
}

func (sl *SettingsList) closeSubmenu() {
	component.DisposeOne(sl.submenu)
	sl.submenu = nil
	if sl.submenuID != "" {
		id := sl.submenuID
		sl.submenuID = ""
		idx := -1
		for i, it := range sl.filtered {
			if !it.Heading && it.ID == id {
				idx = i
				break
			}
		}
		if idx >= 0 {
			sl.selected = idx
		} else {
			sl.clampSelectedIndex()
		}
		sl.notifySelection()
	}
	sl.dirty = true
}

func (sl *SettingsList) findItem(id string) *SettingItem {
	for i := range sl.items {
		if sl.items[i].ID == id {
			return &sl.items[i]
		}
	}
	return nil
}

func (sl *SettingsList) findFiltered(id string) *SettingItem {
	for i := range sl.filtered {
		if sl.filtered[i].ID == id {
			return &sl.filtered[i]
		}
	}
	return nil
}

func indexOf(vals []string, v string) int {
	for i, x := range vals {
		if x == v {
			return i
		}
	}
	return -1
}
