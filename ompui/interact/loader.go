package interact

import (
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// Spinner advance period matching OMP Loader.
const SpinnerAdvance = 80 * time.Millisecond

var defaultSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ClockFunc returns the current time. Injected for deterministic tests.
// When nil, time.Now is used only if Advance is called without an explicit now;
// the loader never starts a global timer.
type ClockFunc func() time.Time

// Loader is a single-line spinner + message. Frame advances only when the host
// calls Advance(now) or Tick(); there is no internal goroutine or global timer.
type Loader struct {
	frames       []string
	currentFrame int
	message      string
	spinnerColor Style
	messageColor Style
	// AnimatedMessage when true hints the host to repaint ~30fps for time-based
	// message colorizers (matches OMP LoaderMessageColorFn.animated).
	AnimatedMessage bool

	clock         ClockFunc
	lastSpinnerAt time.Time
	started       bool
	disposed      bool

	// OnInvalidate is invoked when the display changes (host should request render).
	OnInvalidate func()

	gen    component.Gen
	cached component.Frame
	cacheW int
	dirty  bool
}

// NewLoader constructs a loader with the given styles and message.
// Does not auto-start; call Start then drive Advance from the host clock.
func NewLoader(spinnerColor, messageColor Style, message string, frames ...string) *Loader {
	l := &Loader{
		frames:       defaultSpinnerFrames,
		message:      message,
		spinnerColor: spinnerColor,
		messageColor: messageColor,
		dirty:        true,
	}
	if message == "" {
		l.message = "Loading..."
	}
	if len(frames) > 0 {
		l.frames = append([]string(nil), frames...)
	}
	return l
}

// SetClock injects a deterministic clock (optional).
func (l *Loader) SetClock(clock ClockFunc) {
	l.clock = clock
}

// Start arms the spinner baseline. Idempotent while already started.
func (l *Loader) Start() {
	if l.disposed {
		return
	}
	now := l.now()
	l.lastSpinnerAt = now
	l.started = true
	l.dirty = true
	l.notify()
}

// Stop freezes the spinner (keeps current frame).
func (l *Loader) Stop() {
	l.started = false
}

// Started reports whether the loader is armed for Advance.
func (l *Loader) Started() bool { return l.started && !l.disposed }

// SetMessage updates the message text.
func (l *Loader) SetMessage(message string) {
	if message == l.message {
		return
	}
	l.message = message
	l.dirty = true
	l.notify()
}

// Message returns the current message.
func (l *Loader) Message() string { return l.message }

// Advance moves the spinner based on elapsed time since the last advance.
// Hosts should call this from their render scheduler; no internal timer runs.
func (l *Loader) Advance(now time.Time) {
	if l.disposed || !l.started || len(l.frames) == 0 {
		return
	}
	if l.lastSpinnerAt.IsZero() {
		l.lastSpinnerAt = now
	}
	elapsed := now.Sub(l.lastSpinnerAt)
	if elapsed < SpinnerAdvance {
		if l.AnimatedMessage {
			l.dirty = true
			l.notify()
		}
		return
	}
	steps := int(elapsed / SpinnerAdvance)
	if steps < 1 {
		steps = 1
	}
	l.currentFrame = (l.currentFrame + steps) % len(l.frames)
	l.lastSpinnerAt = l.lastSpinnerAt.Add(time.Duration(steps) * SpinnerAdvance)
	l.dirty = true
	l.notify()
}

// Tick advances using the injected clock or time.Now.
func (l *Loader) Tick() {
	l.Advance(l.now())
}

// FrameIndex returns the current spinner frame index.
func (l *Loader) FrameIndex() int { return l.currentFrame }

// Invalidate implements component.Invalidator.
func (l *Loader) Invalidate() {
	l.dirty = true
	l.cached = component.Frame{}
	l.cacheW = -1
}

// Dispose implements component.Disposable. Idempotent.
func (l *Loader) Dispose() {
	if l.disposed {
		return
	}
	l.disposed = true
	l.started = false
	l.OnInvalidate = nil
}

// Render implements component.Component.
// Leading blank row matches OMP Loader (["", ...text lines]).
func (l *Loader) Render(width int) component.Frame {
	if width < 1 {
		width = 1
	}
	frame := ""
	if len(l.frames) > 0 {
		frame = l.frames[l.currentFrame%len(l.frames)]
	}
	spin := applyStyle(l.spinnerColor, frame)
	msg := applyStyle(l.messageColor, l.message)
	text := spin + " " + msg
	if ansitext.VisibleWidth(text) > width {
		text = ansitext.TruncateToWidth(text, width, "")
	}
	// OMP Loader: leading empty row then text.
	lines := []string{"", text}
	for i := range lines {
		if ansitext.VisibleWidth(lines[i]) > width {
			lines[i] = ansitext.SliceByColumn(lines[i], 0, width)
		}
	}
	changed := l.dirty || l.cacheW != width || !sameLines(l.cached.Lines, lines)
	gen := l.gen.Touch(changed)
	l.dirty = false
	l.cacheW = width
	if !changed && l.cached.Lines != nil {
		return l.cached
	}
	// Entire loader is live (spinner).
	fr := component.NewFrame(lines, gen).WithSeams(0, 0, len(lines))
	l.cached = fr
	return fr
}

func (l *Loader) now() time.Time {
	if l.clock != nil {
		return l.clock()
	}
	return time.Now()
}

func (l *Loader) notify() {
	if l.OnInvalidate != nil {
		l.OnInvalidate()
	}
}

// CancellableLoader is a Loader that aborts on escape/ctrl+c.
type CancellableLoader struct {
	Loader
	aborted bool
	// OnAbort fires once when the user cancels.
	OnAbort func()
	// OnCancel is an alias path; prefer OnAbort. Both fire if set.
	cancelled bool
}

// NewCancellableLoader constructs a cancellable spinner.
func NewCancellableLoader(spinnerColor, messageColor Style, message string, frames ...string) *CancellableLoader {
	cl := &CancellableLoader{}
	cl.Loader = *NewLoader(spinnerColor, messageColor, message, frames...)
	return cl
}

// Aborted reports whether cancel was requested.
func (cl *CancellableLoader) Aborted() bool { return cl.aborted }

// HandleInput implements component.InputHandler. Escape / ctrl+c abort once.
func (cl *CancellableLoader) HandleInput(ev event.Event) {
	if cl.aborted || cl.disposed {
		return
	}
	if IsCancel(ev) {
		cl.aborted = true
		cl.Stop()
		if cl.OnAbort != nil {
			cl.OnAbort()
		}
	}
}

// Dispose stops and clears callbacks.
func (cl *CancellableLoader) Dispose() {
	cl.Loader.Dispose()
	cl.OnAbort = nil
}
