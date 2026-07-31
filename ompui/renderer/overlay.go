package renderer

import (
	"math"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
)

// OverlayAnchor positions an overlay inside the available margin box.
type OverlayAnchor int

const (
	AnchorCenter OverlayAnchor = iota
	AnchorTopLeft
	AnchorTopCenter
	AnchorTopRight
	AnchorLeftCenter
	AnchorRightCenter
	AnchorBottomLeft
	AnchorBottomCenter
	AnchorBottomRight
)

// SizeValue is either an absolute cell count or a percentage of a base.
type SizeValue struct {
	Absolute  int
	Percent   float64 // 0..100 when IsPercent
	IsPercent bool
	Set       bool
}

// SizeAbs returns an absolute size in cells.
func SizeAbs(n int) SizeValue { return SizeValue{Absolute: n, Set: true} }

// SizePct returns a percentage of the reference dimension (0..100).
func SizePct(p float64) SizeValue { return SizeValue{Percent: p, IsPercent: true, Set: true} }

func (s SizeValue) resolve(base int) (int, bool) {
	if !s.Set {
		return 0, false
	}
	if s.IsPercent {
		v := int(math.Floor(float64(base) * s.Percent / 100.0))
		if v < 0 {
			v = 0
		}
		return v, true
	}
	return s.Absolute, true
}

// OverlayMargin is the outer margin box.
type OverlayMargin struct {
	Top, Right, Bottom, Left int
}

// MarginAll sets equal margins on every side.
func MarginAll(n int) OverlayMargin {
	if n < 0 {
		n = 0
	}
	return OverlayMargin{Top: n, Right: n, Bottom: n, Left: n}
}

// OverlayOptions configure one overlay's geometry and alt-screen borrow.
// Coordinates after layout are screen rows/cols into the viewport window.
type OverlayOptions struct {
	Anchor OverlayAnchor
	// Width defaults to min(80, availWidth). MinWidth floors the resolved width.
	Width    SizeValue
	MinWidth int
	// MaxHeight defaults to availHeight.
	MaxHeight SizeValue
	// Row / Col override anchor when Set. Percent forms keep the overlay inside
	// the margin box (0% = start edge, 100% = end edge).
	Row, Col SizeValue
	// OffsetX / OffsetY are applied after anchor/absolute resolve, then clamped.
	OffsetX, OffsetY int
	Margin           OverlayMargin
	// Fullscreen borrows the alternate screen and paints only this overlay
	// (and those above it on the alt buffer). Transcript ledger is untouched.
	Fullscreen bool
	// Visible, when non-nil, gates whether the overlay paints this frame.
	Visible func(width, height int) bool
}

// Overlay is one stacked modal/popup. Lines are the overlay's own content at
// its resolved width (pre-composite). The engine never feeds these into the
// ledger or committed prefix.
type Overlay struct {
	Lines   []string
	Options OverlayOptions
	// Hidden temporarily suppresses painting without removing the entry.
	Hidden bool
}

// Visible reports whether the overlay should paint for the given terminal size.
func (o Overlay) Visible(termW, termH int) bool {
	if o.Hidden {
		return false
	}
	if o.Options.Visible != nil && !o.Options.Visible(termW, termH) {
		return false
	}
	return true
}

type overlayLayout struct {
	width     int
	row       int
	col       int
	maxHeight int
}

func resolveOverlayLayout(opt OverlayOptions, overlayHeight, termW, termH int) overlayLayout {
	mt := max0(opt.Margin.Top)
	mr := max0(opt.Margin.Right)
	mb := max0(opt.Margin.Bottom)
	ml := max0(opt.Margin.Left)

	availW := max1(termW - ml - mr)
	availH := max1(termH - mt - mb)

	width := minInt(80, availW)
	if v, ok := opt.Width.resolve(termW); ok {
		width = v
	}
	if opt.MinWidth > width {
		width = opt.MinWidth
	}
	if width < 1 {
		width = 1
	}
	if width > availW {
		width = availW
	}

	maxH := availH
	if v, ok := opt.MaxHeight.resolve(termH); ok {
		maxH = v
	}
	if maxH < 1 {
		maxH = 1
	}
	if maxH > availH {
		maxH = availH
	}

	effH := overlayHeight
	if effH > maxH {
		effH = maxH
	}

	var row, col int
	if opt.Row.Set {
		if opt.Row.IsPercent {
			maxRow := max0(availH - effH)
			pct := opt.Row.Percent / 100.0
			row = mt + int(math.Floor(float64(maxRow)*pct))
		} else {
			row = opt.Row.Absolute
		}
	} else {
		row = resolveAnchorRow(opt.Anchor, effH, availH, mt)
	}
	if opt.Col.Set {
		if opt.Col.IsPercent {
			maxCol := max0(availW - width)
			pct := opt.Col.Percent / 100.0
			col = ml + int(math.Floor(float64(maxCol)*pct))
		} else {
			col = opt.Col.Absolute
		}
	} else {
		col = resolveAnchorCol(opt.Anchor, width, availW, ml)
	}

	row += opt.OffsetY
	col += opt.OffsetX

	if row < mt {
		row = mt
	}
	if maxR := termH - mb - effH; row > maxR {
		row = maxR
	}
	if col < ml {
		col = ml
	}
	if maxC := termW - mr - width; col > maxC {
		col = maxC
	}

	return overlayLayout{width: width, row: row, col: col, maxHeight: maxH}
}

func resolveAnchorRow(a OverlayAnchor, height, availH, marginTop int) int {
	switch a {
	case AnchorTopLeft, AnchorTopCenter, AnchorTopRight:
		return marginTop
	case AnchorBottomLeft, AnchorBottomCenter, AnchorBottomRight:
		return marginTop + availH - height
	default:
		return marginTop + (availH-height)/2
	}
}

func resolveAnchorCol(a OverlayAnchor, width, availW, marginLeft int) int {
	switch a {
	case AnchorTopLeft, AnchorLeftCenter, AnchorBottomLeft:
		return marginLeft
	case AnchorTopRight, AnchorRightCenter, AnchorBottomRight:
		return marginLeft + availW - width
	default:
		return marginLeft + (availW-width)/2
	}
}

// compositeOverlaysIntoWindow paints visible overlays into a copy of the window
// slice (screen coordinates, stack order). The frame/ledger are never touched.
func compositeOverlaysIntoWindow(window []string, overlays []Overlay, termW, termH int, isImage func(string) bool) []string {
	if len(overlays) == 0 {
		return window
	}
	result := make([]string, len(window))
	copy(result, window)
	for _, entry := range overlays {
		if !entry.Visible(termW, termH) {
			continue
		}
		lay0 := resolveOverlayLayout(entry.Options, 0, termW, termH)
		lines := entry.Lines
		if len(lines) > lay0.maxHeight {
			anchor := entry.Options.Anchor
			if anchor == AnchorBottomLeft || anchor == AnchorBottomCenter || anchor == AnchorBottomRight {
				lines = lines[len(lines)-lay0.maxHeight:]
			} else {
				lines = lines[:lay0.maxHeight]
			}
		}
		lay := resolveOverlayLayout(entry.Options, len(lines), termW, termH)
		for i, ol := range lines {
			idx := lay.row + i
			if idx < 0 || idx >= len(result) {
				continue
			}
			line := ol
			if ansitext.VisibleWidth(line) > lay.width {
				line = ansitext.SliceByColumn(line, 0, lay.width)
			}
			result[idx] = compositeLineAt(result[idx], line, lay.col, lay.width, termW, isImage)
		}
	}
	return result
}

// compositeLineAt splices overlay content into baseLine at startCol.
func compositeLineAt(baseLine, overlayLine string, startCol, overlayWidth, totalWidth int, isImage func(string) bool) string {
	if isImage != nil && isImage(baseLine) {
		return baseLine
	}
	if startCol < 0 {
		startCol = 0
	}
	if overlayWidth < 0 {
		overlayWidth = 0
	}
	if totalWidth < 1 {
		totalWidth = 1
	}

	afterStart := startCol + overlayWidth
	before := ansitext.SliceByColumn(baseLine, 0, startCol)
	afterLen := totalWidth - afterStart
	if afterLen < 0 {
		afterLen = 0
	}
	after := ""
	if afterStart < totalWidth {
		after = ansitext.SliceByColumn(baseLine, afterStart, afterLen+64)
	}
	overlay := ansitext.SliceByColumn(overlayLine, 0, overlayWidth)

	beforeW := ansitext.VisibleWidth(before)
	overlayW := ansitext.VisibleWidth(overlay)
	afterW := ansitext.VisibleWidth(after)

	beforePad := max0(startCol - beforeW)
	overlayPad := max0(overlayWidth - overlayW)
	actualBefore := maxInt(startCol, beforeW)
	actualOverlay := maxInt(overlayWidth, overlayW)
	afterTarget := max0(totalWidth - actualBefore - actualOverlay)
	afterPad := max0(afterTarget - afterW)

	r := ansitext.SegmentReset
	var b strings.Builder
	b.Grow(len(before) + len(overlay) + len(after) + beforePad + overlayPad + afterPad + 16)
	b.WriteString(before)
	for range beforePad {
		b.WriteByte(' ')
	}
	b.WriteString(r)
	b.WriteString(overlay)
	for range overlayPad {
		b.WriteByte(' ')
	}
	b.WriteString(r)
	b.WriteString(after)
	for range afterPad {
		b.WriteByte(' ')
	}
	result := b.String()
	if ansitext.VisibleWidth(result) <= totalWidth {
		return result
	}
	return ansitext.SliceByColumn(result, 0, totalWidth)
}

// hasVisibleOverlay reports whether any overlay paints this frame.
func hasVisibleOverlay(overlays []Overlay, termW, termH int) bool {
	for _, o := range overlays {
		if o.Visible(termW, termH) {
			return true
		}
	}
	return false
}

// wantsAltScreen is true when the topmost visible overlay requests fullscreen.
func wantsAltScreen(overlays []Overlay, termW, termH int) bool {
	for i := len(overlays) - 1; i >= 0; i-- {
		if !overlays[i].Visible(termW, termH) {
			continue
		}
		return overlays[i].Options.Fullscreen
	}
	return false
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
