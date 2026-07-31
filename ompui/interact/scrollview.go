package interact

import (
	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/event"
)

// ScrollbarMode controls when the right-edge scrollbar column is reserved.
type ScrollbarMode uint8

const (
	// ScrollbarAuto reserves a column only when content overflows.
	ScrollbarAuto ScrollbarMode = iota
	// ScrollbarAlways always reserves the scrollbar column.
	ScrollbarAlways
	// ScrollbarNever never draws a scrollbar.
	ScrollbarNever
)

const (
	defaultTrack = "│"
	defaultThumb = "█"
)

// ScrollViewTheme styles the scrollbar track and thumb glyphs.
type ScrollViewTheme struct {
	Track Style
	Thumb Style
}

// ScrollViewOptions configures a fixed-height viewport over pre-rendered lines.
type ScrollViewOptions struct {
	Height          int
	Scrollbar       ScrollbarMode
	TotalRows       int  // used when HasTotalRows
	HasTotalRows    bool // when true, TotalRows is authoritative even if 0
	Theme           ScrollViewTheme
	TrackChar       string
	ThumbChar       string
	Ellipsis        string // default "…"; call SetEllipsis("") after construct to omit
	FastScrollLines int    // default 5
	// FollowTail keeps the viewport pinned to the bottom while content grows,
	// until the user scrolls away. Default false for plain ScrollView.
	FollowTail bool
}

// ScrollView is a fixed-height viewport over pre-rendered lines with an optional
// right-edge scrollbar. It owns only the row offset; callers produce already-
// wrapped logical lines for the current width.
//
// When FollowTail is set, appends that arrive while the viewport is at the
// bottom keep it pinned. Scrolling away sets FollowTail false until ScrollToBottom.
// UnreadCount tracks lines appended while not following.
type ScrollView struct {
	lines           []string
	height          int
	scrollOffset    int
	totalRows       int
	hasTotalRows    bool
	scrollbar       ScrollbarMode
	theme           ScrollViewTheme
	trackChar       string
	thumbChar       string
	ellipsis        string
	fastScrollLines int

	followTail  bool
	wasAtBottom bool
	unreadCount int

	gen    component.Gen
	cached component.Frame
	cacheW int
	dirty  bool
}

// NewScrollView constructs a ScrollView from lines and options.
func NewScrollView(lines []string, opt ScrollViewOptions) *ScrollView {
	sv := &ScrollView{
		height:          maxInt(0, opt.Height),
		scrollbar:       opt.Scrollbar,
		theme:           opt.Theme,
		followTail:      opt.FollowTail,
		wasAtBottom:     true,
		dirty:           true,
		fastScrollLines: 5,
		ellipsis:        "…",
	}
	if opt.TrackChar == "" {
		sv.trackChar = defaultTrack
	} else {
		sv.trackChar = firstCellGlyph(opt.TrackChar, defaultTrack)
	}
	if opt.ThumbChar == "" {
		sv.thumbChar = defaultThumb
	} else {
		sv.thumbChar = firstCellGlyph(opt.ThumbChar, defaultThumb)
	}
	if opt.Ellipsis != "" {
		sv.ellipsis = opt.Ellipsis
	}
	if opt.FastScrollLines > 0 {
		sv.fastScrollLines = opt.FastScrollLines
	}
	if opt.HasTotalRows {
		sv.hasTotalRows = true
		sv.totalRows = maxInt(0, opt.TotalRows)
	}
	sv.lines = append([]string(nil), lines...)
	if opt.FollowTail {
		sv.scrollOffset = sv.MaxScrollOffset()
		sv.wasAtBottom = true
	} else {
		sv.clampScrollOffset()
	}
	return sv
}

// SetEllipsis sets the overflow indicator ("" omits).
func (sv *ScrollView) SetEllipsis(ellipsis string) {
	if sv.ellipsis == ellipsis {
		return
	}
	sv.ellipsis = ellipsis
	sv.dirty = true
}

// SetLines replaces the line buffer. Preserves scroll offset (clamped). When
// follow-tail is active and the view was at bottom, pins to the new bottom and
// clears unread. When not following and lines grow, increments UnreadCount by
// the net growth.
func (sv *ScrollView) SetLines(lines []string) {
	oldLen := sv.rowCount()
	atBottom := sv.scrollOffset >= sv.MaxScrollOffset()
	sv.lines = append([]string(nil), lines...)
	newLen := sv.rowCount()
	if sv.followTail && (atBottom || sv.wasAtBottom) {
		sv.scrollOffset = sv.MaxScrollOffset()
		sv.wasAtBottom = true
		sv.unreadCount = 0
	} else {
		if newLen > oldLen {
			sv.unreadCount += newLen - oldLen
		}
		sv.clampScrollOffset()
		sv.wasAtBottom = sv.scrollOffset >= sv.MaxScrollOffset()
		if sv.wasAtBottom {
			sv.unreadCount = 0
		}
	}
	sv.dirty = true
}

// AppendLines appends rows (follow/unread semantics as SetLines).
func (sv *ScrollView) AppendLines(extra ...string) {
	if len(extra) == 0 {
		return
	}
	atBottom := sv.scrollOffset >= sv.MaxScrollOffset()
	sv.lines = append(sv.lines, extra...)
	if sv.followTail && (atBottom || sv.wasAtBottom) {
		sv.scrollOffset = sv.MaxScrollOffset()
		sv.wasAtBottom = true
		sv.unreadCount = 0
	} else {
		sv.unreadCount += len(extra)
		sv.clampScrollOffset()
		sv.wasAtBottom = sv.scrollOffset >= sv.MaxScrollOffset()
		if sv.wasAtBottom {
			sv.unreadCount = 0
		}
	}
	sv.dirty = true
}

// SetTotalRows sets the logical row count for pre-windowed slices.
// Pass has=false to clear and use len(lines).
func (sv *ScrollView) SetTotalRows(total int, has bool) {
	sv.hasTotalRows = has
	if has {
		sv.totalRows = maxInt(0, total)
	} else {
		sv.totalRows = 0
	}
	sv.clampScrollOffset()
	sv.dirty = true
}

// SetHeight changes the viewport height.
func (sv *ScrollView) SetHeight(height int) {
	h := maxInt(0, height)
	if h == sv.height {
		return
	}
	atBottom := sv.scrollOffset >= sv.MaxScrollOffset()
	sv.height = h
	if sv.followTail && atBottom {
		sv.scrollOffset = sv.MaxScrollOffset()
	} else {
		sv.clampScrollOffset()
	}
	sv.dirty = true
}

// Height returns the viewport height in rows.
func (sv *ScrollView) Height() int { return sv.height }

// SetScrollbar sets the scrollbar mode.
func (sv *ScrollView) SetScrollbar(mode ScrollbarMode) {
	if sv.scrollbar == mode {
		return
	}
	sv.scrollbar = mode
	sv.dirty = true
}

// SetFollowTail enables or disables bottom-follow on append.
func (sv *ScrollView) SetFollowTail(follow bool) {
	sv.followTail = follow
	if follow {
		sv.scrollOffset = sv.MaxScrollOffset()
		sv.wasAtBottom = true
		sv.unreadCount = 0
		sv.dirty = true
	}
}

// FollowTail reports whether tail-follow is enabled.
func (sv *ScrollView) FollowTail() bool { return sv.followTail }

// UnreadCount is the number of lines appended while not at bottom.
func (sv *ScrollView) UnreadCount() int { return sv.unreadCount }

// ClearUnread resets the unread counter.
func (sv *ScrollView) ClearUnread() { sv.unreadCount = 0 }

// ScrollOffset returns the current top-row offset.
func (sv *ScrollView) ScrollOffset() int { return sv.scrollOffset }

// MaxScrollOffset returns the maximum valid offset.
func (sv *ScrollView) MaxScrollOffset() int {
	return maxInt(0, sv.rowCount()-sv.height)
}

// SetScrollOffset sets the top-row offset (clamped). Leaving the bottom disables
// follow-tail pinning until ScrollToBottom / SetFollowTail(true).
func (sv *ScrollView) SetScrollOffset(offset int) {
	sv.scrollOffset = offset
	sv.clampScrollOffset()
	sv.wasAtBottom = sv.scrollOffset >= sv.MaxScrollOffset()
	if sv.wasAtBottom {
		sv.unreadCount = 0
	} else if sv.followTail {
		// User scrolled away: stop auto-pin until they return to bottom.
		sv.followTail = false
	}
	sv.dirty = true
}

// Scroll moves the viewport by delta rows.
func (sv *ScrollView) Scroll(delta int) {
	sv.SetScrollOffset(sv.scrollOffset + delta)
}

// Page scrolls by roughly one page (height-1).
func (sv *ScrollView) Page(delta int) {
	step := maxInt(1, sv.height-1)
	sv.Scroll(step * delta)
}

// ScrollToTop moves to offset 0.
func (sv *ScrollView) ScrollToTop() {
	sv.SetScrollOffset(0)
}

// ScrollToBottom pins to the end and re-enables follow when previously used.
func (sv *ScrollView) ScrollToBottom() {
	sv.scrollOffset = sv.MaxScrollOffset()
	sv.wasAtBottom = true
	sv.unreadCount = 0
	sv.dirty = true
}

// HandleScrollKey applies standard navigation keys. Returns true when consumed.
// Shift+Arrow scrolls by FastScrollLines; plain Arrow by one; Page by page;
// Home/End to ends.
func (sv *ScrollView) HandleScrollKey(ev event.Event) bool {
	if matchKeys(ev, keysScrollFastUp) {
		sv.Scroll(-sv.fastScrollLines)
		return true
	}
	if matchKeys(ev, keysScrollFastDown) {
		sv.Scroll(sv.fastScrollLines)
		return true
	}
	if matchKeys(ev, keysScrollUp) {
		sv.Scroll(-1)
		return true
	}
	if matchKeys(ev, keysScrollDown) {
		sv.Scroll(1)
		return true
	}
	if matchKeys(ev, keysSelectPageUp) {
		sv.Page(-1)
		return true
	}
	if matchKeys(ev, keysSelectPageDown) {
		sv.Page(1)
		return true
	}
	if matchKeys(ev, keysHome) {
		sv.ScrollToTop()
		return true
	}
	if matchKeys(ev, keysEnd) {
		sv.ScrollToBottom()
		return true
	}
	return false
}

// HandleInput implements component.InputHandler via HandleScrollKey.
func (sv *ScrollView) HandleInput(ev event.Event) {
	_ = sv.HandleScrollKey(ev)
}

// Invalidate drops the render cache.
func (sv *ScrollView) Invalidate() {
	sv.dirty = true
	sv.cached = component.Frame{}
	sv.cacheW = -1
}

// Dispose implements component.Disposable (no-op resources).
func (sv *ScrollView) Dispose() {}

// Render implements component.Component.
func (sv *ScrollView) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	sv.clampScrollOffset()
	if !sv.dirty && sv.cacheW == width && sv.cached.Lines != nil {
		return sv.cached
	}

	if sv.height == 0 {
		gen := sv.gen.Touch(sv.dirty || sv.cacheW != width)
		sv.dirty = false
		sv.cacheW = width
		sv.cached = component.EmptyFrame(gen)
		return sv.cached
	}

	showBar := width > 0 && sv.shouldRenderScrollbar()
	contentWidth := maxInt(0, width)
	if showBar {
		contentWidth = maxInt(0, width-1)
	}
	thumb := sv.thumbRange()
	lines := make([]string, 0, sv.height)
	for row := 0; row < sv.height; row++ {
		var sourceIndex int
		if sv.hasTotalRows {
			// Pre-windowed: lines already hold the visible slice; offset is
			// only for thumb math (OMP: sourceIndex = row when totalRows set).
			sourceIndex = row
		} else {
			sourceIndex = sv.scrollOffset + row
		}
		source := ""
		if sourceIndex >= 0 && sourceIndex < len(sv.lines) {
			source = sv.lines[sourceIndex]
		}
		truncated := ansitext.TruncateToWidth(replaceTabs(source), contentWidth, sv.ellipsis)
		if !showBar {
			lines = append(lines, truncated)
			continue
		}
		content := padToWidth(truncated, contentWidth)
		var bar string
		if row >= thumb.start && row < thumb.end {
			bar = applyStyle(sv.theme.Thumb, sv.thumbChar)
		} else {
			bar = applyStyle(sv.theme.Track, sv.trackChar)
		}
		lines = append(lines, content+bar)
	}

	changed := sv.dirty || sv.cacheW != width || !sameLines(sv.cached.Lines, lines)
	gen := sv.gen.Touch(changed)
	sv.dirty = false
	sv.cacheW = width
	if !changed && sv.cached.Lines != nil {
		return sv.cached
	}
	sv.cached = component.NewFrame(lines, gen)
	return sv.cached
}

func (sv *ScrollView) rowCount() int {
	if sv.hasTotalRows {
		return sv.totalRows
	}
	return len(sv.lines)
}

func (sv *ScrollView) clampScrollOffset() {
	sv.scrollOffset = clampInt(sv.scrollOffset, 0, sv.MaxScrollOffset())
}

func (sv *ScrollView) shouldRenderScrollbar() bool {
	if sv.height <= 0 {
		return false
	}
	switch sv.scrollbar {
	case ScrollbarNever:
		return false
	case ScrollbarAlways:
		return true
	default:
		return sv.rowCount() > sv.height
	}
}

type thumbRange struct{ start, end int }

func (sv *ScrollView) thumbRange() thumbRange {
	if sv.height <= 0 {
		return thumbRange{}
	}
	rowCount := sv.rowCount()
	if rowCount <= sv.height {
		return thumbRange{0, sv.height}
	}
	thumbSize := maxInt(1, minInt(sv.height*sv.height/rowCount, sv.height))
	travel := sv.height - thumbSize
	maxOffset := sv.MaxScrollOffset()
	start := 0
	if maxOffset > 0 {
		start = int(float64(sv.scrollOffset)/float64(maxOffset)*float64(travel) + 0.5)
	}
	return thumbRange{start, start + thumbSize}
}
