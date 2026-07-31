package ledger

// State is the append-only committed-prefix ledger.
//
// Fields mirror tui.ts private engine state:
//
//	CommittedRows  — C; frame rows [0, C) are in native history
//	AuditRows      — A ≤ C; [0, A) byte-stable audited zone
//	DurableRows    — Dmark with A ≤ Dmark ≤ C; [A, Dmark) durable exempt zone
//	Prefix         — raw row strings mirroring [0, C); component-owned refs OK
//	WindowTop      — W; frame row mapped to screen row 0
//
// Zero value is a fresh ledger (nothing committed).
type State struct {
	CommittedRows int
	AuditRows     int
	DurableRows   int
	Prefix        []string
	WindowTop     int
}

// Reset clears the ledger to empty (session replace after ED3, or first paint prep).
func (s *State) Reset() {
	*s = State{}
}

// Snapshot returns a shallow copy of counters plus a shared Prefix header.
// Callers must not mutate Prefix without owning the State.
func (s *State) Snapshot() State {
	return State{
		CommittedRows: s.CommittedRows,
		AuditRows:     s.AuditRows,
		DurableRows:   s.DurableRows,
		Prefix:        s.Prefix,
		WindowTop:     s.WindowTop,
	}
}

// ShouldAudit reports whether this frame must run the committed-prefix audit.
//
// Skipped when neverRendered, geometryChanged, or clearScrollback (replace).
// Otherwise runs when the stable prefix does not cover every audited row, or
// when a forced-overflow row became durable/permanent this frame
// (DurableRows < min(CommittedRows, boundaries.Durable)).
//
// Mirrors tui.ts auditRan gate.
func (s *State) ShouldAudit(neverRendered, geometryChanged, clearScrollback bool, stablePrefixRows int, b Boundaries) bool {
	if neverRendered || geometryChanged || clearScrollback {
		return false
	}
	auditUpper := s.AuditRows
	if s.DurableRows < s.CommittedRows {
		auditUpper = s.CommittedRows
	}
	hardAuditEnd := min(s.CommittedRows, b.Durable)
	needHardAudit := s.DurableRows < hardAuditEnd
	return stablePrefixRows < auditUpper || needHardAudit
}

// AuditResult is the outcome of AuditPrefix.
type AuditResult struct {
	// Resynced is true when CommittedRows moved (re-anchor).
	Resynced bool
	// ResyncTo is the new commit index when Resynced; else -1.
	ResyncTo int
}

// AuditPrefix detects committed-prefix violations and re-anchors at the first
// moved audited row. The stale copy stays in history — duplication, never loss.
// Pure in-place restyles keep alignment and are left alone.
//
// permanentEnd is the frame's durableBoundary (rows that this frame asserts are
// durable/permanent). Mirrors #auditCommittedPrefix.
func (s *State) AuditPrefix(frame []string, permanentEnd int) AuditResult {
	if len(s.Prefix) == 0 {
		return AuditResult{ResyncTo: -1}
	}
	resyncTo := FindCommittedPrefixResync(
		frame,
		s.Prefix,
		len(s.Prefix),
		s.AuditRows,
		s.DurableRows,
		permanentEnd,
	)
	if resyncTo < 0 {
		return AuditResult{ResyncTo: -1}
	}
	s.CommittedRows = resyncTo
	if s.AuditRows > resyncTo {
		s.AuditRows = resyncTo
	}
	if s.DurableRows > resyncTo {
		s.DurableRows = resyncTo
	}
	// Truncate without copying the kept prefix.
	s.Prefix = s.Prefix[:resyncTo]
	return AuditResult{Resynced: true, ResyncTo: resyncTo}
}

// PlanKind classifies the render intent the plan feeds.
type PlanKind int

const (
	// PlanFullPaint clears/replays (first paint, session replace, geometry rebuild).
	PlanFullPaint PlanKind = iota
	// PlanUpdate is an ordinary incremental frame.
	PlanUpdate
)

// CommitPlan is pure window/commit math for one frame. Emitters consume it;
// ApplyPlan mutates State after a successful emit.
//
// Mirrors #doRender sections 3–4 (windowTop / chunkTo / reslice flags).
type CommitPlan struct {
	Kind PlanKind

	// WindowTop is the frame row mapped to screen row 0.
	WindowTop int
	// ChunkTo is the exclusive end of the commit chunk for this frame.
	// Full paint: equals WindowTop. Update: may equal CommittedRows (frozen)
	// or WindowTop (advance). Chunk from-state is pre-plan CommittedRows.
	ChunkTo int

	// ClearScrollback is meaningful only for PlanFullPaint. True for session
	// replace or non-mux geometry rebuild. Caller must still suppress ED3 in
	// multiplexers if it overrides.
	ClearScrollback bool

	// Resliced means the prefix storage is replaced wholesale this frame
	// (full paint / shrink / geometry re-base), not merely extended.
	Resliced bool

	// ShrinkOrCursorTail is the shrink-into-prefix, in-place viewport reveal,
	// or short focused-cursor-tail branch: CommittedRows is forced to ChunkTo
	// and prefix is re-sliced from the frame before emit.
	ShrinkOrCursorTail bool

	// GeometryRebase is the mux/in-place geometry path: prefix is re-sliced
	// to the current frame's [0, CommittedRows) at the new width without
	// advancing the commit index.
	GeometryRebase bool

	// PreCommittedRows / PreAuditRows / PreDurableRows are post-audit,
	// pre-commit marks for UpdateAuditRows after emit.
	PreCommittedRows int
	PreAuditRows     int
	PreDurableRows   int
}

// PlanInput is everything CommitPlan needs beyond the live State.
type PlanInput struct {
	FrameLen int
	Height   int

	// FirstPaint is !hasEverRendered.
	FirstPaint bool
	// ReplaceRequested is clearScrollback-on-next-render (session replace).
	ReplaceRequested bool
	// GeometryChanged is width or height change (including net-zero resize event).
	GeometryChanged bool
	// ResizeRepaintsInPlace is mux or alt-screen-toggle terminals: no ED3 rebuild.
	ResizeRepaintsInPlace bool
	// IsMultiplexer suppresses ClearScrollback on full paint.
	IsMultiplexer bool

	// HasVisibleOverlay freezes commits (chunkTo stays at CommittedRows).
	HasVisibleOverlay bool

	// CommittedRowsResynced is AuditPrefix.Resynced this frame.
	CommittedRowsResynced bool
	// HasFocusedCursorTail is true when some cursor marker sits at/after the
	// post-audit CommittedRows (used with short tail after resync).
	HasFocusedCursorTail bool
}

// PlanCommit computes windowTop/chunkTo and branch flags without mutating State.
// Call after optional AuditPrefix. ApplyPlan applies the result to State.
//
// Mirrors tui.ts #doRender window/commit math.
func (s *State) PlanCommit(in PlanInput) CommitPlan {
	frameLen := max(0, in.FrameLen)
	height := max(0, in.Height)
	if height == 0 {
		// Degenerate viewport: keep math defined (windowTop at end).
		height = 0
	}

	geometryRebuild := in.GeometryChanged && !in.ResizeRepaintsInPlace
	fullPaint := in.FirstPaint || in.ReplaceRequested || geometryRebuild

	plan := CommitPlan{
		PreCommittedRows: s.CommittedRows,
		PreAuditRows:     s.AuditRows,
		PreDurableRows:   s.DurableRows,
	}

	if fullPaint {
		plan.Kind = PlanFullPaint
		plan.Resliced = true
		plan.WindowTop = max(0, frameLen-height)
		plan.ChunkTo = plan.WindowTop
		// clearScrollback: replace or geometry rebuild, never in mux.
		if in.ReplaceRequested || geometryRebuild {
			plan.ClearScrollback = !in.IsMultiplexer
		}
		return plan
	}

	plan.Kind = PlanUpdate
	committed := s.CommittedRows

	// Shrink into committed prefix, reveal rows above the committed seam after
	// an in-place viewport expansion, or re-show a short focused cursor tail.
	// Revealed history is re-anchored (duplication, never loss); otherwise a
	// forced window rewrite would clear it and repaint only the live tail.
	targetWindowTop := max(0, frameLen-height)
	if frameLen <= committed ||
		(in.GeometryChanged && in.ResizeRepaintsInPlace && targetWindowTop < committed) ||
		(in.CommittedRowsResynced &&
			frameLen-committed < height &&
			in.HasFocusedCursorTail) {
		plan.WindowTop = targetWindowTop
		plan.ChunkTo = plan.WindowTop
		plan.Resliced = true
		plan.ShrinkOrCursorTail = true
		return plan
	}

	// Ordinary / overlay-freeze / mux-geometry path.
	plan.WindowTop = max(committed, frameLen-height, 0)
	if in.HasVisibleOverlay || in.GeometryChanged {
		plan.ChunkTo = committed
	} else {
		plan.ChunkTo = plan.WindowTop
	}
	if in.GeometryChanged {
		plan.Resliced = true
		plan.GeometryRebase = true
	}
	return plan
}

// ApplyPrefixMutation performs the pre-emit State mutations that the OMP engine
// does inline during planning (shrink re-slice, geometry prefix re-base).
// Full-paint and ordinary update prefix writes happen in ApplyAfterEmit.
//
// frame must be the raw (unprepared) frame rows.
func (s *State) ApplyPrefixMutation(frame []string, plan CommitPlan) {
	switch {
	case plan.ShrinkOrCursorTail:
		// this.#committedRows = chunkTo; this.#committedPrefix = rawFrame.slice(0, chunkTo)
		s.CommittedRows = plan.ChunkTo
		s.Prefix = clonePrefix(frame, plan.ChunkTo)
	case plan.GeometryRebase:
		// this.#committedPrefix = rawFrame.slice(0, this.#committedRows)
		s.Prefix = clonePrefix(frame, s.CommittedRows)
	}
}

// ApplyAfterEmit updates committed rows, window top, prefix storage, and audit
// marks after a successful emit. Mirrors post-emit bookkeeping in #doRender.
//
//	full paint: prefix = frame[0:chunkTo]; committed = chunkTo (via emit);
//	            updateAuditRows(resliced=true, hardAudited=false)
//	update:     extend prefix [len, chunkTo); committed = chunkTo (via emit);
//	            updateAuditRows(resliced, hardAudited=auditRan)
//
// auditRan is whether AuditPrefix ran this frame (feeds durableRows advance).
// boundaries are the current frame's ResolveBoundaries result.
func (s *State) ApplyAfterEmit(frame []string, plan CommitPlan, b Boundaries, auditRan bool) {
	chunkTo := plan.ChunkTo
	if chunkTo < 0 {
		chunkTo = 0
	}
	if chunkTo > len(frame) {
		// Defensive: never claim past the frame; prefer short commit over panic.
		chunkTo = len(frame)
	}

	s.WindowTop = plan.WindowTop
	s.CommittedRows = chunkTo

	switch plan.Kind {
	case PlanFullPaint:
		// Wholesale replace — avoid retaining a longer previous backing array
		// only when the new prefix differs in length or we already resliced.
		s.Prefix = clonePrefix(frame, chunkTo)
		s.UpdateAuditRows(true, plan.PreCommittedRows, plan.PreAuditRows, plan.PreDurableRows, b, false)
	default:
		// Extend without copying the unchanged head.
		s.extendPrefix(frame, chunkTo)
		s.UpdateAuditRows(plan.Resliced, plan.PreCommittedRows, plan.PreAuditRows, plan.PreDurableRows, b, auditRan)
	}
}

// extendPrefix appends frame[len(Prefix):chunkTo) onto Prefix, reusing capacity
// and keeping the unchanged head (no copy of prior rows).
func (s *State) extendPrefix(frame []string, chunkTo int) {
	if chunkTo < 0 {
		chunkTo = 0
	}
	if chunkTo <= len(s.Prefix) {
		// ShrinkOrCursorTail already set Prefix; or no growth.
		s.Prefix = s.Prefix[:chunkTo]
		return
	}
	from := len(s.Prefix)
	if from > len(frame) {
		from = len(frame)
		s.Prefix = s.Prefix[:from]
	}
	if chunkTo <= len(frame) {
		s.Prefix = append(s.Prefix, frame[from:chunkTo]...)
		return
	}
	// Frame shorter than claim — append what exists, pad empty (defensive).
	if from < len(frame) {
		s.Prefix = append(s.Prefix, frame[from:]...)
	}
	for len(s.Prefix) < chunkTo {
		s.Prefix = append(s.Prefix, "")
	}
}

// UpdateAuditRows recomputes AuditRows / DurableRows after a commit.
//
// auditRows tracks the byte-stable boundary; durableRows the durable snapshot
// boundary. A wholesale re-slice (full paint / shrink / geometry) re-bases each
// mark from the current frame (min(committed, boundary)). An incremental extend
// keeps a mark once a row past it has committed (mark < committed): a later RISE
// in a boundary must neither pull already-committed stale snapshots back under
// the byte-stable cap nor retroactively exempt forced-overflow rows already
// audited. durableRows is floored at auditRows so the exempt window can never
// invert.
//
// durableRows also advances when hardAudited ran this frame: the resync's full
// hard scan verified the forced suffix, so those rows are now proven durable.
//
// Mirrors #updateCommittedAuditRows.
func (s *State) UpdateAuditRows(
	resliced bool,
	preCommittedRows, preAuditRows, preDurableRows int,
	b Boundaries,
	hardAudited bool,
) {
	committed := s.CommittedRows
	var auditRows int
	if resliced || preAuditRows >= preCommittedRows {
		auditRows = min(committed, b.ByteStable)
	} else {
		auditRows = min(preAuditRows, committed)
	}
	var durableRows int
	if resliced || preDurableRows >= preCommittedRows || hardAudited {
		durableRows = min(committed, b.Durable)
	} else {
		durableRows = min(preDurableRows, committed)
	}
	s.AuditRows = auditRows
	s.DurableRows = max(auditRows, durableRows)
}

// ChunkFrom returns the inclusive start of this plan's commit chunk
// (pre-plan committed rows). For shrink/full paint the engine rewrites history
// bookkeeping rather than appending a physical chunk from an old index.
func (p CommitPlan) ChunkFrom() int {
	if p.Kind == PlanFullPaint || p.ShrinkOrCursorTail {
		return 0
	}
	return p.PreCommittedRows
}

// ChunkLen is max(0, ChunkTo - ChunkFrom) for ordinary update append accounting.
func (p CommitPlan) ChunkLen() int {
	from := p.ChunkFrom()
	if p.ChunkTo <= from {
		return 0
	}
	return p.ChunkTo - from
}

// clonePrefix returns a new slice header of frame[0:n], sharing string headers
// (strings are immutable). n is clamped to [0, len(frame)].
func clonePrefix(frame []string, n int) []string {
	if n <= 0 {
		return nil
	}
	if n > len(frame) {
		n = len(frame)
	}
	out := make([]string, n)
	copy(out, frame[:n])
	return out
}
