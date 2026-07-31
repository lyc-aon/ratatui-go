package widgets

import (
	"github.com/michaelkelly/ratatui-go/buffer"
	"github.com/michaelkelly/ratatui-go/layout"
	"github.com/michaelkelly/ratatui-go/style"
	"github.com/michaelkelly/ratatui-go/symbols"
	"github.com/michaelkelly/ratatui-go/text"
)

// ScrollbarOrientation is where the scrollbar sits around a given area.
type ScrollbarOrientation int

const (
	// ScrollbarVerticalRight is the default: right edge, vertical scroll.
	ScrollbarVerticalRight ScrollbarOrientation = iota
	// ScrollbarVerticalLeft places the bar on the left edge.
	ScrollbarVerticalLeft
	// ScrollbarHorizontalBottom places the bar on the bottom edge.
	ScrollbarHorizontalBottom
	// ScrollbarHorizontalTop places the bar on the top edge.
	ScrollbarHorizontalTop
)

// String returns a stable name for the orientation.
func (o ScrollbarOrientation) String() string {
	switch o {
	case ScrollbarVerticalLeft:
		return "VerticalLeft"
	case ScrollbarHorizontalBottom:
		return "HorizontalBottom"
	case ScrollbarHorizontalTop:
		return "HorizontalTop"
	default:
		return "VerticalRight"
	}
}

// IsVertical reports whether the orientation scrolls on the vertical axis.
func (o ScrollbarOrientation) IsVertical() bool {
	return o == ScrollbarVerticalRight || o == ScrollbarVerticalLeft
}

// IsHorizontal reports whether the orientation scrolls on the horizontal axis.
func (o ScrollbarOrientation) IsHorizontal() bool {
	return o == ScrollbarHorizontalBottom || o == ScrollbarHorizontalTop
}

// ScrollDirection is used with ScrollbarState.Scroll.
type ScrollDirection int

const (
	// ScrollForward usually means down or right.
	ScrollForward ScrollDirection = iota
	// ScrollBackward usually means up or left.
	ScrollBackward
)

// String returns a stable name for the scroll direction.
func (d ScrollDirection) String() string {
	switch d {
	case ScrollBackward:
		return "Backward"
	default:
		return "Forward"
	}
}

// Scrollbar draws a track/thumb/arrow bar along one edge of an area.
//
// Only RenderStateful is provided (no stateless Render). Zero content length
// or zero track length is a no-op and never divides by zero.
type Scrollbar struct {
	orientation ScrollbarOrientation
	thumbStyle  style.Style
	thumbSymbol string
	trackStyle  style.Style
	trackSymbol *string
	beginSymbol *string
	beginStyle  style.Style
	endSymbol   *string
	endStyle    style.Style
}

// NewScrollbar creates a scrollbar for the given orientation with double-line
// symbols (vertical or horizontal set chosen from orientation).
func NewScrollbar(orientation ScrollbarOrientation) Scrollbar {
	set := symbols.ScrollbarDoubleVertical
	if orientation.IsHorizontal() {
		set = symbols.ScrollbarDoubleHorizontal
	}
	return newScrollbarWithSymbols(orientation, set)
}

func newScrollbarWithSymbols(orientation ScrollbarOrientation, set symbols.ScrollbarSet) Scrollbar {
	track := set.Track
	begin := set.Begin
	end := set.End
	return Scrollbar{
		orientation: orientation,
		thumbSymbol: set.Thumb,
		trackSymbol: &track,
		beginSymbol: &begin,
		endSymbol:   &end,
	}
}

// Orientation sets the bar position and resets symbols to the matching double set.
func (s Scrollbar) Orientation(orientation ScrollbarOrientation) Scrollbar {
	s.orientation = orientation
	set := symbols.ScrollbarDoubleVertical
	if orientation.IsHorizontal() {
		set = symbols.ScrollbarDoubleHorizontal
	}
	return s.Symbols(set)
}

// OrientationAndSymbols sets orientation and symbol set together.
func (s Scrollbar) OrientationAndSymbols(orientation ScrollbarOrientation, set symbols.ScrollbarSet) Scrollbar {
	s.orientation = orientation
	return s.Symbols(set)
}

// ThumbSymbol sets the thumb (handle) symbol.
func (s Scrollbar) ThumbSymbol(sym string) Scrollbar {
	s.thumbSymbol = sym
	return s
}

// ThumbStyle sets the thumb style.
func (s Scrollbar) ThumbStyle(st style.Style) Scrollbar {
	s.thumbStyle = st
	return s
}

// TrackSymbol sets the optional track symbol. Pass nil to draw no track cells.
func (s Scrollbar) TrackSymbol(sym *string) Scrollbar {
	if sym == nil {
		s.trackSymbol = nil
		return s
	}
	v := *sym
	s.trackSymbol = &v
	return s
}

// TrackStyle sets the track style.
func (s Scrollbar) TrackStyle(st style.Style) Scrollbar {
	s.trackStyle = st
	return s
}

// BeginSymbol sets the optional begin-arrow symbol. Pass nil to omit it.
func (s Scrollbar) BeginSymbol(sym *string) Scrollbar {
	if sym == nil {
		s.beginSymbol = nil
		return s
	}
	v := *sym
	s.beginSymbol = &v
	return s
}

// BeginStyle sets the begin-arrow style.
func (s Scrollbar) BeginStyle(st style.Style) Scrollbar {
	s.beginStyle = st
	return s
}

// EndSymbol sets the optional end-arrow symbol. Pass nil to omit it.
func (s Scrollbar) EndSymbol(sym *string) Scrollbar {
	if sym == nil {
		s.endSymbol = nil
		return s
	}
	v := *sym
	s.endSymbol = &v
	return s
}

// EndStyle sets the end-arrow style.
func (s Scrollbar) EndStyle(st style.Style) Scrollbar {
	s.endStyle = st
	return s
}

// Symbols applies a symbol set. Track/begin/end are only replaced when that
// part is currently non-nil (explicit nil stays nil).
func (s Scrollbar) Symbols(set symbols.ScrollbarSet) Scrollbar {
	s.thumbSymbol = set.Thumb
	if s.trackSymbol != nil {
		v := set.Track
		s.trackSymbol = &v
	}
	if s.beginSymbol != nil {
		v := set.Begin
		s.beginSymbol = &v
	}
	if s.endSymbol != nil {
		v := set.End
		s.endSymbol = &v
	}
	return s
}

// Style applies one style to track, thumb, begin, and end.
func (s Scrollbar) Style(st style.Style) Scrollbar {
	s.trackStyle = st
	s.thumbStyle = st
	s.beginStyle = st
	s.endStyle = st
	return s
}

// ScrollbarState is the scroll position/length fed to Scrollbar.RenderStateful.
//
// contentLength must be set or the bar renders blank (no-op).
type ScrollbarState struct {
	contentLength         int
	position              int
	viewportContentLength int
}

// NewScrollbarState constructs state with the given content length.
func NewScrollbarState(contentLength int) ScrollbarState {
	if contentLength < 0 {
		contentLength = 0
	}
	return ScrollbarState{contentLength: contentLength}
}

// Position sets the scroll position (fluent).
func (s ScrollbarState) Position(position int) ScrollbarState {
	if position < 0 {
		position = 0
	}
	s.position = position
	return s
}

// ContentLength sets the total scrollable length (fluent).
func (s ScrollbarState) ContentLength(n int) ScrollbarState {
	if n < 0 {
		n = 0
	}
	s.contentLength = n
	return s
}

// ViewportContentLength sets the viewport size in content units (fluent).
// Zero means "use the track length at render time".
func (s ScrollbarState) ViewportContentLength(n int) ScrollbarState {
	if n < 0 {
		n = 0
	}
	s.viewportContentLength = n
	return s
}

// GetPosition returns the current scroll position.
func (s ScrollbarState) GetPosition() int {
	return s.position
}

// GetContentLength returns the total content length.
func (s ScrollbarState) GetContentLength() int {
	return s.contentLength
}

// GetViewportContentLength returns the configured viewport length (0 = default).
func (s ScrollbarState) GetViewportContentLength() int {
	return s.viewportContentLength
}

// SetPosition mutates the scroll position.
func (s *ScrollbarState) SetPosition(position int) {
	if s == nil {
		return
	}
	if position < 0 {
		position = 0
	}
	s.position = position
}

// SetContentLength mutates the content length.
func (s *ScrollbarState) SetContentLength(n int) {
	if s == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s.contentLength = n
}

// SetViewportContentLength mutates the viewport length.
func (s *ScrollbarState) SetViewportContentLength(n int) {
	if s == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s.viewportContentLength = n
}

// Prev moves the position back by one (not below zero).
func (s *ScrollbarState) Prev() {
	if s == nil {
		return
	}
	if s.position > 0 {
		s.position--
	}
}

// Next moves the position forward by one, clamped to contentLength-1.
func (s *ScrollbarState) Next() {
	if s == nil {
		return
	}
	maxPos := s.contentLength - 1
	if maxPos < 0 {
		maxPos = 0
	}
	if s.position < maxPos {
		s.position++
	}
}

// First jumps to position 0.
func (s *ScrollbarState) First() {
	if s == nil {
		return
	}
	s.position = 0
}

// Last jumps to the last valid position.
func (s *ScrollbarState) Last() {
	if s == nil {
		return
	}
	if s.contentLength <= 0 {
		s.position = 0
		return
	}
	s.position = s.contentLength - 1
}

// Scroll steps once in the given direction.
func (s *ScrollbarState) Scroll(dir ScrollDirection) {
	if s == nil {
		return
	}
	switch dir {
	case ScrollBackward:
		s.Prev()
	default:
		s.Next()
	}
}

// RenderStateful paints the scrollbar for state into area.
//
// No-op when content length is 0 or the track (area minus arrow heads) is 0.
// Never divides by zero.
func (s Scrollbar) RenderStateful(area layout.Rect, buf *buffer.Buffer, state *ScrollbarState) {
	if buf == nil || state == nil {
		return
	}
	area = area.Intersection(buf.Area)
	if area.IsEmpty() {
		return
	}
	if state.contentLength == 0 || s.trackLengthExcludingArrowHeads(area) == 0 {
		return
	}
	barArea, ok := s.scrollbarArea(area)
	if !ok || barArea.IsEmpty() {
		return
	}

	syms := s.barSymbols(barArea, state)
	cells := barCells(barArea)
	n := len(cells)
	if len(syms) < n {
		n = len(syms)
	}
	for i := range n {
		sym := syms[i]
		if sym == nil {
			continue
		}
		buf.SetString(cells[i].X, cells[i].Y, sym.symbol, sym.style)
	}
}

type barSym struct {
	symbol string
	style  style.Style
}

// barSymbols builds one optional symbol per track cell, including begin/end.
func (s Scrollbar) barSymbols(area layout.Rect, state *ScrollbarState) []*barSym {
	trackStart, thumbLen, trackEnd := s.partLengths(area, state)

	out := make([]*barSym, 0, trackStart+thumbLen+trackEnd+2)
	if s.beginSymbol != nil {
		out = append(out, &barSym{symbol: *s.beginSymbol, style: s.beginStyle})
	}
	for range trackStart {
		if s.trackSymbol == nil {
			out = append(out, nil)
		} else {
			out = append(out, &barSym{symbol: *s.trackSymbol, style: s.trackStyle})
		}
	}
	for range thumbLen {
		out = append(out, &barSym{symbol: s.thumbSymbol, style: s.thumbStyle})
	}
	for range trackEnd {
		if s.trackSymbol == nil {
			out = append(out, nil)
		} else {
			out = append(out, &barSym{symbol: *s.trackSymbol, style: s.trackStyle})
		}
	}
	if s.endSymbol != nil {
		out = append(out, &barSym{symbol: *s.endSymbol, style: s.endStyle})
	}
	return out
}

// partLengths returns (track_start, thumb, track_end) cell counts along the track.
func (s Scrollbar) partLengths(area layout.Rect, state *ScrollbarState) (int, int, int) {
	trackLength := s.trackLengthExcludingArrowHeads(area)
	if trackLength == 0 {
		return 0, 0, 0
	}

	viewportLength := s.viewportLength(state, area)
	maxPosition := state.contentLength - 1
	if maxPosition < 0 {
		maxPosition = 0
	}
	startPosition := state.position
	if startPosition < 0 {
		startPosition = 0
	}
	if startPosition > maxPosition {
		startPosition = maxPosition
	}
	maxViewportPosition := maxPosition + viewportLength
	if maxViewportPosition == 0 {
		// Defend against divide-by-zero; full thumb.
		return 0, trackLength, 0
	}

	thumbLength := roundingDivide(viewportLength*trackLength, maxViewportPosition)
	if thumbLength < 1 {
		thumbLength = 1
	}
	if thumbLength > trackLength {
		thumbLength = trackLength
	}

	// Clamp thumb start so thumb always fits in the track (see ratatui #2582).
	thumbStart := roundingDivide(startPosition*trackLength, maxViewportPosition)
	maxStart := trackLength - thumbLength
	if maxStart < 0 {
		maxStart = 0
	}
	if thumbStart < 0 {
		thumbStart = 0
	}
	if thumbStart > maxStart {
		thumbStart = maxStart
	}

	trackEnd := trackLength - (thumbStart + thumbLength)
	if trackEnd < 0 {
		trackEnd = 0
	}
	return thumbStart, thumbLength, trackEnd
}

func (s Scrollbar) scrollbarArea(area layout.Rect) (layout.Rect, bool) {
	switch s.orientation {
	case ScrollbarVerticalLeft:
		cols := area.Columns()
		if len(cols) == 0 {
			return layout.Rect{}, false
		}
		return cols[0], true
	case ScrollbarVerticalRight:
		cols := area.Columns()
		if len(cols) == 0 {
			return layout.Rect{}, false
		}
		return cols[len(cols)-1], true
	case ScrollbarHorizontalTop:
		rows := area.Rows()
		if len(rows) == 0 {
			return layout.Rect{}, false
		}
		return rows[0], true
	case ScrollbarHorizontalBottom:
		rows := area.Rows()
		if len(rows) == 0 {
			return layout.Rect{}, false
		}
		return rows[len(rows)-1], true
	default:
		cols := area.Columns()
		if len(cols) == 0 {
			return layout.Rect{}, false
		}
		return cols[len(cols)-1], true
	}
}

// trackLengthExcludingArrowHeads is the scrolleable track length in cells.
func (s Scrollbar) trackLengthExcludingArrowHeads(area layout.Rect) int {
	startLen := 0
	if s.beginSymbol != nil {
		startLen = text.GraphemeWidth(*s.beginSymbol)
	}
	endLen := 0
	if s.endSymbol != nil {
		endLen = text.GraphemeWidth(*s.endSymbol)
	}
	arrows := startLen + endLen
	if s.orientation.IsVertical() {
		h := area.Height - arrows
		if h < 0 {
			return 0
		}
		return h
	}
	w := area.Width - arrows
	if w < 0 {
		return 0
	}
	return w
}

func (s Scrollbar) viewportLength(state *ScrollbarState, area layout.Rect) int {
	if state.viewportContentLength != 0 {
		return state.viewportContentLength
	}
	if s.orientation.IsVertical() {
		return area.Height
	}
	return area.Width
}

// roundingDivide rounds numerator/denominator to nearest int (halves up).
// Matches Ratatui scrollbar::rounding_divide.
func roundingDivide(numerator, denominator int) int {
	if denominator == 0 {
		return 0
	}
	return (numerator + denominator/2) / denominator
}

// barCells returns the 1×1 cells of a scrollbar strip in paint order.
// Vertical: top→bottom. Horizontal: left→right.
// Mirrors Rust area.columns().flat_map(Rect::rows).
func barCells(area layout.Rect) []layout.Rect {
	if area.IsEmpty() {
		return nil
	}
	out := make([]layout.Rect, 0, area.Width*area.Height)
	for _, col := range area.Columns() {
		for _, row := range col.Rows() {
			out = append(out, row)
		}
	}
	return out
}
