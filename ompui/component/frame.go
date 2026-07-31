package component

import "github.com/lyc-aon/ratatui-go/ompui/ansitext"

// BoundaryUnset marks an optional native-scrollback seam as absent.
// Zero is a valid line index, so seams cannot use 0 as "missing".
const BoundaryUnset = -1

// CursorMarker is the zero-width APC components embed at the hardware cursor
// position when focused. The renderer strips it and places the real cursor.
// Identical to ansitext.CursorMarker / OMP CURSOR_MARKER.
const CursorMarker = ansitext.CursorMarker

// Cursor is an optional hardware-cursor position inside a Frame.
//
// Row is a frame-local line index. Column is the visible cell column where the
// cursor should appear (after marker strip / width measurement by the
// renderer). Components may instead embed [CursorMarker] in a row string;
// both forms are valid.
type Cursor struct {
	Row    int
	Column int
}

// Frame is an immutable line-oriented render result.
//
// Lines is owned by the producing component. Callers MUST NOT mutate the slice
// or its elements. An unchanged component may return the exact same Lines
// reference and Generation; a content change MUST use a new Generation.
//
// LiveRegionStart, CommitSafeEnd, and SnapshotSafeEnd are component-local
// line indices into Lines. Use [BoundaryUnset] when a boundary does not apply.
// When LiveRegionStart is unset, the other two should also be unset. When set,
// valid relationships are:
//
//	0 <= LiveRegionStart <= CommitSafeEnd <= SnapshotSafeEnd <= len(Lines)
//
// Missing CommitSafeEnd defaults to LiveRegionStart for consumers; missing
// SnapshotSafeEnd defaults to CommitSafeEnd (or LiveRegionStart).
//
// The zero Frame value is not seam-safe: LiveRegionStart == 0 looks like a
// live region at row 0. Always construct via [NewFrame], [EmptyFrame], or an
// explicit WithSeams / field set that uses [BoundaryUnset] for absent seams.
type Frame struct {
	// Lines are physical terminal rows at the render width. Never mutate.
	Lines []string

	// Generation is a monotonic content version for this component.
	// Unchanged content keeps the previous value; any content change bumps it.
	Generation uint64

	// Cursor is optional. Nil means no hardware cursor from this frame.
	Cursor *Cursor

	// LiveRegionStart is the first live/mutating row, or BoundaryUnset.
	LiveRegionStart int

	// CommitSafeEnd is the end (exclusive) of the byte-stable live prefix,
	// or BoundaryUnset.
	CommitSafeEnd int

	// SnapshotSafeEnd is the end (exclusive) of the durable snapshot region,
	// or BoundaryUnset.
	SnapshotSafeEnd int
}

// emptyLines is the shared zero-length row slice. Empty and zero-width renders
// reuse this so callers can rely on non-nil Lines when using constructors.
var emptyLines = []string{}

// EmptyFrame returns a frame with no rows and the given generation.
// Boundaries are unset. Lines is a non-nil empty slice.
func EmptyFrame(generation uint64) Frame {
	return Frame{
		Lines:           emptyLines,
		Generation:      generation,
		LiveRegionStart: BoundaryUnset,
		CommitSafeEnd:   BoundaryUnset,
		SnapshotSafeEnd: BoundaryUnset,
	}
}

// NewFrame builds a frame from rows and generation.
//
// A nil lines argument becomes a non-nil empty slice. The slice header is kept
// as-is (not copied); the caller must not mutate it after the call. Boundaries
// start unset.
func NewFrame(lines []string, generation uint64) Frame {
	if lines == nil {
		lines = emptyLines
	}
	return Frame{
		Lines:           lines,
		Generation:      generation,
		LiveRegionStart: BoundaryUnset,
		CommitSafeEnd:   BoundaryUnset,
		SnapshotSafeEnd: BoundaryUnset,
	}
}

// CloneLines returns a shallow copy of lines (new slice header, same strings).
// Nil input yields a non-nil empty slice.
func CloneLines(lines []string) []string {
	if len(lines) == 0 {
		return emptyLines
	}
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

// RowCount returns len(f.Lines). A nil Lines is treated as empty.
func (f Frame) RowCount() int {
	return len(f.Lines)
}

// IsEmpty reports whether the frame has no rows.
func (f Frame) IsEmpty() bool {
	return len(f.Lines) == 0
}

// HasLiveRegion reports whether LiveRegionStart is set.
func (f Frame) HasLiveRegion() bool {
	return f.LiveRegionStart != BoundaryUnset
}

// WithLines returns a copy of f with replacement lines (slice kept as-is).
func (f Frame) WithLines(lines []string) Frame {
	if lines == nil {
		lines = emptyLines
	}
	f.Lines = lines
	return f
}

// WithGeneration returns a copy of f with the given generation.
func (f Frame) WithGeneration(generation uint64) Frame {
	f.Generation = generation
	return f
}

// WithCursor returns a copy of f with cursor set. Pass nil to clear.
func (f Frame) WithCursor(cursor *Cursor) Frame {
	f.Cursor = cursor
	return f
}

// WithCursorAt returns a copy of f with a cursor at (row, column).
func (f Frame) WithCursorAt(row, column int) Frame {
	c := Cursor{Row: row, Column: column}
	f.Cursor = &c
	return f
}

// WithLiveRegion returns a copy of f with LiveRegionStart set.
// Pass BoundaryUnset to clear the live seam and both deeper ends.
func (f Frame) WithLiveRegion(start int) Frame {
	if start == BoundaryUnset {
		f.LiveRegionStart = BoundaryUnset
		f.CommitSafeEnd = BoundaryUnset
		f.SnapshotSafeEnd = BoundaryUnset
		return f
	}
	f.LiveRegionStart = start
	return f
}

// WithCommitSafeEnd returns a copy of f with CommitSafeEnd set.
func (f Frame) WithCommitSafeEnd(end int) Frame {
	f.CommitSafeEnd = end
	return f
}

// WithSnapshotSafeEnd returns a copy of f with SnapshotSafeEnd set.
func (f Frame) WithSnapshotSafeEnd(end int) Frame {
	f.SnapshotSafeEnd = end
	return f
}

// WithSeams returns a copy of f with all three seam indices set.
func (f Frame) WithSeams(liveStart, commitEnd, snapshotEnd int) Frame {
	f.LiveRegionStart = liveStart
	f.CommitSafeEnd = commitEnd
	f.SnapshotSafeEnd = snapshotEnd
	return f
}

// NormalizedSeams returns live/commit/snapshot ends clamped to [0, rows] with
// defaults applied. ok is false when no live region is set; the returned
// values are then zero.
//
// Defaults match OMP: commit defaults to live; snapshot defaults to commit.
func (f Frame) NormalizedSeams() (live, commit, snapshot int, ok bool) {
	rows := len(f.Lines)
	if f.LiveRegionStart == BoundaryUnset {
		return 0, 0, 0, false
	}
	live = clampIndex(f.LiveRegionStart, 0, rows)
	if f.CommitSafeEnd == BoundaryUnset {
		commit = live
	} else {
		commit = clampIndex(f.CommitSafeEnd, live, rows)
	}
	if f.SnapshotSafeEnd == BoundaryUnset {
		snapshot = commit
	} else {
		snapshot = clampIndex(f.SnapshotSafeEnd, commit, rows)
	}
	return live, commit, snapshot, true
}

// TranslateSeams returns a copy of f with seam indices shifted by offset
// (used when concatenating child frames). Unset seams stay unset. Cursor row
// is also shifted when present.
func (f Frame) TranslateSeams(offset int) Frame {
	if offset == 0 {
		return f
	}
	if f.LiveRegionStart != BoundaryUnset {
		f.LiveRegionStart += offset
	}
	if f.CommitSafeEnd != BoundaryUnset {
		f.CommitSafeEnd += offset
	}
	if f.SnapshotSafeEnd != BoundaryUnset {
		f.SnapshotSafeEnd += offset
	}
	if f.Cursor != nil {
		c := *f.Cursor
		c.Row += offset
		f.Cursor = &c
	}
	return f
}

func clampIndex(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Gen is a small monotonic generation counter for leaf components.
type Gen struct {
	n uint64
}

// Current returns the last issued generation (0 before the first Next).
func (g *Gen) Current() uint64 { return g.n }

// Next increments and returns the new generation.
func (g *Gen) Next() uint64 {
	g.n++
	return g.n
}

// Touch increments when condition is true and returns the current generation.
// When condition is false the generation is left unchanged.
func (g *Gen) Touch(changed bool) uint64 {
	if changed {
		return g.Next()
	}
	return g.n
}
