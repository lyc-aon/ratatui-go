package interact

import (
	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// Tab is one entry in a TabBar.
type Tab struct {
	ID    string
	Label string
	// Short is the compact form used when the bar must shrink.
	Short string
	// Muted skips keyboard navigation and uses muted styling.
	Muted bool
}

// TabBarTheme styles the tab bar.
type TabBarTheme struct {
	Label       Style
	ActiveTab   Style
	InactiveTab Style
	Hint        Style
	MutedTab    Style // falls back to InactiveTab
	HoverTab    Style // falls back to InactiveTab
}

// TabBar is a horizontal tab strip: "Label:  Tab1   Tab2  (tab to cycle)".
type TabBar struct {
	tabs        []Tab
	activeIndex int
	theme       TabBarTheme
	label       string
	hoverTabID  string
	// ShowHint renders "(tab to cycle)". Default true.
	ShowHint bool

	// OnTabChange fires when the active tab changes via navigation/select.
	OnTabChange func(tab Tab, index int)

	hitZones []tabHit
	gen      component.Gen
	cached   component.Frame
	cacheW   int
	dirty    bool
}

type tabHit struct {
	line, start, end, index int
}

// NewTabBar constructs a tab bar.
func NewTabBar(label string, tabs []Tab, theme TabBarTheme, initialIndex int) *TabBar {
	tb := &TabBar{
		label:    label,
		tabs:     append([]Tab(nil), tabs...),
		theme:    theme,
		ShowHint: true,
		dirty:    true,
	}
	if len(tb.tabs) > 0 {
		tb.activeIndex = clampInt(initialIndex, 0, len(tb.tabs)-1)
	}
	return tb
}

// ActiveTab returns the current tab (zero value when empty).
func (tb *TabBar) ActiveTab() Tab {
	if len(tb.tabs) == 0 {
		return Tab{}
	}
	return tb.tabs[tb.activeIndex]
}

// ActiveIndex returns the active tab index.
func (tb *TabBar) ActiveIndex() int { return tb.activeIndex }

// SetActiveIndex clamps and activates by index, firing OnTabChange on change.
func (tb *TabBar) SetActiveIndex(index int) {
	if len(tb.tabs) == 0 {
		return
	}
	next := clampInt(index, 0, len(tb.tabs)-1)
	if next == tb.activeIndex {
		return
	}
	tb.activeIndex = next
	tb.dirty = true
	if tb.OnTabChange != nil {
		tb.OnTabChange(tb.tabs[tb.activeIndex], tb.activeIndex)
	}
}

// SetTabs replaces the tab set without firing OnTabChange. Active tab is
// preserved by id when it survives (or forced via activeID).
func (tb *TabBar) SetTabs(tabs []Tab, activeID string) {
	target := activeID
	if target == "" && len(tb.tabs) > 0 {
		target = tb.tabs[tb.activeIndex].ID
	}
	tb.tabs = append([]Tab(nil), tabs...)
	idx := -1
	for i, t := range tb.tabs {
		if t.ID == target {
			idx = i
			break
		}
	}
	if idx >= 0 {
		tb.activeIndex = idx
	} else if len(tb.tabs) == 0 {
		tb.activeIndex = 0
	} else {
		tb.activeIndex = clampInt(tb.activeIndex, 0, len(tb.tabs)-1)
	}
	tb.dirty = true
}

// SetActiveByID sets the active tab without firing OnTabChange.
func (tb *TabBar) SetActiveByID(id string) bool {
	for i, t := range tb.tabs {
		if t.ID == id {
			tb.activeIndex = i
			tb.dirty = true
			return true
		}
	}
	return false
}

// SelectTab activates by id (skips muted), firing OnTabChange on change.
func (tb *TabBar) SelectTab(id string) bool {
	for i, t := range tb.tabs {
		if t.ID == id {
			if t.Muted {
				return false
			}
			tb.SetActiveIndex(i)
			return true
		}
	}
	return false
}

// NextTab moves to the next non-muted tab (wraps).
func (tb *TabBar) NextTab() { tb.stepTab(1) }

// PrevTab moves to the previous non-muted tab (wraps).
func (tb *TabBar) PrevTab() { tb.stepTab(-1) }

func (tb *TabBar) stepTab(delta int) {
	n := len(tb.tabs)
	if n == 0 {
		return
	}
	for step := 1; step <= n; step++ {
		idx := ((tb.activeIndex + delta*step) % n + n) % n
		if !tb.tabs[idx].Muted {
			tb.SetActiveIndex(idx)
			return
		}
	}
}

// HandleInput implements component.InputHandler.
// Tab/Right → next; Shift+Tab/Left → prev. Returns via side effect only.
func (tb *TabBar) HandleInput(ev event.Event) {
	_ = tb.HandleNavKey(ev)
}

// HandleNavKey returns true when the key was consumed.
func (tb *TabBar) HandleNavKey(ev event.Event) bool {
	if matchKeys(ev, keysTab) || matchKeys(ev, keysRight) {
		tb.NextTab()
		return true
	}
	if matchKeys(ev, keysShiftTab) || matchKeys(ev, keysLeft) {
		tb.PrevTab()
		return true
	}
	return false
}

// TabAt resolves a pointer position against the last render.
func (tb *TabBar) TabAt(line, col int) (Tab, bool) {
	for _, z := range tb.hitZones {
		if z.line == line && col >= z.start && col < z.end {
			return tb.tabs[z.index], true
		}
	}
	return Tab{}, false
}

// SetHoverTab highlights the tab under the pointer ("" clears).
func (tb *TabBar) SetHoverTab(id string) {
	if tb.hoverTabID == id {
		return
	}
	tb.hoverTabID = id
	tb.dirty = true
}

// Invalidate implements component.Invalidator.
func (tb *TabBar) Invalidate() {
	tb.dirty = true
	tb.cached = component.Frame{}
	tb.cacheW = -1
}

// Dispose implements component.Disposable.
func (tb *TabBar) Dispose() {
	tb.OnTabChange = nil
}

// Render implements component.Component.
func (tb *TabBar) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	maxWidth := width

	type chunk struct {
		text     string
		tabIndex int // -1 if not a tab button
	}

	build := func(labels []string) []chunk {
		var chunks []chunk
		if tb.label != "" {
			chunks = append(chunks, chunk{text: applyStyle(tb.theme.Label, tb.label+":"), tabIndex: -1})
			chunks = append(chunks, chunk{text: "  ", tabIndex: -1})
		}
		for i, tab := range tb.tabs {
			hovered := tab.ID == tb.hoverTabID && !tab.Muted && i != tb.activeIndex
			var style Style
			switch {
			case tab.Muted:
				style = tb.theme.MutedTab
				if style == nil {
					style = tb.theme.InactiveTab
				}
			case i == tb.activeIndex:
				style = tb.theme.ActiveTab
			case hovered:
				style = tb.theme.HoverTab
				if style == nil {
					style = tb.theme.InactiveTab
				}
			default:
				style = tb.theme.InactiveTab
			}
			lab := labels[i]
			chunks = append(chunks, chunk{text: applyStyle(style, " "+lab+" "), tabIndex: i})
			if i < len(tb.tabs)-1 {
				chunks = append(chunks, chunk{text: "  ", tabIndex: -1})
			}
		}
		if tb.ShowHint {
			chunks = append(chunks, chunk{text: "  ", tabIndex: -1})
			chunks = append(chunks, chunk{text: applyStyle(tb.theme.Hint, "(tab to cycle)"), tabIndex: -1})
		}
		return chunks
	}

	totalW := func(chunks []chunk) int {
		sum := 0
		for _, c := range chunks {
			sum += ansitext.VisibleWidth(c.text)
		}
		return sum
	}

	labels := make([]string, len(tb.tabs))
	for i, t := range tb.tabs {
		labels[i] = t.Label
	}
	chunks := build(labels)
	if totalW(chunks) > maxWidth {
		// Collapse farthest-from-active tabs that have Short forms.
		type order struct{ idx, dist int }
		var collapse []order
		for i, t := range tb.tabs {
			if i == tb.activeIndex || t.Short == "" {
				continue
			}
			d := i - tb.activeIndex
			if d < 0 {
				d = -d
			}
			collapse = append(collapse, order{i, d})
		}
		// Sort by distance descending (farthest first).
		for i := 0; i < len(collapse); i++ {
			for j := i + 1; j < len(collapse); j++ {
				if collapse[j].dist > collapse[i].dist {
					collapse[i], collapse[j] = collapse[j], collapse[i]
				}
			}
		}
		for _, o := range collapse {
			labels[o.idx] = tb.tabs[o.idx].Short
			chunks = build(labels)
			if totalW(chunks) <= maxWidth {
				break
			}
		}
	}

	tb.hitZones = tb.hitZones[:0]
	var lines []string
	currentLine := ""
	currentWidth := 0

	for _, ch := range chunks {
		cw := ansitext.VisibleWidth(ch.text)
		if cw <= 0 {
			continue
		}
		if cw > maxWidth {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
				currentWidth = 0
			}
			if ch.tabIndex >= 0 {
				tb.hitZones = append(tb.hitZones, tabHit{line: len(lines), start: 0, end: maxWidth, index: ch.tabIndex})
			}
			lines = append(lines, ansitext.TruncateToWidth(ch.text, maxWidth, ""))
			continue
		}
		if currentWidth > 0 && currentWidth+cw > maxWidth {
			lines = append(lines, currentLine)
			currentLine = ""
			currentWidth = 0
		}
		if ch.tabIndex >= 0 {
			tb.hitZones = append(tb.hitZones, tabHit{
				line: len(lines), start: currentWidth, end: currentWidth + cw, index: ch.tabIndex,
			})
		}
		currentLine += ch.text
		currentWidth += cw
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}

	changed := tb.dirty || tb.cacheW != width || !sameLines(tb.cached.Lines, lines)
	gen := tb.gen.Touch(changed)
	tb.dirty = false
	tb.cacheW = width
	if !changed && tb.cached.Lines != nil {
		return tb.cached
	}
	tb.cached = component.NewFrame(lines, gen)
	return tb.cached
}
