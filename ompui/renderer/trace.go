package renderer

import (
	"fmt"
	"io"
	"sync"
)

// TraceEvent is one deterministic render-path observation. Fields are stable
// for test oracles; values are ints/bools/short strings only (no wall clock).
type TraceEvent struct {
	Kind string // "plan" | "emit" | "cursor" | "error" | "schedule"

	// Plan / emit classification.
	Mode           string // "fullPaint" | "scrollAppend" | "windowDiff" | "seamRewrite" | "cursorOnly" | "alt"
	ClearScrollback bool
	ChunkFrom      int
	ChunkTo        int
	WindowTop      int
	FrameLen       int
	Width          int
	Height         int
	ForceRewrite   bool
	BytesWritten   int
	Error          string
}

// TraceWriter receives ordered engine observations. Implementations must not
// panic; a nil TraceWriter on Engine is a no-op.
type TraceWriter interface {
	Trace(ev TraceEvent)
}

// FuncTrace adapts a function to TraceWriter.
type FuncTrace func(TraceEvent)

// Trace implements TraceWriter.
func (f FuncTrace) Trace(ev TraceEvent) {
	if f != nil {
		f(ev)
	}
}

// WriterTrace formats one line per event to w. Safe for concurrent use.
type WriterTrace struct {
	mu sync.Mutex
	W  io.Writer
}

// Trace implements TraceWriter.
func (t *WriterTrace) Trace(ev TraceEvent) {
	if t == nil || t.W == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = fmt.Fprintf(t.W,
		"kind=%s mode=%s clear=%v chunk=%d..%d win=%d L=%d %dx%d force=%v bytes=%d err=%q\n",
		ev.Kind, ev.Mode, ev.ClearScrollback, ev.ChunkFrom, ev.ChunkTo, ev.WindowTop,
		ev.FrameLen, ev.Width, ev.Height, ev.ForceRewrite, ev.BytesWritten, ev.Error,
	)
}

// MemTrace records events in memory (tests).
type MemTrace struct {
	mu     sync.Mutex
	Events []TraceEvent
}

// Trace implements TraceWriter.
func (t *MemTrace) Trace(ev TraceEvent) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.Events = append(t.Events, ev)
	t.mu.Unlock()
}

// Snapshot returns a copy of recorded events.
func (t *MemTrace) Snapshot() []TraceEvent {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TraceEvent, len(t.Events))
	copy(out, t.Events)
	return out
}

// Reset clears recorded events.
func (t *MemTrace) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.Events = t.Events[:0]
	t.mu.Unlock()
}
