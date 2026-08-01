package component

import (
	"sync/atomic"

	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// Container is a retained component that vertically stacks children.
//
// Children are rendered every frame (renders may carry side effects: seam and
// stability reports, image registration). Concatenation is memoized by child
// identity, per-child Generation, and width — when all match the previous
// compose, Render returns the exact same Lines slice and Generation.
//
// Stable leading child frames avoid rebuilding their portion of the
// concatenated slice when a later sibling changes: the memoized prefix is
// reused and only the dirty tail is appended.
//
// Container itself implements Component, Invalidator, Disposable,
// TightLayoutAware, StablePrefix, CommittedRowsAware, ViewportTailProvider,
// and InputHandler (input routes to the focus target).
type Container struct {
	children []Component

	// Composition memo. Valid when memoValid is true.
	memoValid      bool
	memoWidth      int
	memoLines      []string
	memoGen        uint64
	memoChildGen   []uint64
	memoChildIdent []Component
	memoChildN     int
	memoFrame      Frame

	// Stable-prefix report for StablePrefix (consumed on read).
	reportedStable int

	// committedRows is the engine claim from the previous frame (local rows).
	// Draw completion may update it from the scheduler goroutine.
	committedRows atomic.Int64

	ignoreTight bool

	// focusTarget receives RouteInput when the container handles input.
	focusTarget Component

	// gen counts container composition changes (not child-list only).
	gen Gen
}

// NewContainer returns an empty container, optionally pre-populated.
func NewContainer(children ...Component) *Container {
	c := &Container{}
	if len(children) > 0 {
		c.children = make([]Component, len(children))
		copy(c.children, children)
	}
	return c
}

// Children returns a snapshot of the child list. Callers must not retain the
// slice across mutations; the slice itself is a copy.
func (c *Container) Children() []Component {
	if c == nil || len(c.children) == 0 {
		return nil
	}
	out := make([]Component, len(c.children))
	copy(out, c.children)
	return out
}

// Len returns the number of direct children.
func (c *Container) Len() int {
	if c == nil {
		return 0
	}
	return len(c.children)
}

// Child returns the i-th direct child, or nil if out of range.
func (c *Container) Child(i int) Component {
	if c == nil || i < 0 || i >= len(c.children) {
		return nil
	}
	return c.children[i]
}

// AddChild appends component. Nil is ignored. Invalidates composition memo.
func (c *Container) AddChild(component Component) {
	if c == nil || component == nil {
		return
	}
	if c.ignoreTight {
		if t, ok := component.(TightLayoutAware); ok {
			t.SetIgnoreTight(true)
		}
	}
	c.children = append(c.children, component)
	c.invalidateMemo()
}

// InsertChild inserts component at index i (clamped to [0, Len]).
// Nil is ignored. Invalidates composition memo.
func (c *Container) InsertChild(i int, component Component) {
	if c == nil || component == nil {
		return
	}
	if i < 0 {
		i = 0
	}
	if i > len(c.children) {
		i = len(c.children)
	}
	if c.ignoreTight {
		if t, ok := component.(TightLayoutAware); ok {
			t.SetIgnoreTight(true)
		}
	}
	c.children = append(c.children, nil)
	copy(c.children[i+1:], c.children[i:])
	c.children[i] = component
	c.invalidateMemo()
}

// RemoveChild removes the first identity match of component.
// Returns true when a child was removed. Does not Dispose the child.
func (c *Container) RemoveChild(component Component) bool {
	if c == nil || component == nil {
		return false
	}
	for i, ch := range c.children {
		if ch == component {
			copy(c.children[i:], c.children[i+1:])
			c.children[len(c.children)-1] = nil
			c.children = c.children[:len(c.children)-1]
			if c.focusTarget == component {
				c.focusTarget = nil
			}
			c.invalidateMemo()
			return true
		}
	}
	return false
}

// Clear removes all children without disposing them. Use Dispose when the
// children are permanently discarded.
func (c *Container) Clear() {
	if c == nil {
		return
	}
	for i := range c.children {
		c.children[i] = nil
	}
	c.children = c.children[:0]
	c.focusTarget = nil
	c.invalidateMemo()
}

// Contains reports whether target is this container or appears in its subtree
// via nested Container children (identity search).
func (c *Container) Contains(target Component) bool {
	if c == nil {
		return false
	}
	return SubtreeContains(c, target)
}

// FindChild returns the direct child whose subtree contains target (identity),
// or nil. Useful for component-scoped partial compose at a root container.
func (c *Container) FindChild(target Component) Component {
	if c == nil || target == nil {
		return nil
	}
	for _, ch := range c.children {
		if SubtreeContains(ch, target) {
			return ch
		}
	}
	return nil
}

// SetFocusTarget selects the child (or descendant) that HandleInput routes to.
// Pass nil to clear. The target is not required to be a direct child.
func (c *Container) SetFocusTarget(component Component) {
	if c == nil {
		return
	}
	c.focusTarget = component
}

// FocusTarget returns the current input routing target.
func (c *Container) FocusTarget() Component {
	if c == nil {
		return nil
	}
	return c.focusTarget
}

// HandleInput routes ev to the focus target via [RouteInput].
func (c *Container) HandleInput(ev event.Event) {
	if c == nil {
		return
	}
	RouteInput(c.focusTarget, ev)
}

// SetIgnoreTight propagates the tight-layout flag to children and invalidates.
func (c *Container) SetIgnoreTight(ignore bool) {
	if c == nil {
		return
	}
	c.ignoreTight = ignore
	for _, ch := range c.children {
		if t, ok := ch.(TightLayoutAware); ok {
			t.SetIgnoreTight(ignore)
		}
	}
	c.Invalidate()
}

// IgnoreTight reports the container's tight-layout flag.
func (c *Container) IgnoreTight() bool {
	if c == nil {
		return false
	}
	return c.ignoreTight
}

// SetNativeScrollbackCommittedRows records the engine's committed-row claim
// for this container. Propagated to children with local offsets during Render.
func (c *Container) SetNativeScrollbackCommittedRows(rows int) {
	if c == nil {
		return
	}
	if rows < 0 {
		rows = 0
	}
	c.committedRows.Store(int64(rows))
}

// Invalidate drops the composition memo and invalidates every child.
func (c *Container) Invalidate() {
	if c == nil {
		return
	}
	c.invalidateMemo()
	for _, ch := range c.children {
		InvalidateOne(ch)
	}
}

// Dispose propagates teardown to children. Idempotent per child via each
// child's own Dispose. Does not clear the child list (call Clear after when
// discarding the structure).
func (c *Container) Dispose() {
	if c == nil {
		return
	}
	for _, ch := range c.children {
		DisposeOne(ch)
	}
}

// RenderStablePrefixRows implements StablePrefix. Reading consumes the report
// and re-bases the baseline to the current memoized lines.
func (c *Container) RenderStablePrefixRows() int {
	if c == nil {
		return 0
	}
	n := c.reportedStable
	if c.memoValid {
		c.reportedStable = len(c.memoLines)
	} else {
		c.reportedStable = 0
	}
	return n
}

// RenderViewportTail implements ViewportTailProvider by rendering children and
// keeping only the bottom maxRows of the composed result. Full-compose memo
// state is not written (settle paint must not see tail side effects on memo).
func (c *Container) RenderViewportTail(width, maxRows int) []string {
	if c == nil || maxRows <= 0 {
		return emptyLines
	}
	if width < 1 {
		width = 1
	}
	// Compose without updating memo / generation / stable-prefix report.
	total := 0
	frames := make([]Frame, len(c.children))
	for i, ch := range c.children {
		if ch == nil {
			frames[i] = EmptyFrame(0)
			continue
		}
		frames[i] = ch.Render(width)
		total += len(frames[i].Lines)
	}
	if total == 0 {
		return emptyLines
	}
	if maxRows > total {
		maxRows = total
	}
	skip := total - maxRows
	out := make([]string, 0, maxRows)
	seen := 0
	for _, fr := range frames {
		n := len(fr.Lines)
		if seen+n <= skip {
			seen += n
			continue
		}
		start := 0
		if seen < skip {
			start = skip - seen
		}
		out = append(out, fr.Lines[start:]...)
		seen += n
	}
	return out
}

// Render implements Component. width is clamped to at least 1.
func (c *Container) Render(width int) Frame {
	if c == nil {
		return EmptyFrame(0)
	}
	if width < 1 {
		width = 1
	}

	children := c.children
	count := len(children)

	// Ensure memo scratch matches count.
	if cap(c.memoChildGen) < count {
		c.memoChildGen = make([]uint64, count)
	} else {
		c.memoChildGen = c.memoChildGen[:count]
	}
	if cap(c.memoChildIdent) < count {
		c.memoChildIdent = make([]Component, count)
	} else {
		c.memoChildIdent = c.memoChildIdent[:count]
	}

	type childResult struct {
		frame    Frame
		lines    []string
		gen      uint64
		sameSlot bool // same identity + generation as previous memo at this index
	}
	results := make([]childResult, count)

	prevValid := c.memoValid && c.memoWidth == width && c.memoChildN == count
	// Confirm every slot still holds the same child identity before trusting gens.
	if prevValid {
		for i := 0; i < count; i++ {
			if c.memoChildIdent[i] != children[i] {
				prevValid = false
				break
			}
		}
	}

	totalRows := 0
	committedRows := int(c.committedRows.Load())
	offset := 0
	allSame := prevValid

	// First pass: render children, feed committed-row claims, collect frames.
	for i := 0; i < count; i++ {
		ch := children[i]
		if ch == nil {
			same := prevValid && c.memoChildIdent[i] == nil && c.memoChildGen[i] == 0
			results[i] = childResult{
				frame:    EmptyFrame(0),
				lines:    emptyLines,
				gen:      0,
				sameSlot: same,
			}
			if !same {
				allSame = false
			}
			continue
		}
		// Propagate remaining committed rows before render (OMP order).
		NotifyCommittedRows(ch, committedRows-offset)

		fr := ch.Render(width)
		lines := fr.Lines
		if lines == nil {
			lines = emptyLines
			fr.Lines = lines
		}
		same := prevValid && c.memoChildIdent[i] == ch && c.memoChildGen[i] == fr.Generation
		if !same {
			allSame = false
		}
		results[i] = childResult{frame: fr, lines: lines, gen: fr.Generation, sameSlot: same}
		totalRows += len(lines)
		offset += len(lines)
	}

	// Memo hit: every child identity+generation matches at this width and count.
	if allSame && c.memoValid {
		// Still consume StablePrefix reports so baselines advance.
		stable := 0
		chain := true
		for i := 0; i < count; i++ {
			ch := children[i]
			var reported int
			var hasReport bool
			if ch != nil {
				reported, hasReport = StablePrefixRows(ch)
			}
			n := len(results[i].lines)
			if chain {
				if hasReport {
					sc := reported
					if sc < 0 {
						sc = 0
					}
					if sc > n {
						sc = n
					}
					stable += sc
					if sc < n {
						chain = false
					}
				} else {
					stable += n
				}
			}
		}
		c.reportedStable = stable
		return c.memoFrame
	}

	// Compute stable prefix row count from leading fully-stable children and
	// in-place StablePrefix reports (for dirty children).
	stableRows := 0
	chainStable := prevValid
	if !c.memoValid {
		chainStable = false
	}
	for i := 0; i < count; i++ {
		ch := children[i]
		n := len(results[i].lines)
		var reported int
		var hasReport bool
		if ch != nil {
			reported, hasReport = StablePrefixRows(ch)
		}
		if !chainStable {
			continue
		}
		if results[i].sameSlot && !hasReport {
			// Identity+generation match and no in-place mutator: full child stable.
			stableRows += n
			continue
		}
		if hasReport {
			sc := reported
			if sc < 0 {
				sc = 0
			}
			if sc > n {
				sc = n
			}
			// In-place mutator report only applies when the slot still holds the
			// same component identity (otherwise the rows are a different child).
			if results[i].sameSlot || (prevValid && c.memoChildIdent[i] == ch) {
				stableRows += sc
			}
			if sc < n || !results[i].sameSlot {
				chainStable = false
			}
			continue
		}
		chainStable = false
	}
	if stableRows > totalRows {
		stableRows = totalRows
	}
	if c.memoValid && stableRows > len(c.memoLines) {
		stableRows = len(c.memoLines)
	}
	if !c.memoValid {
		stableRows = 0
	}

	// Build concatenated lines. Reuse memo prefix when composition only dirties a tail.
	var lines []string
	if totalRows == 0 {
		lines = emptyLines
	} else if stableRows > 0 && stableRows == totalRows && c.memoValid && len(c.memoLines) == totalRows {
		// Entire frame row-stable; keep previous slice header (no copy).
		lines = c.memoLines
	} else if stableRows > 0 && c.memoValid {
		lines = make([]string, 0, totalRows)
		lines = append(lines, c.memoLines[:stableRows]...)
		// Append from first row after stableRows across children.
		seen := 0
		for i := 0; i < count; i++ {
			cl := results[i].lines
			n := len(cl)
			if seen+n <= stableRows {
				seen += n
				continue
			}
			start := 0
			if seen < stableRows {
				start = stableRows - seen
			}
			lines = append(lines, cl[start:]...)
			seen += n
		}
	} else {
		lines = make([]string, 0, totalRows)
		for i := 0; i < count; i++ {
			cl := results[i].lines
			if len(cl) == 0 {
				continue
			}
			lines = append(lines, cl...)
		}
		if lines == nil {
			lines = emptyLines
		}
	}

	// Aggregate seams and cursor: topmost live seam wins; last cursor wins.
	liveStart := BoundaryUnset
	commitEnd := BoundaryUnset
	snapshotEnd := BoundaryUnset
	var cursor *Cursor
	rowOffset := 0
	for i := 0; i < count; i++ {
		fr := results[i].frame
		n := len(results[i].lines)
		if liveStart == BoundaryUnset {
			if live, commit, snap, ok := fr.NormalizedSeams(); ok {
				liveStart = rowOffset + live
				commitEnd = rowOffset + commit
				snapshotEnd = rowOffset + snap
			}
		}
		if fr.Cursor != nil {
			cc := *fr.Cursor
			cc.Row += rowOffset
			cursor = &cc
		}
		rowOffset += n
	}

	// When the composed result is identical to the previous memo, keep the
	// previous generation so callers see reference+gen stability.
	var gen uint64
	reuseGen := c.memoValid &&
		liveStart == c.memoFrame.LiveRegionStart &&
		commitEnd == c.memoFrame.CommitSafeEnd &&
		snapshotEnd == c.memoFrame.SnapshotSafeEnd &&
		cursorEqual(cursor, c.memoFrame.Cursor) &&
		sameStringSlice(lines, c.memoLines)
	if reuseGen {
		// Prefer exact slice header reuse when possible.
		if len(lines) == 0 {
			lines = emptyLines
		} else if len(c.memoLines) > 0 && &lines[0] == &c.memoLines[0] {
			lines = c.memoLines
		}
		gen = c.memoGen
	} else {
		gen = c.gen.Next()
	}

	frame := Frame{
		Lines:           lines,
		Generation:      gen,
		Cursor:          cursor,
		LiveRegionStart: liveStart,
		CommitSafeEnd:   commitEnd,
		SnapshotSafeEnd: snapshotEnd,
	}

	// Commit memo.
	c.memoValid = true
	c.memoWidth = width
	c.memoLines = lines
	c.memoGen = gen
	c.memoChildN = count
	for i := 0; i < count; i++ {
		c.memoChildGen[i] = results[i].gen
		c.memoChildIdent[i] = children[i]
	}
	c.memoFrame = frame
	c.reportedStable = stableRows

	return frame
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	// Fast path: identical backing (same header pointer via first element).
	if &a[0] == &b[0] {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cursorEqual(a, b *Cursor) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Row == b.Row && a.Column == b.Column
}

func (c *Container) invalidateMemo() {
	c.memoValid = false
	c.memoWidth = -1
	c.memoLines = nil
	c.memoChildN = 0
	c.memoFrame = Frame{}
	c.reportedStable = 0
	// Drop identity refs so removed children can be GC'd even if the
	// container lingers with a cleared child list.
	for i := range c.memoChildIdent {
		c.memoChildIdent[i] = nil
	}
}

// SubtreeContains reports whether target == root or target appears under root
// through nested [Container] children (identity comparison).
func SubtreeContains(root, target Component) bool {
	if root == nil || target == nil {
		return false
	}
	if root == target {
		return true
	}
	switch c := root.(type) {
	case *Container:
		for _, ch := range c.children {
			if SubtreeContains(ch, target) {
				return true
			}
		}
	case interface{ Children() []Component }:
		// Allow foreign container-shaped nodes that expose Children().
		for _, ch := range c.Children() {
			if SubtreeContains(ch, target) {
				return true
			}
		}
	}
	return false
}

// FindRootChild returns the direct child of root whose subtree contains target.
// When root is not a *Container, returns target if root == target, else nil.
func FindRootChild(root, target Component) Component {
	if root == nil || target == nil {
		return nil
	}
	if c, ok := root.(*Container); ok {
		return c.FindChild(target)
	}
	if root == target {
		return root
	}
	if SubtreeContains(root, target) {
		return root
	}
	return nil
}
