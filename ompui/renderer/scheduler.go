package renderer

import (
	"sync"
	"time"
)

// Clock abstracts time for deterministic tests.
type Clock interface {
	Now() time.Time
	// AfterFunc runs f after d; returned cancel stops it if not yet fired.
	AfterFunc(d time.Duration, f func()) (cancel func())
}

// systemClock is the process wall clock.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) AfterFunc(d time.Duration, f func()) func() {
	t := time.AfterFunc(d, f)
	return func() { t.Stop() }
}

// ManualClock is a deterministic test clock.
type ManualClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []manualTimer
	nextID int
}

type manualTimer struct {
	id       int
	when     time.Time
	fn       func()
	canceled bool
}

// NewManualClock starts at the given time (zero => Unix zero).
func NewManualClock(start time.Time) *ManualClock {
	return &ManualClock{now: start}
}

// Now implements Clock.
func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AfterFunc implements Clock.
func (c *ManualClock) AfterFunc(d time.Duration, f func()) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	id := c.nextID
	c.timers = append(c.timers, manualTimer{id: id, when: c.now.Add(d), fn: f})
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i := range c.timers {
			if c.timers[i].id == id {
				c.timers[i].canceled = true
				return
			}
		}
	}
}

// Advance moves time forward and fires due timers in order.
func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	var due []func()
	remaining := c.timers[:0]
	for _, t := range c.timers {
		if t.canceled {
			continue
		}
		if !t.when.After(now) {
			due = append(due, t.fn)
			continue
		}
		remaining = append(remaining, t)
	}
	c.timers = remaining
	c.mu.Unlock()
	for _, fn := range due {
		fn()
	}
}

// SchedulerConfig tunes coalescing / throttle.
type SchedulerConfig struct {
	// MinInterval caps ordinary repaint rate. Zero => DefaultMinRenderIntervalMs.
	MinInterval time.Duration
	// Clock overrides the wall clock. Nil => system clock.
	Clock Clock
}

// DefaultScheduler returns a 30fps config on the system clock.
func DefaultScheduler() SchedulerConfig {
	return SchedulerConfig{
		MinInterval: defaultMinInterval(),
	}
}

func defaultMinInterval() time.Duration {
	ms := DefaultMinRenderIntervalMs
	return time.Duration(ms * float64(time.Millisecond))
}

// Scheduler serializes concurrent render requests against one Engine and caps
// ordinary repaints to MinInterval while allowing immediate/flush paths.
//
// Contract:
//   - Request / RequestImmediate are safe for concurrent use.
//   - Only one Draw runs at a time.
//   - Coalesced ordinary requests collapse to the latest Request.
//   - ReasonForce / ReasonReplace / ReasonReset / ReasonFlush / ReasonResize
//     schedule immediately (still serialized).
//   - Stop cancels pending timers and waits for an in-flight Draw to finish.
type Scheduler struct {
	eng *Engine
	cfg SchedulerConfig
	clk Clock

	mu sync.Mutex

	pending      *Request
	pendingForce bool // pending carries immediate intent
	scheduled    bool
	timerCancel  func()
	lastDrawAt   time.Time
	drawing      bool
	stopped      bool

	// wake is signaled when a draw should run; the loop owns Draw calls.
	// We use a simple "run draw inline under lock release" model driven by
	// timers and immediate kicks — no dedicated goroutine required. Callers
	// of Flush wait for completion via cond.
	cond *sync.Cond

	lastErr       error
	committedRows int
}

// NewScheduler wraps eng. eng must be non-nil.
func NewScheduler(eng *Engine, cfg SchedulerConfig) *Scheduler {
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = defaultMinInterval()
	}
	clk := cfg.Clock
	if clk == nil {
		clk = systemClock{}
	}
	s := &Scheduler{
		eng:           eng,
		cfg:           cfg,
		clk:           clk,
		committedRows: eng.CommittedRows(),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Engine returns the underlying engine.
func (s *Scheduler) Engine() *Engine { return s.eng }

// CommittedRows returns the last completed draw's scrollback boundary.
func (s *Scheduler) CommittedRows() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.committedRows
}

// ForceNextWindowRewrite serializes a viewport rewrite request with Draw.
func (s *Scheduler) ForceNextWindowRewrite() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitForIdleLocked()
	s.eng.ForceNextWindowRewrite()
}

// MarkResizeEvent serializes resize state with Draw.
func (s *Scheduler) MarkResizeEvent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitForIdleLocked()
	s.eng.MarkResizeEvent()
}

// SetCaps serializes a terminal capability update with Draw.
func (s *Scheduler) SetCaps(c Caps) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitForIdleLocked()
	s.eng.SetCaps(c)
}

func (s *Scheduler) waitForIdleLocked() {
	for s.drawing {
		s.cond.Wait()
	}
}

// LastError returns the error from the most recent Draw, if any.
func (s *Scheduler) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

// Stop cancels pending work, waits for the active Draw, and rejects new requests.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.timerCancel != nil {
		s.timerCancel()
		s.timerCancel = nil
	}
	s.pending = nil
	s.scheduled = false
	s.waitForIdleLocked()
	s.cond.Broadcast()
}

// Request queues an ordinary (throttled) repaint, coalescing with any pending
// request. Immediate reasons are promoted automatically.
func (s *Scheduler) Request(req Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.mergeLocked(req, isImmediateReason(req.Reason))
	s.armLocked()
}

// RequestImmediate queues req on the immediate path (still serialized).
func (s *Scheduler) RequestImmediate(req Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.mergeLocked(req, true)
	s.armLocked()
}

// Flush runs the pending request (or req if provided) synchronously and waits
// until Draw completes. Useful for tests and shutdown.
func (s *Scheduler) Flush(req *Request) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	if req != nil {
		s.mergeLocked(*req, true)
	} else if s.pending == nil {
		s.mu.Unlock()
		return s.lastErr
	} else {
		s.pendingForce = true
	}
	// Cancel timer; run now.
	if s.timerCancel != nil {
		s.timerCancel()
		s.timerCancel = nil
	}
	s.scheduled = false
	for s.drawing {
		s.cond.Wait()
		if s.stopped {
			s.mu.Unlock()
			return s.lastErr
		}
	}
	s.runDrawLocked()
	err := s.lastErr
	s.mu.Unlock()
	return err
}

func isImmediateReason(r Reason) bool {
	switch r {
	case ReasonForce, ReasonReplace, ReasonReset, ReasonResize, ReasonFlush:
		return true
	default:
		return false
	}
}

func (s *Scheduler) mergeLocked(req Request, force bool) {
	if s.pending == nil {
		cp := req
		s.pending = &cp
	} else {
		// Latest frame wins; OR force/clear intents.
		prev := s.pending
		merged := req
		// Preserve stronger reasons.
		if reasonRank(prev.Reason) > reasonRank(merged.Reason) {
			merged.Reason = prev.Reason
		}
		// OR overlays: prefer newer list (caller's latest tree).
		// Stable prefix: take min (more invalidation).
		if prev.StablePrefixRows < merged.StablePrefixRows {
			merged.StablePrefixRows = prev.StablePrefixRows
		}
		// Concat image sequences.
		if prev.ImageTransmit != "" {
			merged.ImageTransmit = prev.ImageTransmit + merged.ImageTransmit
		}
		if prev.ImagePurge != "" {
			merged.ImagePurge = prev.ImagePurge + merged.ImagePurge
		}
		s.pending = &merged
	}
	if force {
		s.pendingForce = true
	}
}

func reasonRank(r Reason) int {
	switch r {
	case ReasonReplace, ReasonReset:
		return 5
	case ReasonResize:
		return 4
	case ReasonForce:
		return 3
	case ReasonFlush:
		return 2
	default:
		return 1
	}
}

func (s *Scheduler) armLocked() {
	if s.drawing {
		// Draw loop will re-check pending when finished.
		return
	}
	if s.pending == nil {
		return
	}
	if s.pendingForce {
		// Immediate: cancel throttle timer and run ASAP.
		if s.timerCancel != nil {
			s.timerCancel()
			s.timerCancel = nil
		}
		s.scheduled = false
		// Kick async so Request doesn't block on Draw under the caller's stack
		// unless they used Flush. Use AfterFunc(0).
		s.timerCancel = s.clk.AfterFunc(0, s.timerFire)
		s.scheduled = true
		s.traceSchedule("immediate")
		return
	}
	if s.scheduled {
		return
	}
	elapsed := s.clk.Now().Sub(s.lastDrawAt)
	delay := s.cfg.MinInterval - elapsed
	if delay < 0 {
		delay = 0
	}
	s.timerCancel = s.clk.AfterFunc(delay, s.timerFire)
	s.scheduled = true
	s.traceSchedule("throttle")
}

func (s *Scheduler) timerFire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timerCancel = nil
	s.scheduled = false
	if s.stopped || s.pending == nil {
		return
	}
	if s.drawing {
		return
	}
	s.runDrawLocked()
}

// runDrawLocked executes one Draw. Caller holds s.mu; released around eng.Draw.
func (s *Scheduler) runDrawLocked() {
	if s.pending == nil || s.drawing {
		return
	}
	req := *s.pending
	s.pending = nil
	force := s.pendingForce
	s.pendingForce = false
	s.drawing = true
	s.mu.Unlock()

	// Apply force window rewrite for force reasons.
	if force && req.Reason == ReasonUpdate {
		req.Reason = ReasonFlush
	}
	err := s.eng.Draw(req)

	s.mu.Lock()
	s.lastErr = err
	s.committedRows = s.eng.CommittedRows()
	s.lastDrawAt = s.clk.Now()
	s.drawing = false
	s.cond.Broadcast()
	// If more work arrived during Draw, arm again.
	if s.pending != nil && !s.stopped {
		s.armLocked()
	}
}

func (s *Scheduler) traceSchedule(mode string) {
	if s.eng != nil {
		s.eng.trace(TraceEvent{Kind: "schedule", Mode: mode})
	}
}
