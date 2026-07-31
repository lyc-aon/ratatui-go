package ledger

// FrameInput is one composed frame's ledger inputs (post-compose, pre-emit).
//
// Seam fields use Unset (-1) when the component omits them. A real seam at row
// 0 is LiveStart == 0. Prefer NewFrameInput to get Unset defaults (shell
// semantics: whatever scrolls is final).
type FrameInput struct {
	// Frame is the raw (unprepared) composed row slice. Ledger stores
	// references into it; callers must treat committed prefix rows as immutable.
	Frame []string

	// Height is the terminal viewport row count.
	Height int

	// Seams from NativeScrollbackLiveRegion (absolute frame indices).
	// Unset (-1) = omit. Zero is a valid live seam at row 0.
	LiveStart    int
	CommitSafe   int
	SnapshotSafe int

	// FirstPaint is !hasEverRendered.
	FirstPaint bool
	// ReplaceRequested is clearScrollback-on-next-render.
	ReplaceRequested bool
	// GeometryChanged is width/height change (including net-zero resize event).
	GeometryChanged bool
	// ResizeRepaintsInPlace is mux or alt-screen-toggle terminals.
	ResizeRepaintsInPlace bool
	// IsMultiplexer suppresses ClearScrollback on full paint.
	IsMultiplexer bool
	// HasVisibleOverlay freezes commits.
	HasVisibleOverlay bool

	// StablePrefixRows is the compose-reported leading unchanged row count.
	// Gates the audit when it covers every audited row.
	StablePrefixRows int

	// CursorRows are absolute frame row indices of hardware cursor markers
	// (post strip). Used only for the short focused-cursor-tail resync branch.
	CursorRows []int
}

// NewFrameInput returns a FrameInput with shell-semantic seams (all Unset).
func NewFrameInput(frame []string, height int) FrameInput {
	return FrameInput{
		Frame:        frame,
		Height:       height,
		LiveStart:    Unset,
		CommitSafe:   Unset,
		SnapshotSafe: Unset,
	}
}

// FrameResult is the pure ledger decision for one frame, ready for emitters.
type FrameResult struct {
	Boundaries Boundaries
	Audit      AuditResult
	// AuditRan is whether the audit detector executed (feeds durable advance).
	AuditRan bool
	Plan     CommitPlan
}

// Step runs the full pure ledger pipeline for one frame up through planning and
// pre-emit prefix mutation:
//
//  1. ResolveBoundaries
//  2. ShouldAudit → AuditPrefix (duplication-never-loss re-anchor)
//  3. PlanCommit (full / shrink / overlay freeze / mux geometry / ordinary)
//  4. ApplyPrefixMutation (shrink re-slice / geometry prefix re-base)
//
// After the caller successfully emits, it MUST call Finish(frame, result) to
// advance CommittedRows, WindowTop, Prefix, and audit marks.
//
// Deterministic over []string. Never loses content: audit prefers duplication.
func (s *State) Step(in FrameInput) FrameResult {
	frame := in.Frame
	if frame == nil {
		frame = []string{}
	}
	frameLen := len(frame)

	// Callers that mean "no seam" MUST pass Unset (-1). Zero is a valid seam at
	// row 0. ResolveBoundaries treats Unset as omitted.
	b := ResolveBoundaries(frameLen, in.LiveStart, in.CommitSafe, in.SnapshotSafe)

	auditRan := s.ShouldAudit(in.FirstPaint, in.GeometryChanged, in.ReplaceRequested, in.StablePrefixRows, b)

	var audit AuditResult
	if auditRan {
		audit = s.AuditPrefix(frame, b.Durable)
	} else {
		audit.ResyncTo = -1
	}

	hasFocusTail := false
	if audit.Resynced {
		for _, row := range in.CursorRows {
			if row >= s.CommittedRows {
				hasFocusTail = true
				break
			}
		}
	}

	plan := s.PlanCommit(PlanInput{
		FrameLen:              frameLen,
		Height:                in.Height,
		FirstPaint:            in.FirstPaint,
		ReplaceRequested:      in.ReplaceRequested,
		GeometryChanged:       in.GeometryChanged,
		ResizeRepaintsInPlace: in.ResizeRepaintsInPlace,
		IsMultiplexer:         in.IsMultiplexer,
		HasVisibleOverlay:     in.HasVisibleOverlay,
		CommittedRowsResynced: audit.Resynced,
		HasFocusedCursorTail:  hasFocusTail,
	})

	s.ApplyPrefixMutation(frame, plan)

	return FrameResult{
		Boundaries: b,
		Audit:      audit,
		AuditRan:   auditRan,
		Plan:       plan,
	}
}

// Finish applies post-emit bookkeeping. Call exactly once after a successful
// emit driven by the matching Step result. frame must be the same raw frame
// passed to Step.
func (s *State) Finish(frame []string, res FrameResult) {
	if frame == nil {
		frame = []string{}
	}
	s.ApplyAfterEmit(frame, res.Plan, res.Boundaries, res.AuditRan)
}

// NewState returns an empty ledger.
func NewState() *State {
	return &State{}
}
