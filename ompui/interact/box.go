package interact

import (
	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// BoxBorder draws an outline around a Box.
type BoxBorder struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
	Color       Style
}

// DefaultBoxBorder returns single-line box-drawing glyphs.
func DefaultBoxBorder() BoxBorder {
	return BoxBorder{
		TopLeft: "┌", TopRight: "┐",
		BottomLeft: "└", BottomRight: "┘",
		Horizontal: "─", Vertical: "│",
	}
}

// Box stacks children with padding, optional background, and optional border.
type Box struct {
	children  []component.Component
	paddingX  int
	paddingY  int
	bgFn      Style
	border    *BoxBorder
	ignoreTight bool
	focusTarget component.Component

	gen    component.Gen
	cached component.Frame
	cacheW int
	// memo of child generations + line refs
	childGens  []uint64
	childLines [][]string
	dirty      bool
}

// NewBox constructs a Box with the given padding.
func NewBox(paddingX, paddingY int, children ...component.Component) *Box {
	b := &Box{
		paddingX: maxInt(0, paddingX),
		paddingY: maxInt(0, paddingY),
		dirty:    true,
	}
	for _, c := range children {
		b.AddChild(c)
	}
	return b
}

// Children returns a snapshot of the child list.
func (b *Box) Children() []component.Component {
	out := make([]component.Component, len(b.children))
	copy(out, b.children)
	return out
}

// AddChild appends a child.
func (b *Box) AddChild(c component.Component) {
	if c == nil {
		return
	}
	if b.ignoreTight {
		if t, ok := c.(component.TightLayoutAware); ok {
			t.SetIgnoreTight(true)
		}
	}
	b.children = append(b.children, c)
	b.dirty = true
}

// RemoveChild removes the first identity match.
func (b *Box) RemoveChild(c component.Component) bool {
	for i, ch := range b.children {
		if ch == c {
			b.children = append(b.children[:i], b.children[i+1:]...)
			b.dirty = true
			return true
		}
	}
	return false
}

// Clear removes all children without disposing them.
func (b *Box) Clear() {
	b.children = nil
	b.dirty = true
}

// SetPaddingX sets horizontal padding.
func (b *Box) SetPaddingX(n int) {
	n = maxInt(0, n)
	if b.paddingX == n {
		return
	}
	b.paddingX = n
	b.dirty = true
}

// SetPaddingY sets vertical padding.
func (b *Box) SetPaddingY(n int) {
	n = maxInt(0, n)
	if b.paddingY == n {
		return
	}
	b.paddingY = n
	b.dirty = true
}

// SetBgFn sets the background painter (nil clears).
func (b *Box) SetBgFn(fn Style) {
	b.bgFn = fn
	b.dirty = true
}

// SetBorder sets or clears the border.
func (b *Box) SetBorder(border *BoxBorder) {
	b.border = border
	b.dirty = true
}

// SetIgnoreTight implements component.TightLayoutAware.
func (b *Box) SetIgnoreTight(ignore bool) {
	b.ignoreTight = ignore
	for _, c := range b.children {
		if t, ok := c.(component.TightLayoutAware); ok {
			t.SetIgnoreTight(ignore)
		}
	}
	b.dirty = true
}

// IgnoreTight reports the flag.
func (b *Box) IgnoreTight() bool { return b.ignoreTight }

// SetFocusTarget selects the child that receives input.
func (b *Box) SetFocusTarget(c component.Component) {
	b.focusTarget = c
}

// FocusTarget returns the input routing target.
func (b *Box) FocusTarget() component.Component { return b.focusTarget }

// HandleInput routes to the focus target.
func (b *Box) HandleInput(ev event.Event) {
	component.RouteInput(b.focusTarget, ev)
}

// OwnsOverlayFocusTarget implements component.OverlayFocusOwner.
func (b *Box) OwnsOverlayFocusTarget(c component.Component) bool {
	if c == nil {
		return false
	}
	if c == b {
		return true
	}
	for _, ch := range b.children {
		if component.IsOverlayFocusTarget(ch, c) || ch == c {
			return true
		}
		if component.SubtreeContains(ch, c) {
			return true
		}
	}
	return false
}

// Invalidate implements component.Invalidator.
func (b *Box) Invalidate() {
	b.dirty = true
	b.cached = component.Frame{}
	b.cacheW = -1
	for _, c := range b.children {
		component.InvalidateOne(c)
	}
}

// Dispose implements component.Disposable.
func (b *Box) Dispose() {
	for _, c := range b.children {
		component.DisposeOne(c)
	}
}

// Render implements component.Component.
func (b *Box) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	padX := effectivePaddingX(b.paddingX, b.ignoreTight)
	var border *BoxBorder
	if b.border != nil && width-2 >= padX*2+1 {
		border = b.border
	}
	innerWidth := width
	if border != nil {
		innerWidth = width - 2
	}
	contentWidth := maxInt(1, innerWidth-padX*2)

	count := len(b.children)
	childLines := make([][]string, count)
	childGens := make([]uint64, count)
	var cursor *component.Cursor
	cursorOffset := 0
	contentRows := 0
	// Top padding contributes to cursor offset when present.
	topPad := b.paddingY
	if contentRows == 0 && count == 0 {
		// empty
	}
	for i, ch := range b.children {
		fr := ch.Render(contentWidth)
		childLines[i] = fr.Lines
		childGens[i] = fr.Generation
		contentRows += len(fr.Lines)
		if fr.Cursor != nil && cursor == nil {
			c := *fr.Cursor
			c.Row += topPad + cursorOffset
			c.Column += padX
			if border != nil {
				c.Row++
				c.Column++
			}
			cursor = &c
		}
		cursorOffset += len(fr.Lines)
	}

	unchanged := !b.dirty && b.cacheW == width &&
		len(b.childGens) == count && len(b.childLines) == count
	if unchanged {
		for i := 0; i < count; i++ {
			if b.childGens[i] != childGens[i] || !sameRef(b.childLines[i], childLines[i]) {
				unchanged = false
				break
			}
		}
	}
	if unchanged && b.cached.Lines != nil {
		// Still refresh cursor on focus changes inside children.
		if cursorEqual(b.cached.Cursor, cursor) {
			return b.cached
		}
	}

	var result []string
	if contentRows > 0 || b.paddingY > 0 || border != nil {
		leftPad := padSpaces(padX)
		interior := make([]string, 0, contentRows+b.paddingY*2)
		for i := 0; i < b.paddingY; i++ {
			interior = append(interior, applyBackground("", innerWidth, b.bgFn))
		}
		for _, lines := range childLines {
			for _, ln := range lines {
				interior = append(interior, applyBackground(leftPad+ln, innerWidth, b.bgFn))
			}
		}
		for i := 0; i < b.paddingY; i++ {
			interior = append(interior, applyBackground("", innerWidth, b.bgFn))
		}

		if border != nil {
			paint := border.Color
			if paint == nil {
				paint = func(s string) string { return s }
			}
			h := border.Horizontal
			if h == "" {
				h = "─"
			}
			v := border.Vertical
			if v == "" {
				v = "│"
			}
			rule := ""
			if innerWidth > 0 {
				// Repeat first cell of horizontal glyph.
				hg := firstCellGlyph(h, "─")
				for i := 0; i < innerWidth; i++ {
					rule += hg
				}
			}
			tl := firstNonEmpty(border.TopLeft, "┌")
			tr := firstNonEmpty(border.TopRight, "┐")
			bl := firstNonEmpty(border.BottomLeft, "└")
			br := firstNonEmpty(border.BottomRight, "┘")
			side := paint(firstCellGlyph(v, "│"))
			result = make([]string, 0, len(interior)+2)
			result = append(result, paint(tl+rule+tr))
			for _, row := range interior {
				// Ensure interior fills innerWidth for border alignment.
				row = padToWidth(row, innerWidth)
				// If bg already padded, VisibleWidth should match.
				if ansitext.VisibleWidth(row) > innerWidth {
					row = ansitext.TruncateToWidth(row, innerWidth, "")
					row = padToWidth(row, innerWidth)
				}
				result = append(result, side+row+side)
			}
			result = append(result, paint(bl+rule+br))
		} else {
			result = interior
		}
	}
	if result == nil {
		result = []string{}
	}

	changed := b.dirty || b.cacheW != width || !sameLines(b.cached.Lines, result) || !cursorEqual(b.cached.Cursor, cursor)
	gen := b.gen.Touch(changed)
	b.dirty = false
	b.cacheW = width
	b.childGens = childGens
	b.childLines = childLines
	if !changed && b.cached.Lines != nil {
		return b.cached
	}
	fr := component.NewFrame(result, gen).WithCursor(cursor)
	b.cached = fr
	return fr
}

func sameRef(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	// Slice header identity: compare via first-element address trick is unsafe;
	// use generation tracking as primary. Here compare lengths and pointer of
	// underlying array via full equality fallback.
	return sameLines(a, b) && len(a) == len(b)
}

func firstNonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
