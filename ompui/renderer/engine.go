package renderer

import (
	"fmt"
	"io"
	"sync"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/ledger"
)

// Engine is the normal-screen append-only scrollback renderer.
//
// Not safe for concurrent Draw; use Scheduler to serialize requests.
// Terminal open/raw/restore lifecycle stays outside this type.
type Engine struct {
	out  io.Writer
	caps Caps
	tr   TraceWriter

	mu sync.Mutex // optional external safety; Scheduler holds this across Draw

	ledger ledger.State
	prep   prepareCache

	// Prior-frame / window caches (post-composite, prepared).
	previousWindow   []string
	previousFrameLen int
	previousWidth    int
	previousHeight   int

	hwCursor hardwareCursorState

	hasEverRendered      bool
	forceViewportRepaint bool
	desync               bool // true after a failed emit; next draw must full-paint

	// Per-frame emit bookkeeping (set inside Draw, read by emitUpdate).
	framePreCommitted int // post-audit, pre-plan committed rows for chunkFrom

	// Fullscreen alt-screen overlay state. Ledger/window caches stay frozen.
	altActive        bool
	altPreviousLines []string
	altEnterWidth    int
	altEnterHeight   int

	// Non-mux resize-drag alt borrow (throwaway viewport paints).
	resizeAltActive bool

	writeBuf byteBuf

	// scratch for marker extraction on a mutable copy of raw rows when needed.
	rawScratch []string
}

// New builds an Engine writing ANSI to out. out must be non-nil.
// Caps may be zero (no sync, no images, show cursor off).
func New(out io.Writer, caps Caps) *Engine {
	if out == nil {
		out = io.Discard
	}
	return &Engine{
		out:               out,
		caps:              caps,
		framePreCommitted: -1,
	}
}

// SetTrace installs a deterministic trace backend (nil disables).
func (e *Engine) SetTrace(t TraceWriter) { e.tr = t }

// SetCaps replaces capability decisions (e.g. after a DECRQM probe).
func (e *Engine) SetCaps(c Caps) { e.caps = c }

// Caps returns the current capability snapshot.
func (e *Engine) Caps() Caps { return e.caps }

// SetWriter replaces the output writer (tests / reconnect). Does not reset state.
func (e *Engine) SetWriter(w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	e.out = w
}

// CommittedRows is the ledger's C — rows [0,C) are in native history.
func (e *Engine) CommittedRows() int { return e.ledger.CommittedRows }

// WindowTop is the frame row mapped to screen row 0.
func (e *Engine) WindowTop() int { return e.ledger.WindowTop }

// LedgerSnapshot returns a shallow copy of ledger counters + prefix header.
func (e *Engine) LedgerSnapshot() ledger.State { return e.ledger.Snapshot() }

// Desynchronized reports whether the last emit failed and ordinary diffs are
// poisoned until a successful full-paint reason.
func (e *Engine) Desynchronized() bool { return e.desync }

// HasRendered is true after at least one successful normal-screen paint.
func (e *Engine) HasRendered() bool { return e.hasEverRendered }

// InvalidatePrepared lowers the prepared-line cache floor to stablePrefixRows
// (call after compose reports a stable prefix splice).
func (e *Engine) InvalidatePrepared(stablePrefixRows int) {
	e.prep.InvalidateTo(stablePrefixRows)
}

// ResetScrollback clears ledger and paint caches after an external ED3 or
// session teardown. Does not write to the terminal.
func (e *Engine) ResetScrollback() {
	e.ledger.Reset()
	e.prep.Reset()
	e.previousWindow = e.previousWindow[:0]
	e.previousFrameLen = 0
	e.previousWidth = 0
	e.previousHeight = 0
	e.hasEverRendered = false
	e.forceViewportRepaint = false
	e.desync = false
	e.forgetHardwareCursorState()
	e.altActive = false
	e.altPreviousLines = nil
	e.resizeAltActive = false
}

func (e *Engine) trace(ev TraceEvent) {
	if e.tr != nil {
		e.tr.Trace(ev)
	}
}

// Draw renders one frame synchronously. On error the ledger is rolled back to
// its pre-Step snapshot so the caller can retry; window caches are only
// advanced after a successful write.
func (e *Engine) Draw(req Request) error {
	width := req.Width
	height := req.Height
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	// Degenerate geometry: nothing to paint, keep state.
	if width == 0 || height == 0 {
		return nil
	}

	isImage := func(s string) bool { return e.caps.IsImageLine(s) }

	// --- fullscreen alt-screen short-circuit ---
	wantAlt := wantsAltScreen(req.Overlays, width, height)
	if wantAlt && !e.altActive {
		if _, err := ioWriteString(e.out, altScreenEnter+mouseTrackingOn); err != nil {
			return err
		}
		e.forgetHardwareCursorState()
		e.recordHardwareCursorHidden()
		e.altActive = true
		e.altPreviousLines = nil
		e.altEnterWidth = width
		e.altEnterHeight = height
	} else if !wantAlt && e.altActive {
		if _, err := ioWriteString(e.out, mouseTrackingOff+altScreenExit); err != nil {
			return err
		}
		e.forgetHardwareCursorState()
		e.altActive = false
		e.altPreviousLines = nil
		if width != e.altEnterWidth || height != e.altEnterHeight {
			// Geometry changed on alt — force rebuild on next normal paint.
			req.Reason = ReasonResize
		}
	}
	if e.altActive {
		return e.drawAltFrame(req, width, height, isImage)
	}

	// --- resize viewport-only throwaway path (no ledger advance) ---
	if req.ResizeViewportOnly && e.hasEverRendered && !req.forceWindowIntent() &&
		!hasVisibleOverlay(req.Overlays, width, height) {
		return e.drawResizeViewport(req, width, height, isImage)
	}

	rawFrame := req.resolvedRows()
	// Working copy only when we must strip markers from the caller's rows.
	// Prefer mutating a shallow copy of the slice header's strings via scratch
	// so the caller's Frame.Lines strings stay unchanged when they embed markers.
	needScratch := false
	for _, line := range rawFrame {
		if stringsContainsMarker(line) {
			needScratch = true
			break
		}
	}
	var work []string
	if needScratch {
		if cap(e.rawScratch) < len(rawFrame) {
			e.rawScratch = make([]string, len(rawFrame))
		} else {
			e.rawScratch = e.rawScratch[:len(rawFrame)]
		}
		copy(e.rawScratch, rawFrame)
		work = e.rawScratch
	} else {
		work = rawFrame
	}

	markers := ExtractCursorMarkers(work)
	// Explicit cursor from request/frame as additional candidate (bottom-most wins
	// among markers; explicit used when no marker in window).
	explicitCursor := req.resolvedCursor()

	live, commit, snapshot := req.resolvedSeams()
	stable := req.StablePrefixRows
	if stable < 0 {
		stable = 0
	}
	// Invalidate prepared cache to the stable floor so the dirty tail re-fits.
	e.prep.InvalidateTo(stable)

	firstPaint := !e.hasEverRendered
	replaceRequested := req.clearScrollbackIntent()
	// Desync forces full-paint recovery (no ED3 unless replace/reset also set).
	if e.desync {
		firstPaint = true
		e.forceViewportRepaint = true
	}

	geometryChanged := req.Reason == ReasonResize ||
		(e.previousWidth > 0 && e.previousWidth != width) ||
		(e.previousHeight > 0 && e.previousHeight != height)
	if geometryChanged {
		e.forgetHardwareCursorState()
		e.prep.Reset()
	}

	visibleOverlay := hasVisibleOverlay(req.Overlays, width, height)

	// Snapshot ledger BEFORE Step so emit failure can roll back pre-emit mutations
	// (audit re-anchor, shrink reslice, geometry rebase).
	preStep := e.ledger.Snapshot()
	// Deep-copy Prefix so rollback does not alias a mutated backing array.
	if preStep.Prefix != nil {
		p := make([]string, len(preStep.Prefix))
		copy(p, preStep.Prefix)
		preStep.Prefix = p
	}

	// Cursor rows for the short focused-cursor-tail branch (absolute frame rows).
	cursorRows := make([]int, 0, len(markers)+1)
	for _, m := range markers {
		cursorRows = append(cursorRows, m.Row)
	}
	if explicitCursor != nil {
		cursorRows = append(cursorRows, explicitCursor.Row)
	}

	in := ledger.NewFrameInput(work, height)
	in.LiveStart = live
	in.CommitSafe = commit
	in.SnapshotSafe = snapshot
	in.FirstPaint = firstPaint
	in.ReplaceRequested = replaceRequested
	in.GeometryChanged = geometryChanged
	in.ResizeRepaintsInPlace = e.caps.ResizeRepaintsInPlace
	in.IsMultiplexer = e.caps.Multiplexer
	in.HasVisibleOverlay = visibleOverlay
	in.StablePrefixRows = stable
	in.CursorRows = cursorRows

	res := e.ledger.Step(in)
	// Capture post-audit pre-plan committed index for emit chunkFrom.
	// After Step, shrink/geometry may have already mutated CommittedRows/Prefix.
	// Plan.PreCommittedRows is the post-audit value before those mutations.
	e.framePreCommitted = res.Plan.PreCommittedRows

	plan := res.Plan
	windowTop := plan.WindowTop
	chunkTo := plan.ChunkTo

	// Rollback helper — restore pre-Step ledger on any failure after Step.
	rollback := func() {
		e.ledger = preStep
		e.framePreCommitted = -1
	}

	// Pick visible cursor (bottom-most marker at/below window top).
	var cursorPos *CursorPos
	if m := pickVisibleCursor(markers, windowTop); m != nil {
		cursorPos = m
	} else if explicitCursor != nil && explicitCursor.Row >= windowTop {
		cursorPos = explicitCursor
	}

	// Prepare frame (width-fit, normalize, coalesce happens at terminalLine).
	prepared := e.prep.PrepareFrame(work, width, isImage)

	// Build window slice.
	window := make([]string, height)
	for r := range height {
		idx := windowTop + r
		if idx >= 0 && idx < len(prepared) {
			window[r] = prepared[idx]
		}
	}
	if visibleOverlay {
		window = compositeOverlaysIntoWindow(window, req.Overlays, width, height, isImage)
		// Overlay may introduce markers; strip and prefer them.
		ovMarkers := ExtractCursorMarkers(window)
		if len(ovMarkers) > 0 {
			// ovMarkers are window-local rows, bottom-first.
			cursorPos = &CursorPos{
				Row: windowTop + ovMarkers[0].Row,
				Col: ovMarkers[0].Col,
			}
		}
		window = prepareLinesArray(window, width, isImage)
	}

	fullPaint := plan.Kind == ledger.PlanFullPaint
	// Force-window on update path when requested or mux geometry.
	forceWindow := e.forceViewportRepaint || req.forceWindowIntent() ||
		(geometryChanged && e.caps.ResizeRepaintsInPlace)
	if req.Reason == ReasonForce {
		forceWindow = true
		e.forceViewportRepaint = true
	}

	clearSB := plan.ClearScrollback
	// Geometry rebuild / replace already set ClearScrollback in the plan
	// (suppressed in mux). ReasonReset/Replace without plan flag still want it
	// outside mux when first paint was false.
	if (req.Reason == ReasonReplace || req.Reason == ReasonReset) && !e.caps.Multiplexer {
		clearSB = true
	}

	e.trace(TraceEvent{
		Kind:            "plan",
		Mode:            planMode(fullPaint),
		ClearScrollback: clearSB,
		ChunkFrom:       e.framePreCommitted,
		ChunkTo:         chunkTo,
		WindowTop:       windowTop,
		FrameLen:        len(work),
		Width:           width,
		Height:          height,
		ForceRewrite:    forceWindow,
	})

	prevWindowTop := preStep.WindowTop
	prevHW := e.hwCursor.row

	var emitErr error
	if fullPaint {
		emitErr = e.emitFullPaint(
			prepared, window, width, height, cursorPos,
			req.ImagePurge, req.ImageTransmit,
			clearSB, chunkTo, windowTop,
		)
	} else {
		if req.ImageTransmit != "" {
			if _, err := ioWriteString(e.out, req.ImageTransmit); err != nil {
				rollback()
				e.desync = true
				e.trace(TraceEvent{Kind: "error", Error: err.Error()})
				return err
			}
		}
		emitErr = e.emitUpdate(
			prepared, window, width, height, cursorPos,
			req.ImagePurge,
			chunkTo, windowTop, prevWindowTop, prevHW,
			forceWindow,
		)
	}

	if emitErr != nil {
		rollback()
		e.desync = true
		e.trace(TraceEvent{Kind: "error", Error: emitErr.Error(), Mode: planMode(fullPaint)})
		return emitErr
	}

	// Successful emit — advance ledger bookkeeping.
	e.ledger.Finish(work, res)
	e.framePreCommitted = -1
	e.hasEverRendered = true
	e.desync = false
	if fullPaint {
		e.forceViewportRepaint = false
	}

	if req.Notify != nil {
		component.NotifyCommittedRows(req.Notify, e.ledger.CommittedRows)
	}
	return nil
}

func planMode(full bool) string {
	if full {
		return "fullPaint"
	}
	return "update"
}

func stringsContainsMarker(s string) bool {
	return len(s) > 0 && (len(ansitext.CursorMarker) > 0) &&
		(len(s) >= len(ansitext.CursorMarker)) &&
		containsMarker(s)
}

func containsMarker(s string) bool {
	// Small helper avoiding import cycle with strings in hot comment path.
	m := ansitext.CursorMarker
	for i := 0; i+len(m) <= len(s); i++ {
		if s[i:i+len(m)] == m {
			return true
		}
	}
	return false
}

func (e *Engine) drawAltFrame(req Request, width, height int, isImage func(string) bool) error {
	base := make([]string, height)
	lines := compositeOverlaysIntoWindow(base, req.Overlays, width, height, isImage)
	_ = ExtractCursorMarkers(lines) // strip; hardware cursor stays hidden on alt
	lines = prepareLinesArray(lines, width, isImage)
	if req.ImageTransmit != "" {
		if _, err := ioWriteString(e.out, req.ImageTransmit); err != nil {
			return err
		}
	}
	force := e.forceViewportRepaint || req.forceWindowIntent()
	e.forceViewportRepaint = false
	if err := e.emitAltFrame(lines, width, height, force); err != nil {
		e.desync = true
		return err
	}
	return nil
}

func (e *Engine) drawResizeViewport(req Request, width, height int, isImage func(string) bool) error {
	raw := req.resolvedRows()
	// Bottom `height` rows of the frame, top-aligned when short.
	n := len(raw)
	contentRows := n
	if contentRows > height {
		contentRows = height
	}
	window := make([]string, height)
	start := n - contentRows
	if start < 0 {
		start = 0
	}
	for i := range contentRows {
		window[i] = raw[start+i]
	}
	_ = ExtractCursorMarkers(window)
	window = prepareLinesArray(window, width, isImage)
	return e.emitResizeViewport(window, height, contentRows, width)
}

// MarkResizeEvent notes that a SIGWINCH (or equivalent) occurred so the next
// Draw classifies as geometry-changed even if dimensions net out unchanged.
func (e *Engine) MarkResizeEvent() {
	// Represented by forcing previous dimensions to a sentinel mismatch on next
	// compare only when caller also passes ReasonResize. Stored as force flag.
	e.forceViewportRepaint = true
	// Zeroing previous dims would mis-detect first paint; instead callers should
	// pass ReasonResize. This method arms force rewrite as a safety net.
}

// ForceNextWindowRewrite arms a full window rewrite on the next update emit.
func (e *Engine) ForceNextWindowRewrite() {
	e.forceViewportRepaint = true
}

// DebugString returns a short ledger summary for diagnostics.
func (e *Engine) DebugString() string {
	return fmt.Sprintf("C=%d W=%d A=%d D=%d Lprev=%d %dx%d ever=%v desync=%v",
		e.ledger.CommittedRows, e.ledger.WindowTop,
		e.ledger.AuditRows, e.ledger.DurableRows,
		e.previousFrameLen, e.previousWidth, e.previousHeight,
		e.hasEverRendered, e.desync,
	)
}
