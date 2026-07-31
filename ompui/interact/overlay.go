package interact

import (
	"strconv"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/event"
)

// OverlayAnchor positions an overlay within the available terminal area.
type OverlayAnchor string

const (
	AnchorCenter       OverlayAnchor = "center"
	AnchorTopLeft      OverlayAnchor = "top-left"
	AnchorTopRight     OverlayAnchor = "top-right"
	AnchorBottomLeft   OverlayAnchor = "bottom-left"
	AnchorBottomRight  OverlayAnchor = "bottom-right"
	AnchorTopCenter    OverlayAnchor = "top-center"
	AnchorBottomCenter OverlayAnchor = "bottom-center"
	AnchorLeftCenter   OverlayAnchor = "left-center"
	AnchorRightCenter  OverlayAnchor = "right-center"
)

// OverlayMargin is per-side margin from terminal edges.
type OverlayMargin struct {
	Top, Right, Bottom, Left int
}

// SizeValue is an absolute column/row count or a percentage of the terminal
// dimension. Percent form is "50%" (only that syntax).
type SizeValue struct {
	Abs  int
	Pct  float64 // 0..100; used when IsPct
	IsPct bool
}

// SizeAbs returns an absolute size.
func SizeAbs(n int) SizeValue { return SizeValue{Abs: n} }

// SizePct returns a percentage size (e.g. 50 for "50%").
func SizePct(pct float64) SizeValue { return SizeValue{Pct: pct, IsPct: true} }

// ParseSizeValue parses a number or "N%" string. ok false on empty/invalid.
func ParseSizeValue(s string) (SizeValue, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return SizeValue{}, false
	}
	if strings.HasSuffix(s, "%") {
		p, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return SizeValue{}, false
		}
		return SizePct(p), true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return SizeValue{}, false
	}
	return SizeAbs(n), true
}

func (v SizeValue) resolve(reference int) (int, bool) {
	if v.IsPct {
		return int(float64(reference) * v.Pct / 100), true
	}
	if v.Abs == 0 && !v.IsPct {
		// Zero absolute may be intentional; treat as set only if Abs explicitly
		// used via SizeAbs(0). Callers use pointer/optional on Options fields.
		return v.Abs, true
	}
	return v.Abs, true
}

// OverlayOptions configures positioning and sizing for one stack entry.
type OverlayOptions struct {
	Width    *SizeValue
	MinWidth int
	MaxHeight *SizeValue

	Anchor  OverlayAnchor
	OffsetX int
	OffsetY int

	Row *SizeValue
	Col *SizeValue

	// Margin: when UniformMargin >= 0 and Margin is nil, uniform applies.
	Margin       *OverlayMargin
	UniformMargin int // -1 = unset; >=0 applies to all sides when Margin is nil

	// Visible gates rendering each frame. nil → always visible (unless hidden).
	Visible func(termWidth, termHeight int) bool

	// Fullscreen borrows the alternate screen while this is the topmost visible overlay.
	Fullscreen bool
}

// OverlayLayout is the resolved geometry for painting one overlay.
type OverlayLayout struct {
	Width     int
	Row       int
	Col       int
	MaxHeight int
	Height    int // effective height after clamp
}

// OverlayFrame is one composited overlay contribution for the renderer.
type OverlayFrame struct {
	Component component.Component
	Lines     []string
	Layout    OverlayLayout
	Cursor    *component.Cursor // frame-local; renderer translates by Layout.Row/Col
	Fullscreen bool
	Options   OverlayOptions
}

// OverlayHandle controls one stack entry.
type OverlayHandle struct {
	stack *OverlayStack
	entry *overlayEntry
}

// Hide permanently removes the overlay.
func (h OverlayHandle) Hide() {
	if h.stack == nil || h.entry == nil {
		return
	}
	h.stack.hideEntry(h.entry)
}

// SetHidden temporarily hides or shows the overlay.
func (h OverlayHandle) SetHidden(hidden bool) {
	if h.stack == nil || h.entry == nil {
		return
	}
	h.stack.setHidden(h.entry, hidden)
}

// IsHidden reports temporary hide state.
func (h OverlayHandle) IsHidden() bool {
	if h.entry == nil {
		return true
	}
	return h.entry.hidden
}

// Component returns the overlay root.
func (h OverlayHandle) Component() component.Component {
	if h.entry == nil {
		return nil
	}
	return h.entry.component
}

type overlayEntry struct {
	component component.Component
	options   OverlayOptions
	preFocus  component.Component
	hidden    bool
}

// OverlayStack owns modal z-order, geometry, and focus transfer/restoration.
// It does not paint; the renderer consumes Frames()/Composite.
type OverlayStack struct {
	entries []*overlayEntry
	focused component.Component
	// BaseFocus is the non-overlay focus restored when the stack empties.
	BaseFocus component.Component
	// UseTerminalCursor is synced onto focus targets.
	UseTerminalCursor bool
}

// NewOverlayStack constructs an empty stack.
func NewOverlayStack() *OverlayStack {
	return &OverlayStack{}
}

// Len returns the number of stack entries (including hidden).
func (s *OverlayStack) Len() int { return len(s.entries) }

// HasVisible reports any currently visible overlay.
func (s *OverlayStack) HasVisible(termWidth, termHeight int) bool {
	return s.topVisible(termWidth, termHeight) != nil
}

// Focused returns the current focus target (overlay or base).
func (s *OverlayStack) Focused() component.Component { return s.focused }

// SetBaseFocus records the non-overlay focus and applies it when no overlay is focused.
func (s *OverlayStack) SetBaseFocus(c component.Component) {
	s.BaseFocus = c
	if s.topVisible(0, 0) == nil && len(s.entries) == 0 {
		s.applyFocus(c)
	}
}

// SetFocus requests focus. When a visible overlay is up, focus is clamped to
// that overlay or one of its owned targets.
func (s *OverlayStack) SetFocus(c component.Component, termWidth, termHeight int) {
	top := s.topVisible(termWidth, termHeight)
	if top != nil && !component.IsOverlayFocusTarget(top.component, c) {
		cur := s.focused
		if component.IsOverlayFocusTarget(top.component, cur) {
			c = cur
		} else {
			c = top.component
		}
	}
	s.applyFocus(c)
}

// Show pushes an overlay, focuses it when visible, and returns a handle.
func (s *OverlayStack) Show(c component.Component, opt OverlayOptions) OverlayHandle {
	if c == nil {
		return OverlayHandle{stack: s}
	}
	if t, ok := c.(component.TightLayoutAware); ok {
		t.SetIgnoreTight(true)
	}
	e := &overlayEntry{
		component: c,
		options:   opt,
		preFocus:  s.focused,
		hidden:    false,
	}
	if e.preFocus == nil {
		e.preFocus = s.BaseFocus
	}
	s.entries = append(s.entries, e)
	// Visibility needs term size; without it assume visible.
	s.applyFocus(c)
	return OverlayHandle{stack: s, entry: e}
}

// HideTop pops the topmost entry and restores focus.
func (s *OverlayStack) HideTop(termWidth, termHeight int) {
	n := len(s.entries)
	if n == 0 {
		return
	}
	e := s.entries[n-1]
	s.entries = s.entries[:n-1]
	top := s.topVisible(termWidth, termHeight)
	if top != nil {
		s.applyFocus(top.component)
	} else {
		s.applyFocus(e.preFocus)
	}
}

// HandleInput routes to the focused component.
func (s *OverlayStack) HandleInput(ev event.Event) {
	component.RouteInput(s.focused, ev)
}

// ReconcileFocus ensures focus stays on a visible overlay after resize.
func (s *OverlayStack) ReconcileFocus(termWidth, termHeight int) {
	// If focused overlay became invisible, redirect.
	for _, e := range s.entries {
		if e.component != s.focused && !component.IsOverlayFocusTarget(e.component, s.focused) {
			continue
		}
		if !s.isVisible(e, termWidth, termHeight) {
			top := s.topVisible(termWidth, termHeight)
			if top != nil {
				s.applyFocus(top.component)
			} else {
				s.applyFocus(e.preFocus)
			}
			return
		}
	}
	// Clamp SetFocus rules.
	top := s.topVisible(termWidth, termHeight)
	if top != nil && !component.IsOverlayFocusTarget(top.component, s.focused) {
		s.applyFocus(top.component)
	}
}

// WantsAltScreen reports whether the topmost visible overlay requests fullscreen.
func (s *OverlayStack) WantsAltScreen(termWidth, termHeight int) bool {
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if !s.isVisible(e, termWidth, termHeight) {
			continue
		}
		return e.options.Fullscreen
	}
	return false
}

// Frames renders all visible overlays bottom→top for the renderer.
// Overlay rows are separate from the transcript; the renderer must composite
// them into the window only and freeze scrollback commits while any exist.
func (s *OverlayStack) Frames(termWidth, termHeight int) []OverlayFrame {
	if termWidth < 1 {
		termWidth = 1
	}
	if termHeight < 1 {
		termHeight = 1
	}
	var out []OverlayFrame
	for _, e := range s.entries {
		if !s.isVisible(e, termWidth, termHeight) {
			continue
		}
		// Width/maxHeight first (height-independent).
		probe := ResolveOverlayLayout(e.options, 0, termWidth, termHeight)
		fr := e.component.Render(probe.Width)
		lines := fr.Lines
		if len(lines) > probe.MaxHeight {
			anchor := e.options.Anchor
			if anchor == "" {
				anchor = AnchorCenter
			}
			switch anchor {
			case AnchorBottomLeft, AnchorBottomCenter, AnchorBottomRight:
				lines = lines[len(lines)-probe.MaxHeight:]
			default:
				lines = lines[:probe.MaxHeight]
			}
		}
		// Clamp each line to width.
		clipped := make([]string, len(lines))
		for i, ln := range lines {
			if ansitext.VisibleWidth(ln) > probe.Width {
				clipped[i] = ansitext.SliceByColumn(ln, 0, probe.Width)
			} else {
				clipped[i] = ln
			}
		}
		layout := ResolveOverlayLayout(e.options, len(clipped), termWidth, termHeight)
		layout.Height = len(clipped)
		out = append(out, OverlayFrame{
			Component:  e.component,
			Lines:      clipped,
			Layout:     layout,
			Cursor:     fr.Cursor,
			Fullscreen: e.options.Fullscreen,
			Options:    e.options,
		})
	}
	return out
}

// Composite overlays into a copy of window (screen rows). Later stack entries
// paint on top. Does not mutate window. termWidth pads/clips each row.
func Composite(window []string, frames []OverlayFrame, termWidth int) []string {
	if len(frames) == 0 {
		return window
	}
	result := make([]string, len(window))
	copy(result, window)
	for _, f := range frames {
		for i, ln := range f.Lines {
			idx := f.Layout.Row + i
			if idx < 0 || idx >= len(result) {
				continue
			}
			result[idx] = compositeLineAt(result[idx], ln, f.Layout.Col, f.Layout.Width, termWidth)
		}
	}
	return result
}

// ResolveOverlayLayout computes width/row/col/maxHeight for an overlay.
func ResolveOverlayLayout(opt OverlayOptions, overlayHeight, termWidth, termHeight int) OverlayLayout {
	if termWidth < 1 {
		termWidth = 1
	}
	if termHeight < 1 {
		termHeight = 1
	}
	mTop, mRight, mBottom, mLeft := resolveMargin(opt)
	availW := maxInt(1, termWidth-mLeft-mRight)
	availH := maxInt(1, termHeight-mTop-mBottom)

	width := minInt(80, availW)
	if opt.Width != nil {
		if w, ok := opt.Width.resolve(termWidth); ok {
			width = w
		}
	}
	if opt.MinWidth > 0 {
		width = maxInt(width, opt.MinWidth)
	}
	width = clampInt(width, 1, availW)

	maxHeight := availH
	if opt.MaxHeight != nil {
		if h, ok := opt.MaxHeight.resolve(termHeight); ok {
			maxHeight = h
		}
	}
	maxHeight = clampInt(maxHeight, 1, availH)

	effH := minInt(overlayHeight, maxHeight)

	var row, col int
	if opt.Row != nil {
		if opt.Row.IsPct {
			maxRow := maxInt(0, availH-effH)
			pct := opt.Row.Pct / 100
			row = mTop + int(float64(maxRow)*pct)
		} else {
			row = opt.Row.Abs
		}
	} else {
		anchor := opt.Anchor
		if anchor == "" {
			anchor = AnchorCenter
		}
		row = resolveAnchorRow(anchor, effH, availH, mTop)
	}

	if opt.Col != nil {
		if opt.Col.IsPct {
			maxCol := maxInt(0, availW-width)
			pct := opt.Col.Pct / 100
			col = mLeft + int(float64(maxCol)*pct)
		} else {
			col = opt.Col.Abs
		}
	} else {
		anchor := opt.Anchor
		if anchor == "" {
			anchor = AnchorCenter
		}
		col = resolveAnchorCol(anchor, width, availW, mLeft)
	}

	row += opt.OffsetY
	col += opt.OffsetX
	row = clampInt(row, mTop, termHeight-mBottom-effH)
	col = clampInt(col, mLeft, termWidth-mRight-width)

	return OverlayLayout{Width: width, Row: row, Col: col, MaxHeight: maxHeight, Height: effH}
}

func resolveMargin(opt OverlayOptions) (top, right, bottom, left int) {
	if opt.Margin != nil {
		return maxInt(0, opt.Margin.Top), maxInt(0, opt.Margin.Right),
			maxInt(0, opt.Margin.Bottom), maxInt(0, opt.Margin.Left)
	}
	if opt.UniformMargin >= 0 {
		m := opt.UniformMargin
		return m, m, m, m
	}
	return 0, 0, 0, 0
}

func resolveAnchorRow(anchor OverlayAnchor, height, availHeight, marginTop int) int {
	switch anchor {
	case AnchorTopLeft, AnchorTopCenter, AnchorTopRight:
		return marginTop
	case AnchorBottomLeft, AnchorBottomCenter, AnchorBottomRight:
		return marginTop + availHeight - height
	default:
		return marginTop + (availHeight-height)/2
	}
}

func resolveAnchorCol(anchor OverlayAnchor, width, availWidth, marginLeft int) int {
	switch anchor {
	case AnchorTopLeft, AnchorLeftCenter, AnchorBottomLeft:
		return marginLeft
	case AnchorTopRight, AnchorRightCenter, AnchorBottomRight:
		return marginLeft + availWidth - width
	default:
		return marginLeft + (availWidth-width)/2
	}
}

func compositeLineAt(base, overlay string, startCol, overlayWidth, totalWidth int) string {
	if startCol < 0 {
		startCol = 0
	}
	// Simple ANSI-aware splice: before | overlay | after, padded to totalWidth.
	before := ansitext.SliceByColumn(base, 0, startCol)
	afterStart := startCol + overlayWidth
	after := ""
	if afterStart < totalWidth {
		// Take remainder of base from afterStart.
		baseW := ansitext.VisibleWidth(base)
		if afterStart < baseW {
			after = ansitext.SliceByColumn(base, afterStart, baseW-afterStart)
		}
	}
	ov := overlay
	if ansitext.VisibleWidth(ov) > overlayWidth {
		ov = ansitext.SliceByColumn(ov, 0, overlayWidth)
	}
	beforePad := maxInt(0, startCol-ansitext.VisibleWidth(before))
	ovPad := maxInt(0, overlayWidth-ansitext.VisibleWidth(ov))
	actualBefore := maxInt(startCol, ansitext.VisibleWidth(before))
	actualOv := maxInt(overlayWidth, ansitext.VisibleWidth(ov))
	afterTarget := maxInt(0, totalWidth-actualBefore-actualOv)
	afterPad := maxInt(0, afterTarget-ansitext.VisibleWidth(after))

	return before + padSpaces(beforePad) + ov + padSpaces(ovPad) + after + padSpaces(afterPad)
}

func (s *OverlayStack) topVisible(termWidth, termHeight int) *overlayEntry {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.isVisible(s.entries[i], termWidth, termHeight) {
			return s.entries[i]
		}
	}
	return nil
}

func (s *OverlayStack) isVisible(e *overlayEntry, termWidth, termHeight int) bool {
	if e == nil || e.hidden {
		return false
	}
	if e.options.Visible != nil {
		// When size unknown (0,0), treat as visible so Show can focus.
		if termWidth <= 0 || termHeight <= 0 {
			return true
		}
		return e.options.Visible(termWidth, termHeight)
	}
	return true
}

func (s *OverlayStack) hideEntry(e *overlayEntry) {
	for i, x := range s.entries {
		if x == e {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			if component.IsOverlayFocusTarget(e.component, s.focused) {
				top := s.topVisible(0, 0)
				if top != nil {
					s.applyFocus(top.component)
				} else {
					s.applyFocus(e.preFocus)
				}
			}
			return
		}
	}
}

func (s *OverlayStack) setHidden(e *overlayEntry, hidden bool) {
	if e.hidden == hidden {
		return
	}
	e.hidden = hidden
	if hidden {
		if component.IsOverlayFocusTarget(e.component, s.focused) {
			top := s.topVisible(0, 0)
			if top != nil {
				s.applyFocus(top.component)
			} else {
				s.applyFocus(e.preFocus)
			}
		}
	} else if s.isVisible(e, 0, 0) {
		s.applyFocus(e.component)
	}
}

func (s *OverlayStack) applyFocus(next component.Component) {
	component.ApplyFocus(s.focused, next, s.UseTerminalCursor)
	s.focused = next
}

// InvalidateAll invalidates every overlay component.
func (s *OverlayStack) InvalidateAll() {
	for _, e := range s.entries {
		component.InvalidateOne(e.component)
	}
}

// DisposeAll disposes every overlay component and clears the stack.
func (s *OverlayStack) DisposeAll() {
	for _, e := range s.entries {
		component.DisposeOne(e.component)
	}
	s.entries = nil
	s.applyFocus(s.BaseFocus)
}
