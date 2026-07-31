package layout

import "math"

// Layout divides a rectangular area into segments using constraints, flex, margin, and spacing.
//
// Builder methods are value-style: they return a modified copy and never mutate the receiver.
type Layout struct {
	direction   Direction
	constraints []Constraint
	margin      Margin
	flex        Flex
	spacing     int // positive = gap cells; negative = overlap cells
}

// DefaultFlex is the flex mode used by new layouts (matches ratatui Flex::Start).
const DefaultFlex = FlexStart

// New returns a Layout with the given direction and constraints.
// Default flex is FlexStart; margin and spacing are zero.
func New(direction Direction, constraints ...Constraint) Layout {
	return Layout{
		direction:   direction,
		constraints: copyConstraints(constraints),
		flex:        DefaultFlex,
	}
}

// Vertical returns a vertical layout (top to bottom) with the given constraints.
func Vertical(constraints ...Constraint) Layout {
	return New(VerticalDir, constraints...)
}

// Horizontal returns a horizontal layout (left to right) with the given constraints.
func Horizontal(constraints ...Constraint) Layout {
	return New(HorizontalDir, constraints...)
}

// Direction returns a copy with the split axis set.
func (l Layout) Direction(d Direction) Layout {
	l.direction = d
	return l
}

// Constraints returns a copy with replaced constraints (slice is copied).
func (l Layout) Constraints(constraints ...Constraint) Layout {
	l.constraints = copyConstraints(constraints)
	return l
}

// Margin returns a copy with uniform margin on both axes.
func (l Layout) Margin(n int) Layout {
	n = clampNonNeg(n)
	l.margin = Margin{Horizontal: n, Vertical: n}
	return l
}

// HorizontalMargin returns a copy with only the horizontal margin changed.
func (l Layout) HorizontalMargin(n int) Layout {
	l.margin.Horizontal = clampNonNeg(n)
	return l
}

// VerticalMargin returns a copy with only the vertical margin changed.
func (l Layout) VerticalMargin(n int) Layout {
	l.margin.Vertical = clampNonNeg(n)
	return l
}

// Flex returns a copy with the given flex mode.
func (l Layout) Flex(f Flex) Layout {
	l.flex = f
	return l
}

// Spacing returns a copy with the gap (positive) or overlap (negative) between segments.
func (l Layout) Spacing(n int) Layout {
	l.spacing = n
	return l
}

// Split divides area into one rectangle per constraint.
func (l Layout) Split(area Rect) []Rect {
	segments, _ := l.SplitWithSpacers(area)
	return segments
}

// SplitWithSpacers divides area and also returns the spacer rectangles between segments.
// There is always one more spacer than segments (before, between, and after).
//
// Results are cached in a process-wide LRU (DefaultCacheSize) for common layouts
// with at most 32 constraints. Callers always receive fresh slices.
func (l Layout) SplitWithSpacers(area Rect) (segments, spacers []Rect) {
	if key, ok := l.cacheKey(area); ok {
		if v, hit := globalLayoutCache.get(key); hit {
			return v.segments, v.spacers
		}
		// Compute outside the cache mutex so concurrent misses progress in parallel.
		segments, spacers = l.splitUncached(area)
		// Duplicate race inserts are fine; last writer wins / both store equivalent results.
		globalLayoutCache.put(key, segments, spacers)
		return segments, spacers
	}
	return l.splitUncached(area)
}

// splitUncached performs the actual split without consulting the layout cache.
func (l Layout) splitUncached(area Rect) (segments, spacers []Rect) {
	inner := area.Inner(l.margin)
	n := len(l.constraints)
	if n == 0 {
		// Lone spacer spans the entire inner area (upstream edge-chaining).
		sp := make([]Rect, 1)
		sp[0] = alongAxis(inner, l.direction, axisStart(inner, l.direction), axisSize(inner, l.direction))
		return nil, sp
	}

	total := axisSize(inner, l.direction)
	sizes, spacerSizes := allocate(total, l.constraints, l.flex, l.spacing)

	segments = make([]Rect, n)
	spacers = make([]Rect, n+1)

	cursor := axisStart(inner, l.direction)
	for i := range n {
		sp := spacerSizes[i]
		// Spacer rect uses non-negative extent; overlap is expressed by cursor movement.
		spExtent := sp
		if spExtent < 0 {
			spExtent = 0
		}
		spacers[i] = alongAxis(inner, l.direction, cursor, spExtent)
		cursor += sp
		// Keep segment start inside the inner area when overlap pulls left/up.
		segStart := cursor
		if segStart < axisStart(inner, l.direction) {
			segStart = axisStart(inner, l.direction)
		}
		segments[i] = alongAxis(inner, l.direction, segStart, sizes[i])
		cursor = segStart + sizes[i]
	}
	lastSp := spacerSizes[n]
	if lastSp < 0 {
		lastSp = 0
	}
	spacers[n] = alongAxis(inner, l.direction, cursor, lastSp)
	return segments, spacers
}

func copyConstraints(in []Constraint) []Constraint {
	if len(in) == 0 {
		return nil
	}
	out := make([]Constraint, len(in))
	copy(out, in)
	return out
}

func axisSize(r Rect, d Direction) int {
	if d == HorizontalDir {
		return r.Width
	}
	return r.Height
}

func axisStart(r Rect, d Direction) int {
	if d == HorizontalDir {
		return r.X
	}
	return r.Y
}

func alongAxis(area Rect, d Direction, start, size int) Rect {
	size = clampNonNeg(size)
	// Negative spacers (overlap) move the cursor backward; size stored on spacer rect is abs.
	if d == HorizontalDir {
		return Rect{X: start, Y: area.Y, Width: size, Height: area.Height}
	}
	return Rect{X: area.X, Y: start, Width: area.Width, Height: size}
}

// allocate computes segment sizes and spacer sizes (len spacers = n+1) for the main axis.
//
// Mirrors ratatui 0.30.2's kasuari constraint setup (layout.rs try_split / configure_*).
func allocate(total int, constraints []Constraint, flex Flex, spacing int) (sizes, spacers []int) {
	n := len(constraints)
	sizes = make([]int, n)
	spacers = make([]int, n+1)
	if n == 0 {
		return sizes, spacers
	}
	total = clampNonNeg(total)
	const prec = 100.0

	solver := newCSolver()
	nv := 2*n + 2
	vars := make([]cVar, nv)
	for i := range vars {
		vars[i] = solver.newVar()
	}
	type elem struct{ start, end cVar }
	segE := make([]elem, n)
	spE := make([]elem, n+1)
	for i := 0; i < n+1; i++ {
		spE[i] = elem{start: vars[2*i], end: vars[2*i+1]}
	}
	for i := 0; i < n; i++ {
		segE[i] = elem{start: vars[2*i+1], end: vars[2*i+2]}
	}
	areaStart := 0.0
	areaEnd := float64(total) * prec
	areaStartV, areaEndV := vars[0], vars[nv-1]

	add := func(cn cConstraint) { _ = solver.addConstraint(cn) }
	sz := func(e elem) cExpr { return exprSize(e.start, e.end) }
	areaSize := exprSize(areaStartV, areaEndV)

	// configure_area
	add(cnEQ(exprVar(areaStartV), strengthRequired, areaStart))
	add(cnEQ(exprVar(areaEndV), strengthRequired, areaEnd))

	// configure_variable_in_area_constraints
	for _, v := range vars {
		add(cnGE(exprVar(v), strengthRequired, areaStart))
		add(cnLE(exprVar(v), strengthRequired, areaEnd))
	}

	// configure_variable_constraints: variables.skip(1).tuples() => (v1,v2),(v3,v4),...
	for i := 1; i+1 < nv; i += 2 {
		add(cConstraint{
			expr: exprVar(vars[i]).sub(exprVar(vars[i+1])),
			op:   relLE,
			str:  strengthRequired,
		})
	}

	// configure_flex_constraints
	spacingF := float64(spacing) * prec
	var spacersMid []elem
	if len(spE) >= 2 {
		spacersMid = spE[1 : len(spE)-1]
	}

	emptyStr := strengthSub(strengthRequired, strengthWeak)

	switch flex {
	case FlexLegacy:
		for _, sp := range spacersMid {
			add(cnEQ(sz(sp), strSpacerSizeEQ, spacingF))
		}
		if len(spE) > 0 {
			add(cnEQ(sz(spE[0]), emptyStr, 0))
			add(cnEQ(sz(spE[len(spE)-1]), emptyStr, 0))
		}
	case FlexSpaceAround:
		if len(spE) <= 2 {
			for i := 0; i < len(spE); i++ {
				for j := i + 1; j < len(spE); j++ {
					add(cnEQExpr(sz(spE[i]), strSpacerSizeEQ, sz(spE[j])))
				}
			}
			for _, sp := range spE {
				add(cnGE(sz(sp), strSpacerSizeEQ, spacingF))
				add(cnEQExpr(sz(sp), strSpaceGrow, areaSize))
			}
		} else {
			first, last := spE[0], spE[len(spE)-1]
			middle := spE[1 : len(spE)-1]
			for i := 0; i < len(middle); i++ {
				for j := i + 1; j < len(middle); j++ {
					add(cnEQExpr(sz(middle[i]), strSpacerSizeEQ, sz(middle[j])))
				}
			}
			if len(middle) > 0 {
				add(cnEQExpr(sz(middle[0]), strSpacerSizeEQ, sz(first).mul(2)))
				add(cnEQExpr(sz(middle[0]), strSpacerSizeEQ, sz(last).mul(2)))
			}
			for _, sp := range spE {
				add(cnGE(sz(sp), strSpacerSizeEQ, spacingF))
				add(cnEQExpr(sz(sp), strSpaceGrow, areaSize))
			}
		}
	case FlexSpaceEvenly:
		for i := 0; i < len(spE); i++ {
			for j := i + 1; j < len(spE); j++ {
				add(cnEQExpr(sz(spE[i]), strSpacerSizeEQ, sz(spE[j])))
			}
		}
		for _, sp := range spE {
			add(cnGE(sz(sp), strSpacerSizeEQ, spacingF))
			add(cnEQExpr(sz(sp), strSpaceGrow, areaSize))
		}
	case FlexSpaceBetween:
		for i := 0; i < len(spacersMid); i++ {
			for j := i + 1; j < len(spacersMid); j++ {
				add(cnEQExpr(sz(spacersMid[i]), strSpacerSizeEQ, sz(spacersMid[j])))
			}
		}
		for _, sp := range spacersMid {
			add(cnGE(sz(sp), strSpacerSizeEQ, spacingF))
			add(cnEQExpr(sz(sp), strSpaceGrow, areaSize))
		}
		if len(spE) > 0 {
			add(cnEQ(sz(spE[0]), emptyStr, 0))
			add(cnEQ(sz(spE[len(spE)-1]), emptyStr, 0))
		}
	case FlexStart:
		for _, sp := range spacersMid {
			add(cnEQ(sz(sp), strSpacerSizeEQ, spacingF))
		}
		if len(spE) > 0 {
			add(cnEQ(sz(spE[0]), emptyStr, 0))
			add(cnEQExpr(sz(spE[len(spE)-1]), strGrow, areaSize))
		}
	case FlexCenter:
		for _, sp := range spacersMid {
			add(cnEQ(sz(sp), strSpacerSizeEQ, spacingF))
		}
		if len(spE) > 0 {
			first, last := spE[0], spE[len(spE)-1]
			add(cnEQExpr(sz(first), strGrow, areaSize))
			add(cnEQExpr(sz(last), strGrow, areaSize))
			add(cnEQExpr(sz(first), strSpacerSizeEQ, sz(last)))
		}
	case FlexEnd:
		for _, sp := range spacersMid {
			add(cnEQ(sz(sp), strSpacerSizeEQ, spacingF))
		}
		if len(spE) > 0 {
			add(cnEQ(sz(spE[len(spE)-1]), emptyStr, 0))
			add(cnEQExpr(sz(spE[0]), strGrow, areaSize))
		}
	default:
		// treat as Start
		for _, sp := range spacersMid {
			add(cnEQ(sz(sp), strSpacerSizeEQ, spacingF))
		}
		if len(spE) > 0 {
			add(cnEQ(sz(spE[0]), emptyStr, 0))
			add(cnEQExpr(sz(spE[len(spE)-1]), strGrow, areaSize))
		}
	}

	// configure_constraints
	for i, c := range constraints {
		seg := segE[i]
		switch c.kind {
		case ConstraintMax:
			add(cnLE(sz(seg), strMaxSizeLE, float64(c.a)*prec))
			add(cnEQ(sz(seg), strMaxSizeEQ, float64(c.a)*prec))
		case ConstraintMin:
			add(cnGE(sz(seg), strMinSizeGE, float64(c.a)*prec))
			if flex.IsLegacy() {
				add(cnEQ(sz(seg), strMinSizeEQ, float64(c.a)*prec))
			} else {
				add(cnEQExpr(sz(seg), strFillGrow, areaSize))
			}
		case ConstraintLength:
			add(cnEQ(sz(seg), strLengthSizeEQ, float64(c.a)*prec))
		case ConstraintPercentage:
			add(cnEQExpr(sz(seg), strPercentageEQ, areaSize.mul(float64(c.a)/100.0)))
		case ConstraintRatio:
			den := c.b
			if den <= 0 {
				den = 1
			}
			add(cnEQExpr(sz(seg), strRatioSizeEQ, areaSize.mul(float64(c.a)/float64(den))))
		case ConstraintFill:
			add(cnEQExpr(sz(seg), strFillGrow, areaSize))
		}
	}

	// configure_fill_constraints
	type gi struct {
		i int
		s float64
	}
	grows := make([]gi, 0, n)
	for i, c := range constraints {
		if c.IsFill() || (!flex.IsLegacy() && c.IsMin()) {
			sc := 1.0
			if c.IsFill() {
				sc = float64(c.a)
				if sc < 1e-6 {
					sc = 1e-6
				}
			}
			grows = append(grows, gi{i: i, s: sc})
		}
	}
	for a := 0; a < len(grows); a++ {
		for b := a + 1; b < len(grows); b++ {
			// right_scale * left_size == left_scale * right_size
			li, ri := grows[a], grows[b]
			lhs := sz(segE[li.i]).mul(ri.s)
			rhs := sz(segE[ri.i]).mul(li.s)
			add(cnEQExpr(lhs, strGrow, rhs))
		}
	}

	// ALL_SEGMENT_GROW for non-legacy
	if !flex.IsLegacy() {
		for i := 0; i+1 < n; i++ {
			add(cnEQExpr(sz(segE[i]), strAllSegmentGrow, sz(segE[i+1])))
		}
	}

	// changes_to_rects rounding
	roundPos := func(v float64) int {
		return int(math.Round(math.Round(v) / prec))
	}
	val := func(v cVar) float64 { return solver.getValue(v) }

	for i := 0; i < n; i++ {
		st := roundPos(val(segE[i].start))
		en := roundPos(val(segE[i].end))
		if en < st {
			sizes[i] = 0
		} else {
			sizes[i] = en - st
		}
	}
	// Signed spacer deltas so SplitWithSpacers can place overlapping segments.
	// Spacer rect widths still use max(0, spacer) when materializing rects.
	for i := 0; i < n+1; i++ {
		st := roundPos(val(spE[i].start))
		en := roundPos(val(spE[i].end))
		spacers[i] = en - st
	}
	return sizes, spacers
}
