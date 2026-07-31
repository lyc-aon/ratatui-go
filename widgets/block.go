package widgets

import (
	"github.com/lyc-aon/ratatui-go/buffer"
	"github.com/lyc-aon/ratatui-go/layout"
	"github.com/lyc-aon/ratatui-go/style"
	"github.com/lyc-aon/ratatui-go/symbols"
	"github.com/lyc-aon/ratatui-go/text"
)

// TitlePosition is where a Block title is drawn.
type TitlePosition int

const (
	// TitlePositionTop places the title on the top edge (default).
	TitlePositionTop TitlePosition = iota
	// TitlePositionBottom places the title on the bottom edge.
	TitlePositionBottom
)

// String returns a stable name for the title position.
func (p TitlePosition) String() string {
	switch p {
	case TitlePositionBottom:
		return "Bottom"
	default:
		return "Top"
	}
}

// titleEntry is one title with optional explicit position.
// nil position means "use Block.titlesPosition default".
type titleEntry struct {
	pos  *TitlePosition
	line text.Line
}

// Block draws borders, titles, padding, and an optional shadow around content.
//
// Value builder: every With*/setter returns a modified copy. Render never panics
// on zero-size or clipped areas.
type Block struct {
	titles          []titleEntry
	titlesStyle     style.Style
	titlesAlignment layout.Alignment
	titlesPosition  TitlePosition
	borders         Borders
	borderStyle     style.Style
	borderSet       symbols.BorderSet
	style           style.Style
	padding         Padding
	mergeBorders    symbols.MergeStrategy
	shadow          *Shadow
}

// NewBlock creates a block with no borders or padding.
func NewBlock() Block {
	return Block{
		titlesAlignment: layout.AlignLeft,
		titlesPosition:  TitlePositionTop,
		borders:         BorderNone,
		borderSet:       BorderTypePlain.ToBorderSet(),
		padding:         PaddingZero,
		mergeBorders:    symbols.MergeStrategyReplace,
	}
}

// Bordered creates a block with all borders enabled.
func Bordered() Block {
	b := NewBlock()
	b.borders = BorderAll
	return b
}

// Title adds a title using the block's default TitlePosition.
// Call multiple times for multiple titles.
func (b Block) Title(title text.Line) Block {
	b.titles = append(copyTitles(b.titles), titleEntry{line: copyLine(title)})
	return b
}

// TitleTop adds a title fixed to the top edge.
func (b Block) TitleTop(title text.Line) Block {
	pos := TitlePositionTop
	b.titles = append(copyTitles(b.titles), titleEntry{pos: &pos, line: copyLine(title)})
	return b
}

// TitleBottom adds a title fixed to the bottom edge.
func (b Block) TitleBottom(title text.Line) Block {
	pos := TitlePositionBottom
	b.titles = append(copyTitles(b.titles), titleEntry{pos: &pos, line: copyLine(title)})
	return b
}

// TitleStyle sets the style patched onto every title area before the line draws.
func (b Block) TitleStyle(st style.Style) Block {
	b.titlesStyle = st
	return b
}

// TitleAlignment sets the default alignment for titles that do not set one.
func (b Block) TitleAlignment(a layout.Alignment) Block {
	b.titlesAlignment = a
	return b
}

// TitlePosition sets the default position for titles added via Title.
func (b Block) TitlePosition(p TitlePosition) Block {
	b.titlesPosition = p
	return b
}

// BorderStyle sets the style applied to border cells.
func (b Block) BorderStyle(st style.Style) Block {
	b.borderStyle = st
	return b
}

// Style sets the base style applied to the whole block area before borders.
func (b Block) Style(st style.Style) Block {
	b.style = st
	return b
}

// WithBorders selects which sides draw a border.
func (b Block) WithBorders(borders Borders) Block {
	b.borders = borders
	return b
}

// BorderType sets the border symbols from a preset BorderType.
// Overwrites any custom BorderSet.
func (b Block) BorderType(t BorderType) Block {
	b.borderSet = t.ToBorderSet()
	return b
}

// BorderSet sets a custom border symbol set.
// Overwrites any BorderType selection.
func (b Block) BorderSet(set symbols.BorderSet) Block {
	b.borderSet = set
	return b
}

// WithPadding sets the inner padding inside the borders.
func (b Block) WithPadding(p Padding) Block {
	b.padding = p
	return b
}

// MergeBorders sets how overlapping border symbols combine.
func (b Block) MergeBorders(strategy symbols.MergeStrategy) Block {
	b.mergeBorders = strategy
	return b
}

// WithShadow attaches a shadow rendered behind the block.
func (b Block) WithShadow(s Shadow) Block {
	cp := s
	b.shadow = &cp
	return b
}

// Inner returns the content area after borders, titles, and padding.
//
// Every subtract saturates at zero. X is clamped so it never past Right().
// A top title (even without a top border) reserves one top row; same for bottom.
func (b Block) Inner(area layout.Rect) layout.Rect {
	inner := area
	if b.borders.Intersects(BorderLeft) {
		inner.X = satAddClamp(inner.X, 1, inner.Right())
		inner.Width = satSub(inner.Width, 1)
	}
	if b.borders.Intersects(BorderTop) || b.hasTitleAtPosition(TitlePositionTop) {
		inner.Y = satAddClamp(inner.Y, 1, inner.Bottom())
		inner.Height = satSub(inner.Height, 1)
	}
	if b.borders.Intersects(BorderRight) {
		inner.Width = satSub(inner.Width, 1)
	}
	if b.borders.Intersects(BorderBottom) || b.hasTitleAtPosition(TitlePositionBottom) {
		inner.Height = satSub(inner.Height, 1)
	}

	inner.X = satAdd(inner.X, b.padding.Left)
	inner.Y = satAdd(inner.Y, b.padding.Top)

	hPad := satAdd(b.padding.Left, b.padding.Right)
	vPad := satAdd(b.padding.Top, b.padding.Bottom)
	inner.Width = satSub(inner.Width, hPad)
	inner.Height = satSub(inner.Height, vPad)
	return inner
}

// HorizontalSpace returns (left, right) cells the block consumes for borders+padding.
func (b Block) HorizontalSpace() (left, right int) {
	left = satAdd(b.padding.Left, boolInt(b.borders.Contains(BorderLeft)))
	right = satAdd(b.padding.Right, boolInt(b.borders.Contains(BorderRight)))
	return left, right
}

// VerticalSpace returns (top, bottom) cells the block consumes for borders, titles, padding.
func (b Block) VerticalSpace() (top, bottom int) {
	hasTop := b.borders.Contains(BorderTop) || b.hasTitleAtPosition(TitlePositionTop)
	hasBottom := b.borders.Contains(BorderBottom) || b.hasTitleAtPosition(TitlePositionBottom)
	top = satAdd(b.padding.Top, boolInt(hasTop))
	bottom = satAdd(b.padding.Bottom, boolInt(hasBottom))
	return top, bottom
}

// Render draws the block into buf within area.
//
// Intersects with buf.Area and returns immediately on empty. Order:
// base style → borders (sides then corners) → titles → shadow.
func (b Block) Render(area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	buf.SetStyle(area, b.style)
	b.renderBorders(area, buf)
	b.renderTitles(area, buf)
	b.renderShadow(area, buf)
}

func (b Block) hasTitleAtPosition(position TitlePosition) bool {
	for i := range b.titles {
		pos := b.titlesPosition
		if b.titles[i].pos != nil {
			pos = *b.titles[i].pos
		}
		if pos == position {
			return true
		}
	}
	return false
}

func (b Block) renderBorders(area layout.Rect, buf *buffer.Buffer) {
	b.renderSides(area, buf)
	b.renderCorners(area, buf)
}

func (b Block) renderSides(area layout.Rect, buf *buffer.Buffer) {
	left := area.X
	top := area.Y
	right := satSub(area.Right(), 1)
	bottom := satSub(area.Bottom(), 1)

	// When merge != Replace, inset side runs by 1 at owned corners so the
	// corner character is written cleanly afterward (upstream block.rs).
	inset := b.mergeBorders != symbols.MergeStrategyReplace
	leftInsetAmt := boolInt(inset && b.borders.Contains(BorderLeft))
	topInsetAmt := boolInt(inset && b.borders.Contains(BorderTop))
	rightInsetAmt := boolInt(inset && b.borders.Contains(BorderRight))
	bottomInsetAmt := boolInt(inset && b.borders.Contains(BorderBottom))

	leftInset := satAdd(left, leftInsetAmt)
	topInset := satAdd(top, topInsetAmt)
	rightInset := satSub(right, rightInsetAmt)
	bottomInset := satSub(bottom, bottomInsetAmt)

	if b.borders.Contains(BorderLeft) {
		b.fillVertical(buf, left, topInset, bottomInset, b.borderSet.VerticalLeft)
	}
	if b.borders.Contains(BorderTop) {
		b.fillHorizontal(buf, leftInset, rightInset, top, b.borderSet.HorizontalTop)
	}
	if b.borders.Contains(BorderRight) {
		b.fillVertical(buf, right, topInset, bottomInset, b.borderSet.VerticalRight)
	}
	if b.borders.Contains(BorderBottom) {
		b.fillHorizontal(buf, leftInset, rightInset, bottom, b.borderSet.HorizontalBottom)
	}
}

func (b Block) renderCorners(area layout.Rect, buf *buffer.Buffer) {
	right := satSub(area.Right(), 1)
	bottom := satSub(area.Bottom(), 1)
	left := area.X
	top := area.Y

	type corner struct {
		need Borders
		x, y int
		sym  string
	}
	corners := []corner{
		{BorderRight | BorderBottom, right, bottom, b.borderSet.BottomRight},
		{BorderRight | BorderTop, right, top, b.borderSet.TopRight},
		{BorderLeft | BorderBottom, left, bottom, b.borderSet.BottomLeft},
		{BorderLeft | BorderTop, left, top, b.borderSet.TopLeft},
	}
	for _, c := range corners {
		if b.borders.Contains(c.need) {
			b.putBorder(buf, c.x, c.y, c.sym)
		}
	}
}

func (b Block) fillHorizontal(buf *buffer.Buffer, x0, x1, y int, sym string) {
	if x1 < x0 {
		return
	}
	for x := x0; x <= x1; x++ {
		b.putBorder(buf, x, y, sym)
	}
}

func (b Block) fillVertical(buf *buffer.Buffer, x, y0, y1 int, sym string) {
	if y1 < y0 {
		return
	}
	for y := y0; y <= y1; y++ {
		b.putBorder(buf, x, y, sym)
	}
}

func (b Block) putBorder(buf *buffer.Buffer, x, y int, sym string) {
	cell := buf.GetMut(x, y)
	if cell == nil {
		return
	}
	mergeCellSymbol(cell, sym, b.mergeBorders)
	cell.SetStyle(b.borderStyle)
}

func (b Block) renderTitles(area layout.Rect, buf *buffer.Buffer) {
	b.renderTitlePosition(TitlePositionTop, area, buf)
	b.renderTitlePosition(TitlePositionBottom, area, buf)
}

func (b Block) renderTitlePosition(position TitlePosition, area layout.Rect, buf *buffer.Buffer) {
	// Order defines overlap: left, then center, then right (upstream).
	b.renderLeftTitles(position, area, buf)
	b.renderCenterTitles(position, area, buf)
	b.renderRightTitles(position, area, buf)
}

func (b Block) renderLeftTitles(position TitlePosition, area layout.Rect, buf *buffer.Buffer) {
	titles := b.filteredTitles(position, layout.AlignLeft)
	titlesArea := b.titlesArea(area, position)
	for i := range titles {
		if titlesArea.IsEmpty() {
			break
		}
		title := titles[i]
		tw := title.Width()
		titleArea := titlesArea
		if tw < titleArea.Width {
			titleArea.Width = tw
		}
		buf.SetStyle(titleArea, b.titlesStyle)
		renderTitleLine(title, titleArea, buf)

		advance := satAdd(tw, 1)
		titlesArea.X = satAdd(titlesArea.X, advance)
		titlesArea.Width = satSub(titlesArea.Width, advance)
	}
}

func (b Block) renderRightTitles(position TitlePosition, area layout.Rect, buf *buffer.Buffer) {
	titles := b.filteredTitles(position, layout.AlignRight)
	titlesArea := b.titlesArea(area, position)
	// Reverse order so successive titles pack against the right edge.
	for i := len(titles) - 1; i >= 0; i-- {
		if titlesArea.IsEmpty() {
			break
		}
		title := titles[i]
		tw := title.Width()
		x := titlesArea.Right() - tw
		if x < titlesArea.X {
			x = titlesArea.X
		}
		w := tw
		if w > titlesArea.Width {
			w = titlesArea.Width
		}
		titleArea := layout.Rect{X: x, Y: titlesArea.Y, Width: w, Height: titlesArea.Height}
		buf.SetStyle(titleArea, b.titlesStyle)
		renderTitleLine(title, titleArea, buf)

		titlesArea.Width = satSub(satSub(titlesArea.Width, tw), 1)
	}
}

func (b Block) renderCenterTitles(position TitlePosition, area layout.Rect, buf *buffer.Buffer) {
	area = b.titlesArea(area, position)
	titles := b.filteredTitles(position, layout.AlignCenter)
	if len(titles) == 0 {
		return
	}
	// Titles rendered with one space after each except the last.
	totalWidth := 0
	for i := range titles {
		totalWidth = satAdd(totalWidth, satAdd(titles[i].Width(), 1))
	}
	totalWidth = satSub(totalWidth, 1)

	if totalWidth <= area.Width {
		b.renderCenteredTitlesWithoutTruncation(titles, totalWidth, area, buf)
	} else {
		b.renderCenteredTitlesWithTruncation(titles, totalWidth, area, buf)
	}
}

func (b Block) renderCenteredTitlesWithoutTruncation(titles []text.Line, totalWidth int, area layout.Rect, buf *buffer.Buffer) {
	x := satAdd(area.X, (satSub(area.Width, totalWidth))/2)
	cur := layout.Rect{X: x, Y: area.Y, Width: area.Width, Height: area.Height}
	// Shrink width to remaining from x.
	cur.Width = satSub(area.Right(), x)
	for i := range titles {
		w := titles[i].Width()
		titleArea := layout.Rect{X: cur.X, Y: cur.Y, Width: w, Height: cur.Height}
		if titleArea.Width > cur.Width {
			titleArea.Width = cur.Width
		}
		buf.SetStyle(titleArea, b.titlesStyle)
		renderTitleLine(titles[i], titleArea, buf)
		advance := satAdd(w, 1)
		cur.X = satAdd(cur.X, advance)
		cur.Width = satSub(cur.Width, advance)
	}
}

func (b Block) renderCenteredTitlesWithTruncation(titles []text.Line, totalWidth int, area layout.Rect, buf *buffer.Buffer) {
	offset := satSub(totalWidth, area.Width) / 2
	for i := range titles {
		if area.IsEmpty() {
			break
		}
		tw := titles[i].Width()
		w := area.Width
		if tw < w {
			w = tw
		}
		w = satSub(w, offset)
		titleArea := layout.Rect{X: area.X, Y: area.Y, Width: w, Height: area.Height}
		buf.SetStyle(titleArea, b.titlesStyle)
		line := titles[i]
		if offset > 0 {
			// Truncate left side: right-align into the truncated window.
			renderTitleLine(line.RightAligned(), titleArea, buf)
			offset = satSub(satSub(offset, w), 1)
		} else {
			renderTitleLine(line.LeftAligned(), titleArea, buf)
		}
		advance := satAdd(w, 1)
		area.X = satAdd(area.X, advance)
		area.Width = satSub(area.Width, advance)
	}
}

func (b Block) filteredTitles(position TitlePosition, alignment layout.Alignment) []text.Line {
	var out []text.Line
	for i := range b.titles {
		pos := b.titlesPosition
		if b.titles[i].pos != nil {
			pos = *b.titles[i].pos
		}
		if pos != position {
			continue
		}
		line := b.titles[i].line
		align := b.titlesAlignment
		if line.Alignment != nil {
			align = *line.Alignment
		}
		if align != alignment {
			continue
		}
		out = append(out, line)
	}
	return out
}

// titlesArea is one row tall spanning the block width minus side borders.
func (b Block) titlesArea(area layout.Rect, position TitlePosition) layout.Rect {
	leftBorder := boolInt(b.borders.Contains(BorderLeft))
	rightBorder := boolInt(b.borders.Contains(BorderRight))
	y := area.Y
	if position == TitlePositionBottom {
		y = satSub(area.Bottom(), 1)
	}
	return layout.Rect{
		X:      satAdd(area.X, leftBorder),
		Y:      y,
		Width:  satSub(satSub(area.Width, leftBorder), rightBorder),
		Height: 1,
	}
}

func (b Block) renderShadow(baseArea layout.Rect, buf *buffer.Buffer) {
	if b.shadow != nil {
		b.shadow.Render(baseArea, buf)
	}
}

// renderTitleLine paints a title line into area using text.Line.RenderData + buffer writes.
//
// Matches Line widget semantics: set line style on area, place spans with alignment.
func renderTitleLine(line text.Line, area layout.Rect, buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	// Titles are single-row.
	area.Height = 1
	if line.Width() == 0 {
		return
	}
	buf.SetStyle(area, line.Style)

	// Force alignment onto a copy so RenderData sees the line's own alignment
	// (title filtering already selected by alignment; keep line as-is).
	spans, _, leftPad := line.RenderData(area.Width)
	x := satAdd(area.X, leftPad)
	remaining := satSub(area.Width, leftPad)
	for i := range spans {
		if remaining <= 0 {
			break
		}
		st := line.Style.Patch(spans[i].Style)
		nx, _ := buf.SetStringN(x, area.Y, spans[i].Content, remaining, st)
		w := nx - x
		if w < 0 {
			w = 0
		}
		x = nx
		remaining = satSub(remaining, w)
	}
}

func copyTitles(in []titleEntry) []titleEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]titleEntry, len(in))
	copy(out, in)
	return out
}

func copyLine(l text.Line) text.Line {
	// text.FromSpanSlice / WithSpans copy spans; rebuild to own the slice.
	if len(l.Spans) == 0 {
		return l
	}
	spans := make([]text.Span, len(l.Spans))
	copy(spans, l.Spans)
	out := text.Line{Spans: spans, Style: l.Style}
	if l.Alignment != nil {
		a := *l.Alignment
		out.Alignment = &a
	}
	return out
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func satAdd(a, b int) int {
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	// Prevent overflow into negative for practical terminal sizes.
	sum := a + b
	if sum < a {
		return int(^uint(0) >> 1) // max int
	}
	return sum
}

func satAddClamp(a, b, max int) int {
	v := satAdd(a, b)
	if v > max {
		return max
	}
	return v
}

func satSub(a, b int) int {
	if b < 0 {
		b = 0
	}
	if a <= b {
		return 0
	}
	return a - b
}
