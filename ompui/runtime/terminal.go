package runtime

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/input"
	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
)

// Terminal is the sole TTY owner for the Go frontend.
//
// Construct with New, then Start. Events delivers decoded input in order.
// Write/WriteString serialize output. Stop/Restore are idempotent.
type Terminal struct {
	in  *os.File
	out *os.File

	opts Options

	mu sync.Mutex

	started bool
	stopped bool
	dead    bool // write path permanently failed

	// Lifecycle
	plat         platformState
	altScreen    bool
	mouseEnabled bool
	kittyActive  bool
	kittyEnable  string
	modifyOther  bool
	inBandResize bool
	progressOn   bool

	// Capabilities / identity
	caps       *termcaps.Snapshot
	cells      *termcaps.CellSize
	ttyID      string
	appearance Appearance
	osc99OK    bool
	osc99Caps  map[string]string
	osc99Next  int

	privateModeSupport map[int]bool
	xtermScrollRestore map[int]struct{}

	// Geometry
	cols, rows        int
	widthPx, heightPx int
	reportedCols      int
	reportedRows      int
	hasReportedCols   bool
	hasReportedRows   bool

	// Channels / loops
	events   chan event.Event
	resizeCh chan struct{}
	writeCh  chan writeReq
	cmdCh    chan any
	stopCh   loopSignal
	loopDone chan struct{}
	wg       sync.WaitGroup

	// Timers owned by loop (via cmd) or lifecycle
	progressStop chan struct{}
	osc11Stop    chan struct{}

	// Decoder + probes — input loop only after Start
	dec   *input.Decoder
	probe probeRouter

	// Pending kitty fallback timer handle (loop-owned)
	kittyFallbackCancel context.CancelFunc

	// mode2031 debounce
	mode2031Cancel context.CancelFunc

	writeLog func([]byte)

	// conPTY chunking
	conPTY bool

	// env as plain map for platform helpers
	envMap map[string]string
}

type writeReq struct {
	data []byte
	done chan error
}

// loopSignal is a closable stop broadcast.
type loopSignal struct {
	once sync.Once
	ch   chan struct{}
}

func newLoopSignal() loopSignal {
	return loopSignal{ch: make(chan struct{})}
}

func (s *loopSignal) stop() {
	s.once.Do(func() { close(s.ch) })
}

func (s *loopSignal) done() <-chan struct{} { return s.ch }

// command types for the input coordinator (serialized with reads).
type (
	cmdQueryCPR struct {
		ctx context.Context
		res chan cprResult
	}
	cmdEnableMouse struct{ enable bool }
	cmdSetTitle    struct{ title string }
	cmdSetProgress struct{ active bool }
	cmdNotify      struct{ n Notification }
	cmdDrain       struct {
		max  time.Duration
		idle time.Duration
		done chan struct{}
	}
	cmdEnterAlt      struct{ enter bool }
	cmdQueryBG       struct{}
	cmdStartProbes   struct{}
	cmdCPRTimeout    struct{ res chan cprResult }
	cmdKittyFallback struct{}
)

// New constructs a Terminal over injected input/output files.
// in is read for user input and probe replies; out is written for all output.
// Neither file is closed by Terminal.
func New(in, out *os.File, opts Options) (*Terminal, error) {
	if in == nil || out == nil {
		return nil, errNilFile
	}
	opts = opts.withDefaults()

	envMap := map[string]string{}
	for k, v := range opts.Env {
		envMap[k] = v
	}

	isTTY := isTerminalFile(out)
	caps := termcaps.Resolve(termcaps.ResolveOptions{
		Env:   opts.Env,
		IsTTY: isTTY,
	})

	ttyPath := opts.TTYPath
	if ttyPath == "" && isTerminalFile(in) {
		ttyPath = ttyDevicePath(int(in.Fd()))
	}
	ttyID := termcaps.ResolveSessionID(isTerminalFile(in), ttyPath, opts.Env)

	cols, rows := 80, 24
	if c, r, err := termSize(out); err == nil {
		cols, rows = c, r
	}
	wPx, hPx := platformWindowPixels(int(out.Fd()))

	t := &Terminal{
		in:                 in,
		out:                out,
		opts:               opts,
		caps:               caps,
		cells:              termcaps.NewCellSize(),
		ttyID:              ttyID,
		cols:               cols,
		rows:               rows,
		widthPx:            wPx,
		heightPx:           hPx,
		privateModeSupport: make(map[int]bool),
		xtermScrollRestore: make(map[int]struct{}),
		writeLog:           opts.WriteLog,
		conPTY:             platformIsConPTY(opts.Platform, envMap),
		envMap:             envMap,
		osc99Caps:          make(map[string]string),
	}
	return t, nil
}

// Start enables raw mode, starts the input coordinator and resize watcher,
// and begins capability probes. Idempotent error if already started.
func (t *Terminal) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.started && !t.stopped {
		t.mu.Unlock()
		return errAlreadyStarted
	}
	// Allow re-Start after Stop.
	t.stopped = false
	t.dead = false
	t.started = true

	t.events = make(chan event.Event, t.opts.EventBuffer)
	t.resizeCh = make(chan struct{}, 1)
	t.writeCh = make(chan writeReq, 32)
	t.cmdCh = make(chan any, 32)
	t.stopCh = newLoopSignal()
	t.loopDone = make(chan struct{})

	headless := t.opts.Headless
	enterAlt := t.opts.EnterAltScreen
	t.mu.Unlock()

	if headless {
		// Open events channel but skip all side effects.
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			t.headlessLoop(ctx)
		}()
		return nil
	}

	// Writer loop first so lifecycle writes never block on an empty consumer.
	t.wg.Add(1)
	go t.writeLoop()

	// Raw mode first.
	st, err := enableRaw(t.in)
	if err != nil {
		t.stopCh.stop()
		t.mu.Lock()
		t.started = false
		t.mu.Unlock()
		// writeLoop exits on stopCh
		t.wg.Wait()
		return err
	}
	t.mu.Lock()
	t.plat.raw = st
	t.plat.rawFD = int(t.in.Fd())
	t.mu.Unlock()

	// Windows VT input after raw (raw resets console flags).
	winSt := platformEnableVTInput(t.in)
	t.mu.Lock()
	t.plat.win = winSt
	t.mu.Unlock()

	if enterAlt {
		if err := t.writeRaw([]byte(seqEnterAltScreen)); err != nil {
			_ = t.restorePlatform()
			t.stopCh.stop()
			t.mu.Lock()
			t.started = false
			t.mu.Unlock()
			t.wg.Wait()
			return err
		}
		t.mu.Lock()
		t.altScreen = true
		t.mu.Unlock()
		setAltScreenActiveLocked(true)
	}

	// Bracketed paste.
	_ = t.writeRaw([]byte(seqBracketedPasteEnable))

	registerActive(t)

	// Decoder
	t.dec = input.NewDecoder(input.Options{
		KittyActive:     false,
		WindowsTerminal: t.opts.WindowsTerminal,
	})

	// Input coordinator + resize watcher.
	t.wg.Add(1)
	go t.inputLoop(ctx)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		startResizeWatcher(t.out, t.resizeCh, t.stopCh.done())
	}()

	// Kick probes from the input loop so DA1 FIFO is single-threaded.
	if !t.opts.DisableProbes {
		select {
		case t.cmdCh <- cmdStartProbes{}:
		case <-t.stopCh.done():
		}
	}

	return nil
}

func (t *Terminal) headlessLoop(ctx context.Context) {
	defer close(t.loopDone)
	defer close(t.events)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh.done():
			return
		case cmd := <-t.cmdCh:
			t.handleCmd(cmd)
		case req := <-t.writeCh:
			if req.done != nil {
				req.done <- nil
			}
		}
	}
}

func (t *Terminal) startProbes() {
	// Kitty keyboard query + DA1 sentinel.
	t.probe.pushOwner(da1Owner{kind: da1Keyboard})
	_ = t.enqueueWrite([]byte(seqKittyKeyboardQuery + seqDA1))

	// Fallback timer for modifyOtherKeys.
	fbCtx, fbCancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.kittyFallbackCancel = fbCancel
	timeout := t.opts.KittyFallbackTimeout
	t.mu.Unlock()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-fbCtx.Done():
			return
		case <-t.stopCh.done():
			return
		case <-timer.C:
			select {
			case t.cmdCh <- cmdKittyFallback{}:
			case <-t.stopCh.done():
			}
		}
	}()

	// OSC 11
	t.queryBackgroundColor()

	// OSC 99
	t.queryOsc99Support()

	// Mode 2031 enable (subscribe); DECRQM decides whether to keep polling.
	_ = t.enqueueWrite([]byte(seqMode2031Enable))

	// OSC 11 poll (skip WSL).
	if !t.isWSL() && t.opts.Osc11PollInterval > 0 {
		t.startOsc11Poll()
	}

	// DECRQM probes.
	for _, mode := range []int{modeSyncOutput, modeInBandResize, modeAppearanceNotif} {
		t.queryPrivateMode(mode)
	}
	for _, mode := range xtermScrollToBottomModes {
		t.queryPrivateMode(mode)
	}
}

func (t *Terminal) isWSL() bool {
	if t.opts.Platform != "linux" {
		return false
	}
	return t.envMap["WSL_DISTRO_NAME"] != "" || t.envMap["WSL_INTEROP"] != ""
}

func (t *Terminal) startOsc11Poll() {
	stop := make(chan struct{})
	t.mu.Lock()
	if t.osc11Stop != nil {
		close(t.osc11Stop)
	}
	t.osc11Stop = stop
	interval := t.opts.Osc11PollInterval
	t.mu.Unlock()

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.stopCh.done():
				return
			case <-ticker.C:
				t.mu.Lock()
				dead := t.dead || t.stopped
				t.mu.Unlock()
				if dead {
					return
				}
				select {
				case t.cmdCh <- cmdQueryBG{}:
				case <-stop:
					return
				case <-t.stopCh.done():
					return
				}
			}
		}
	}()
}

func (t *Terminal) stopOsc11Poll() {
	t.mu.Lock()
	if t.osc11Stop != nil {
		close(t.osc11Stop)
		t.osc11Stop = nil
	}
	t.mu.Unlock()
}

func (t *Terminal) queryBackgroundColor() {
	t.mu.Lock()
	if t.dead || t.stopped {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()
	// Queue if in flight.
	if t.probe.osc11Pending || t.probe.hasOwner(da1OSC11) {
		t.probe.osc11QueryQueued = true
		return
	}
	t.startOsc11Query()
}

func (t *Terminal) startOsc11Query() {
	t.probe.osc11Pending = true
	t.probe.osc11Buf = nil
	t.probe.pushOwner(da1Owner{kind: da1OSC11})
	_ = t.enqueueWrite([]byte(seqOSC11Query + seqDA1))
}

func (t *Terminal) queryOsc99Support() {
	t.probe.osc99PendingID = ""
	t.probe.osc99Buf = nil
	t.mu.Lock()
	t.osc99OK = false
	t.osc99Caps = make(map[string]string)
	if t.caps.NotifyProtocol != termcaps.NotifyProtocolOSC99 || t.dead || t.stopped {
		t.mu.Unlock()
		return
	}
	t.osc99Next++
	id := "omp-probe-" + itoa(t.osc99Next)
	t.mu.Unlock()

	t.probe.osc99PendingID = id
	t.probe.pushOwner(da1Owner{kind: da1OSC99Probe, id: id})
	_ = t.enqueueWrite([]byte("\x1b]99;i=" + id + ":p=?;\x1b\\" + seqDA1))
}

func (t *Terminal) queryPrivateMode(mode int) {
	t.mu.Lock()
	if t.dead || t.stopped {
		t.mu.Unlock()
		return
	}
	if _, ok := t.privateModeSupport[mode]; ok {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()
	t.probe.pushOwner(da1Owner{kind: da1PrivateMode, mode: mode})
	_ = t.enqueueWrite([]byte("\x1b[?" + itoa(mode) + "$p" + seqDA1))
}

// Events returns the receive-only channel of decoded events.
// Closed after Stop completes and the input loop drains.
func (t *Terminal) Events() <-chan event.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.events
}

// Size returns the last known terminal geometry.
func (t *Terminal) Size() event.Size {
	t.mu.Lock()
	defer t.mu.Unlock()
	cols, rows := t.cols, t.rows
	if t.inBandResize {
		if t.hasReportedCols {
			cols = t.reportedCols
		}
		if t.hasReportedRows {
			rows = t.reportedRows
		}
	}
	return event.Size{
		Cols:     cols,
		Rows:     rows,
		WidthPx:  t.widthPx,
		HeightPx: t.heightPx,
	}
}

// Capabilities returns a point-in-time copy of the live capability snapshot.
func (t *Terminal) Capabilities() *termcaps.Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	snapshot := *t.caps
	return &snapshot
}

// Appearance returns the last detected appearance.
func (t *Terminal) Appearance() Appearance {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.appearance
}

// TTYID returns the stable session identity string (may be empty).
func (t *Terminal) TTYID() string {
	return t.ttyID
}

// KittyProtocolActive reports whether Kitty keyboard protocol is pushed.
func (t *Terminal) KittyProtocolActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.kittyActive
}

// KittyEnableSequence returns the push sequence in effect, or "".
func (t *Terminal) KittyEnableSequence() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.kittyActive {
		return ""
	}
	return t.kittyEnable
}

// CellDimensions returns the current cell pixel size.
func (t *Terminal) CellDimensions() termcaps.CellDimensions {
	return t.cells.Get()
}

// Osc99Supported reports whether OSC 99 structured notifications are confirmed.
func (t *Terminal) Osc99Supported() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.osc99OK
}

// PrivateModeSupported returns DECRQM result for mode, and whether resolved.
func (t *Terminal) PrivateModeSupported(mode int) (supported bool, resolved bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.privateModeSupport[mode]
	return s, ok
}

// Write serializes raw bytes to the output TTY.
func (t *Terminal) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	t.mu.Lock()
	if t.dead {
		t.mu.Unlock()
		return 0, errTerminalDead
	}
	if t.stopped || !t.started {
		t.mu.Unlock()
		// Allow writes during construction probes? Only after Start.
		return 0, errNotStarted
	}
	// Copy — caller may reuse p.
	data := append([]byte(nil), p...)
	ch := t.writeCh
	t.mu.Unlock()

	done := make(chan error, 1)
	select {
	case ch <- writeReq{data: data, done: done}:
	case <-t.stopCh.done():
		return 0, errTerminalStopped
	}
	select {
	case err := <-done:
		if err != nil {
			return 0, err
		}
		return len(p), nil
	case <-t.stopCh.done():
		return 0, errTerminalStopped
	}
}

// WriteString writes s to the output TTY.
func (t *Terminal) WriteString(s string) (int, error) {
	return t.Write([]byte(s))
}

// writeRaw writes bypassing the started check (lifecycle sequences).
func (t *Terminal) writeRaw(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	t.mu.Lock()
	if t.dead {
		t.mu.Unlock()
		return errTerminalDead
	}
	ch := t.writeCh
	// Once started, lifecycle writes stay on the writer loop until stopCh closes.
	// External Write is gated by stopped, so teardown cannot race new output.
	useLoop := ch != nil && t.started
	t.mu.Unlock()
	if !useLoop {
		return t.directWrite(p)
	}
	done := make(chan error, 1)
	select {
	case ch <- writeReq{data: append([]byte(nil), p...), done: done}:
		select {
		case err := <-done:
			return err
		case <-t.stopCh.done():
			// Writer may still complete; prefer its result briefly.
			select {
			case err := <-done:
				return err
			case <-time.After(50 * time.Millisecond):
				return t.directWrite(p)
			}
		}
	case <-t.stopCh.done():
		return t.directWrite(p)
	}
}

func (t *Terminal) enqueueWrite(p []byte) error {
	return t.writeRaw(p)
}

func (t *Terminal) directWrite(p []byte) error {
	t.mu.Lock()
	if t.dead {
		t.mu.Unlock()
		return errTerminalDead
	}
	conPTY := t.conPTY
	logFn := t.writeLog
	out := t.out
	t.mu.Unlock()

	platformEnsureUTF8()

	var err error
	if conPTY && len(p) > maxConPTYWriteChunkBytes {
		for _, chunk := range chunkForConPTY(p, maxConPTYWriteChunkBytes) {
			if _, err = out.Write(chunk); err != nil {
				break
			}
		}
	} else {
		_, err = out.Write(p)
	}
	if err != nil {
		if isRetryableWriteErr(err) {
			return err
		}
		t.mu.Lock()
		t.dead = true
		t.mu.Unlock()
		return err
	}
	if logFn != nil {
		logFn(p)
	}
	return nil
}

func (t *Terminal) writeLoop() {
	defer t.wg.Done()
	for {
		select {
		case <-t.stopCh.done():
			// Drain pending writes best-effort.
			for {
				select {
				case req := <-t.writeCh:
					err := t.directWrite(req.data)
					if req.done != nil {
						req.done <- err
					}
				default:
					return
				}
			}
		case req := <-t.writeCh:
			err := t.directWrite(req.data)
			if req.done != nil {
				req.done <- err
			}
		}
	}
}

// QueryCursorPosition sends CPR and waits for the reply via the input loop.
func (t *Terminal) QueryCursorPosition(ctx context.Context) (CursorPosition, error) {
	t.mu.Lock()
	if !t.started || t.stopped {
		t.mu.Unlock()
		return CursorPosition{}, errNotStarted
	}
	cmdCh := t.cmdCh
	t.mu.Unlock()

	res := make(chan cprResult, 1)
	select {
	case cmdCh <- cmdQueryCPR{ctx: ctx, res: res}:
	case <-ctx.Done():
		return CursorPosition{}, ctx.Err()
	case <-t.stopCh.done():
		return CursorPosition{}, errTerminalStopped
	}
	select {
	case r := <-res:
		return r.pos, r.err
	case <-ctx.Done():
		return CursorPosition{}, ctx.Err()
	case <-t.stopCh.done():
		return CursorPosition{}, errTerminalStopped
	}
}

// SetTitle sets the terminal window title (OSC 0).
func (t *Terminal) SetTitle(title string) error {
	// Strip C0 that would break OSC.
	clean := sanitizeTitle(title)
	return t.sendCmd(cmdSetTitle{title: clean})
}

// SetProgress enables/disables OSC 9;4 progress with keepalive.
func (t *Terminal) SetProgress(active bool) error {
	return t.sendCmd(cmdSetProgress{active: active})
}

// Notify sends a desktop/bell notification.
func (t *Terminal) Notify(n Notification) error {
	if termcaps.IsNotificationSuppressed(t.opts.Env) {
		return nil
	}
	return t.sendCmd(cmdNotify{n: n})
}

// EnableMouse enables SGR + any-event + basic mouse tracking.
func (t *Terminal) EnableMouse() error {
	return t.sendCmd(cmdEnableMouse{enable: true})
}

// DisableMouse disables mouse tracking modes.
func (t *Terminal) DisableMouse() error {
	return t.sendCmd(cmdEnableMouse{enable: false})
}

// EnterAltScreen enters the alternate screen (explicit only).
func (t *Terminal) EnterAltScreen() error {
	return t.sendCmd(cmdEnterAlt{enter: true})
}

// LeaveAltScreen leaves the alternate screen if this Terminal entered it.
func (t *Terminal) LeaveAltScreen() error {
	return t.sendCmd(cmdEnterAlt{enter: false})
}

// DrainInput discards input for up to max (default 1s), exiting early after
// idle (default 50ms) with no bytes. Disables Kitty/modifyOtherKeys first so
// late key-releases do not generate new sequences for the parent shell.
func (t *Terminal) DrainInput(ctx context.Context, max, idle time.Duration) error {
	if max <= 0 {
		max = time.Duration(defaultDrainMaxMs) * time.Millisecond
	}
	if idle <= 0 {
		idle = time.Duration(defaultDrainIdleMs) * time.Millisecond
	}
	t.mu.Lock()
	if !t.started || t.stopped {
		t.mu.Unlock()
		return nil
	}
	cmdCh := t.cmdCh
	t.mu.Unlock()

	done := make(chan struct{})
	select {
	case cmdCh <- cmdDrain{max: max, idle: idle, done: done}:
	case <-ctx.Done():
		return ctx.Err()
	case <-t.stopCh.done():
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.stopCh.done():
		return nil
	}
}

func (t *Terminal) sendCmd(cmd any) error {
	t.mu.Lock()
	if !t.started || t.stopped {
		t.mu.Unlock()
		return errNotStarted
	}
	ch := t.cmdCh
	t.mu.Unlock()
	select {
	case ch <- cmd:
		return nil
	case <-t.stopCh.done():
		return errTerminalStopped
	}
}

// Stop restores terminal modes and stops loops. Idempotent.
func (t *Terminal) Stop() error {
	return t.restore(true)
}

// Restore is an alias for Stop (lifecycle naming).
func (t *Terminal) Restore() error {
	return t.Stop()
}

func (t *Terminal) restore(full bool) error {
	t.mu.Lock()
	if t.stopped || !t.started {
		// Still try platform restore if raw is held (partial Start failure).
		hasRaw := t.plat.raw != nil
		t.mu.Unlock()
		if hasRaw {
			return t.restorePlatform()
		}
		return nil
	}
	t.stopped = true
	t.mu.Unlock()

	unregisterActive(t)

	// Cancel timers.
	t.mu.Lock()
	if t.kittyFallbackCancel != nil {
		t.kittyFallbackCancel()
		t.kittyFallbackCancel = nil
	}
	if t.mode2031Cancel != nil {
		t.mode2031Cancel()
		t.mode2031Cancel = nil
	}
	t.mu.Unlock()
	t.stopOsc11Poll()
	t.clearProgressTimer()

	// Mode teardown sequences (before stopping input so writes still work).
	_ = t.writeRaw([]byte(seqSyncOutputDisable + seqAutowrapEnable))
	_ = t.writeRaw([]byte(seqBracketedPasteDisable))
	_ = t.writeRaw([]byte(seqEnhancedPasteDisable))
	_ = t.writeRaw([]byte(seqMouseDisableAll))
	_ = t.writeRaw([]byte(seqMode2031Disable))

	t.mu.Lock()
	restoreModes := make([]int, 0, len(t.xtermScrollRestore))
	for mode := range t.xtermScrollRestore {
		restoreModes = append(restoreModes, mode)
	}
	t.xtermScrollRestore = make(map[int]struct{})
	inBand := t.inBandResize
	kitty := t.kittyActive
	modify := t.modifyOther
	alt := t.altScreen
	t.mu.Unlock()
	for _, mode := range restoreModes {
		_ = t.writeRaw([]byte("\x1b[?" + itoa(mode) + "h"))
	}

	if inBand {
		_ = t.writeRaw([]byte(seqInBandResizeDisable))
		t.mu.Lock()
		t.inBandResize = false
		t.mu.Unlock()
	}
	if kitty {
		_ = t.writeRaw([]byte(seqKittyPop))
		t.mu.Lock()
		t.kittyActive = false
		t.mu.Unlock()
	}
	if modify {
		_ = t.writeRaw([]byte(seqModifyOtherKeysDisable))
		t.mu.Lock()
		t.modifyOther = false
		t.mu.Unlock()
	}

	// Stop loops.
	t.stopCh.stop()

	// Every loop uses stop-aware bounded reads/timers; wait for full ownership
	// release before restoring raw mode or allowing a restart.
	t.wg.Wait()

	// Clear probe state (fail CPR waiters).
	t.probe.clear()

	// Platform restore: VT input, then raw mode.
	err := t.restorePlatform()

	// Leave alt screen only if we entered it (never blind).
	if full && alt {
		_ = t.directWrite([]byte(seqLeaveAltScreen))
		t.mu.Lock()
		t.altScreen = false
		t.mu.Unlock()
		setAltScreenActiveLocked(false)
	}

	_ = t.directWrite([]byte(seqShowCursor))

	// Close events if loop did not.
	select {
	case <-t.loopDone:
	default:
		// inputLoop closes events
	}

	t.mu.Lock()
	t.started = false
	t.appearance = AppearanceUnknown
	t.osc99OK = false
	t.privateModeSupport = make(map[int]bool)
	t.hasReportedCols = false
	t.hasReportedRows = false
	t.mu.Unlock()

	return err
}

func (t *Terminal) restorePlatform() error {
	t.mu.Lock()
	win := t.plat.win
	raw := t.plat.raw
	rawFD := t.plat.rawFD
	t.plat.win = nil
	t.mu.Unlock()

	platformRestoreVTInput(win)

	if raw == nil {
		return nil
	}
	err := restoreRaw(rawFD, raw)
	if err == nil {
		t.mu.Lock()
		t.plat.raw = nil
		t.plat.rawFD = 0
		t.mu.Unlock()
	}
	// Keep raw state on failure so retry works.
	return err
}

func (t *Terminal) clearProgressTimer() {
	t.mu.Lock()
	if t.progressStop != nil {
		close(t.progressStop)
		t.progressStop = nil
	}
	t.progressOn = false
	t.mu.Unlock()
}

// inputLoop is the single reader of t.in.
func (t *Terminal) inputLoop(ctx context.Context) {
	defer t.wg.Done()
	defer close(t.loopDone)
	defer close(t.events)

	buf := make([]byte, 4096)
	var draining bool
	var drainDeadline time.Time
	var drainIdle time.Duration
	var drainLast time.Time
	var drainDone chan struct{}

	cb := t.probeCallbacks()

	for {
		// Decoder deadline.
		var timer *time.Timer
		var timerC <-chan time.Time
		if dl, ok := t.dec.Deadline(); ok {
			d := time.Until(dl)
			if d < 0 {
				d = 0
			}
			timer = time.NewTimer(d)
			timerC = timer.C
		}

		// Drain idle check.
		var drainTimerC <-chan time.Time
		var drainTimer *time.Timer
		if draining {
			now := time.Now()
			if now.After(drainDeadline) || now.Sub(drainLast) >= drainIdle {
				draining = false
				if drainDone != nil {
					close(drainDone)
					drainDone = nil
				}
			} else {
				wait := drainIdle - now.Sub(drainLast)
				if end := time.Until(drainDeadline); end < wait {
					wait = end
				}
				if wait < 0 {
					wait = 0
				}
				drainTimer = time.NewTimer(wait)
				drainTimerC = drainTimer.C
			}
		}

		// Non-blocking: process commands and resize first.
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
			return
		case <-t.stopCh.done():
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
			return
		case cmd := <-t.cmdCh:
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
			if d, ok := cmd.(cmdDrain); ok {
				// Disable kitty / modifyOtherKeys before drain.
				t.disableKeyboardProtocols()
				draining = true
				drainDeadline = time.Now().Add(d.max)
				drainIdle = d.idle
				drainLast = time.Now()
				drainDone = d.done
				continue
			}
			t.handleCmd(cmd)
			continue
		case <-t.resizeCh:
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
			t.handleOSResize()
			continue
		case <-timerC:
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
			t.dec.PopDue(time.Now())
			t.emitDecoded(draining)
			continue
		case <-drainTimerC:
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
			// loop will re-check drain condition
			continue
		default:
			if timer != nil {
				timer.Stop()
			}
			if drainTimer != nil {
				drainTimer.Stop()
			}
		}

		// Blocking read with short timeout so we can service cmds/resize/timers.
		timeout := 25 * time.Millisecond
		if dl, ok := t.dec.Deadline(); ok {
			if d := time.Until(dl); d > 0 && d < timeout {
				timeout = d
			}
		}
		n, err := readWithTimeout(t.in, buf, timeout)
		if n > 0 {
			if draining {
				drainLast = time.Now()
				// Discard bytes during drain.
			} else {
				t.dec.Write(buf[:n])
				// Route each decoded raw sequence through probe first.
				// Decoder already emits semantic events; we need probe
				// interception on framed sequences before user delivery.
				// Strategy: use decoder events, but also feed a side framer
				// for probe-only patterns that decoder surfaces as KindRaw.
				t.routeAndEmit(cb, draining)
			}
		}
		if err != nil && err != os.ErrDeadlineExceeded && !isTimeout(err) {
			if err == io.EOF || isClosedErr(err) {
				t.emitEvent(event.ErrorEvent(errInputClosed, nil))
				return
			}
			t.emitEvent(event.ErrorEvent(err, nil))
			// keep running on transient errors
		}
		// Pop decoder timers.
		t.dec.PopDue(time.Now())
		if !draining {
			t.emitDecoded(false)
		}
	}
}

func (t *Terminal) disableKeyboardProtocols() {
	t.mu.Lock()
	kitty := t.kittyActive
	modify := t.modifyOther
	if t.kittyFallbackCancel != nil {
		t.kittyFallbackCancel()
		t.kittyFallbackCancel = nil
	}
	t.mu.Unlock()
	if kitty {
		_ = t.enqueueWrite([]byte(seqKittyPop))
		t.mu.Lock()
		t.kittyActive = false
		t.mu.Unlock()
		if t.dec != nil {
			t.dec.SetKittyActive(false)
		}
	}
	if modify {
		_ = t.enqueueWrite([]byte(seqModifyOtherKeysDisable))
		t.mu.Lock()
		t.modifyOther = false
		t.mu.Unlock()
	}
}

func (t *Terminal) routeAndEmit(cb probeCallbacks, draining bool) {
	// Drain decoder events; intercept probe-shaped Raw (and any) via Raw bytes.
	for {
		ev, ok := t.dec.Next()
		if !ok {
			return
		}
		if draining {
			continue
		}
		// Probe router works on framed sequences. For events that carry Raw,
		// try probe consumption first.
		if len(ev.Raw) > 0 {
			t.mu.Lock()
			inBand := t.inBandResize
			t.mu.Unlock()
			if t.probe.route(ev.Raw, inBand, cb) {
				continue
			}
		}
		t.emitEvent(ev)
	}
}

func (t *Terminal) emitDecoded(draining bool) {
	cb := t.probeCallbacks()
	t.routeAndEmit(cb, draining)
}

func (t *Terminal) emitEvent(ev event.Event) {
	// Backpressure: block until delivered or stop. Never drop.
	select {
	case t.events <- ev:
	case <-t.stopCh.done():
	}
}

func (t *Terminal) handleOSResize() {
	cols, rows, err := termSize(t.out)
	if err != nil {
		return
	}
	wPx, hPx := platformWindowPixels(int(t.out.Fd()))

	t.mu.Lock()
	// Reconcile in-band cache with OS (OS is authoritative on SIGWINCH).
	if t.inBandResize {
		if t.hasReportedCols && cols > 0 && t.reportedCols != cols {
			t.hasReportedCols = false
		}
		if t.hasReportedRows && rows > 0 && t.reportedRows != rows {
			t.hasReportedRows = false
		}
	}
	t.cols = cols
	t.rows = rows
	if wPx > 0 {
		t.widthPx = wPx
	}
	if hPx > 0 {
		t.heightPx = hPx
	}
	sz := event.Size{Cols: cols, Rows: rows, WidthPx: t.widthPx, HeightPx: t.heightPx}
	if t.inBandResize {
		if t.hasReportedCols {
			sz.Cols = t.reportedCols
		}
		if t.hasReportedRows {
			sz.Rows = t.reportedRows
		}
	}
	t.mu.Unlock()

	t.emitEvent(event.ResizeEvent(sz, nil))
}

func (t *Terminal) handleCmd(cmd any) {
	switch c := cmd.(type) {
	case cmdQueryCPR:
		t.doCPR(c)
	case cmdEnableMouse:
		if c.enable {
			_ = t.directWrite([]byte(seqMouseBasicEnable + seqMouseAnyEnable + seqMouseSGREnable))
			t.mu.Lock()
			t.mouseEnabled = true
			t.mu.Unlock()
		} else {
			_ = t.directWrite([]byte(seqMouseDisableAll))
			t.mu.Lock()
			t.mouseEnabled = false
			t.mu.Unlock()
		}
	case cmdSetTitle:
		_ = t.directWrite([]byte("\x1b]0;" + c.title + "\x07"))
	case cmdSetProgress:
		t.setProgressLocked(c.active)
	case cmdNotify:
		t.mu.Lock()
		ok := t.osc99OK
		next := &t.osc99Next
		proto := t.caps.NotifyProtocol
		t.mu.Unlock()
		payload := formatNotification(proto, ok, c.n, next)
		_ = t.directWrite([]byte(payload))
	case cmdEnterAlt:
		if c.enter {
			_ = t.directWrite([]byte(seqEnterAltScreen))
			t.mu.Lock()
			t.altScreen = true
			t.mu.Unlock()
			setAltScreenActiveLocked(true)
			// Re-push kitty flags (per-screen).
			t.mu.Lock()
			seq := t.kittyEnable
			active := t.kittyActive
			t.mu.Unlock()
			if active && seq != "" {
				_ = t.directWrite([]byte(seq))
			}
		} else {
			t.mu.Lock()
			was := t.altScreen
			t.mu.Unlock()
			if was {
				_ = t.directWrite([]byte(seqLeaveAltScreen))
				t.mu.Lock()
				t.altScreen = false
				t.mu.Unlock()
				setAltScreenActiveLocked(false)
			}
		}
	case cmdQueryBG:
		t.queryBackgroundColor()
	case cmdStartProbes:
		t.startProbes()
	case cmdCPRTimeout:
		// Only fail if this waiter is still the outstanding CPR owner.
		if o, ok := t.probe.takeCPROwner(); ok && (c.res == nil || o.cpr == c.res) {
			select {
			case o.cpr <- cprResult{err: errCPRTimeout}:
			default:
			}
		}
	case cmdKittyFallback:
		t.mu.Lock()
		kitty := t.kittyActive
		mod := t.modifyOther
		t.mu.Unlock()
		if !kitty && !mod {
			_ = t.directWrite([]byte(seqModifyOtherKeysEnable))
			t.mu.Lock()
			t.modifyOther = true
			t.mu.Unlock()
		}
	case cmdDrain:
		// handled in inputLoop select
	}
}

func (t *Terminal) setProgressLocked(active bool) {
	if active {
		_ = t.directWrite([]byte(seqProgressActive))
		t.mu.Lock()
		if t.progressStop == nil {
			stop := make(chan struct{})
			t.progressStop = stop
			t.progressOn = true
			t.mu.Unlock()
			t.wg.Add(1)
			go func() {
				defer t.wg.Done()
				ticker := time.NewTicker(time.Duration(progressKeepaliveInterval) * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-stop:
						return
					case <-t.stopCh.done():
						return
					case <-ticker.C:
						_ = t.directWrite([]byte(seqProgressActive))
					}
				}
			}()
		} else {
			t.progressOn = true
			t.mu.Unlock()
		}
	} else {
		t.clearProgressTimer()
		_ = t.directWrite([]byte(seqProgressClear))
	}
}

func (t *Terminal) doCPR(c cmdQueryCPR) {
	timeout := t.opts.CPRTimeout
	if deadline, ok := c.ctx.Deadline(); ok {
		if d := time.Until(deadline); d > 0 && d < timeout {
			timeout = d
		}
	}
	resCh := c.res
	t.probe.cprPending = true
	t.probe.pushOwner(da1Owner{kind: da1CPR, cpr: resCh})
	if err := t.directWrite([]byte(seqCPR)); err != nil {
		t.probe.takeCPROwner()
		select {
		case resCh <- cprResult{err: err}:
		default:
		}
		return
	}
	// Timeout posts back into the input loop so takeCPROwner stays single-threaded.
	cmdCh := t.cmdCh
	stop := t.stopCh.done()
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			select {
			case cmdCh <- cmdCPRTimeout{res: resCh}:
			case <-stop:
			}
		case <-c.ctx.Done():
			select {
			case cmdCh <- cmdCPRTimeout{res: resCh}:
			case <-stop:
			}
		case <-stop:
		}
	}()
}

func (t *Terminal) probeCallbacks() probeCallbacks {
	return probeCallbacks{
		onKittyFlags: func(flags int) {
			t.mu.Lock()
			if t.kittyFallbackCancel != nil {
				t.kittyFallbackCancel()
				t.kittyFallbackCancel = nil
			}
			undoModify := t.modifyOther
			t.mu.Unlock()
			if undoModify {
				_ = t.directWrite([]byte(seqModifyOtherKeysDisable))
				t.mu.Lock()
				t.modifyOther = false
				t.mu.Unlock()
			}
			var seq string
			if flags >= 3 {
				seq = seqKittyPushLevel7
			} else {
				seq = seqKittyPushLevel1
			}
			_ = t.directWrite([]byte(seq))
			t.mu.Lock()
			t.kittyActive = true
			t.kittyEnable = seq
			t.mu.Unlock()
			if t.dec != nil {
				t.dec.SetKittyActive(true)
			}
		},
		onOSC11: func(r, g, b string) {
			mode := luminanceAppearance(r, g, b)
			t.mu.Lock()
			prev := t.appearance
			t.appearance = mode
			// Start queued query once cycle drains.
			queued := t.probe.osc11QueryQueued
			pendingOwner := t.probe.hasOwner(da1OSC11)
			t.mu.Unlock()
			if mode != prev {
				// Surface as KindRaw with appearance marker? Emit resize-less
				// synthetic event: KindRaw carrying "appearance:dark" is weak.
				// Contract Events yields event.Event — use KindRaw with text.
				t.emitEvent(event.Event{
					Kind: event.KindRaw,
					Text: "appearance:" + mode.String(),
					Raw:  []byte("appearance:" + mode.String()),
				})
			}
			if queued && !pendingOwner {
				t.probe.osc11QueryQueued = false
				t.startOsc11Query()
			}
		},
		onAppearanceDSR: func() {
			t.stopOsc11Poll()
			t.mu.Lock()
			if t.mode2031Cancel != nil {
				t.mode2031Cancel()
			}
			ctx, cancel := context.WithCancel(context.Background())
			t.mode2031Cancel = cancel
			t.mu.Unlock()
			t.wg.Add(1)
			go func() {
				defer t.wg.Done()
				timer := time.NewTimer(time.Duration(mode2031DebounceMs) * time.Millisecond)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-t.stopCh.done():
					return
				case <-timer.C:
					select {
					case t.cmdCh <- cmdQueryBG{}:
					case <-t.stopCh.done():
					}
				}
			}()
		},
		onPrivateMode: func(mode int, status string) {
			t.resolvePrivateMode(mode, isPrivateModeSupported(status))
			if isXtermScrollToBottomMode(mode) && isPrivateModeSet(status) {
				t.disableXtermScrollToBottom(mode)
			}
		},
		onPrivateModeMiss: func(mode int) {
			t.resolvePrivateMode(mode, false)
		},
		onOSC99: func(meta, payload string) bool {
			pending := t.probe.osc99PendingID
			if pending == "" {
				return false
			}
			m := parseOsc99KeyValues(meta)
			if m["i"] != pending || m["p"] != "?" {
				return false
			}
			caps := parseOsc99KeyValues(payload)
			t.mu.Lock()
			t.osc99Caps = caps
			t.mu.Unlock()
			p := caps["p"]
			supported := false
			for _, part := range splitComma(p) {
				if part == "title" {
					supported = true
					break
				}
			}
			t.resolveOsc99(pending, supported)
			return true
		},
		onOSC99Miss: func(id string) {
			t.resolveOsc99(id, false)
		},
		onInBandResize: func(rows, cols, yPx, xPx int) {
			t.mu.Lock()
			prevCols, prevRows := t.cols, t.rows
			if t.inBandResize {
				if t.hasReportedCols {
					prevCols = t.reportedCols
				}
				if t.hasReportedRows {
					prevRows = t.reportedRows
				}
			}
			if rows > 0 {
				t.reportedRows = rows
				t.hasReportedRows = true
				t.rows = rows
			}
			if cols > 0 {
				t.reportedCols = cols
				t.hasReportedCols = true
				t.cols = cols
			}
			if cols > 0 && xPx > 0 && rows > 0 && yPx > 0 {
				dims := termcaps.CellDimensions{
					WidthPx:  maxInt(1, (xPx+cols/2)/cols),
					HeightPx: maxInt(1, (yPx+rows/2)/rows),
				}
				// Match OMP Math.round
				dims.WidthPx = maxInt(1, int(float64(xPx)/float64(cols)+0.5))
				dims.HeightPx = maxInt(1, int(float64(yPx)/float64(rows)+0.5))
				t.cells.Set(dims)
				termcaps.SetCellDimensions(dims)
				t.widthPx = xPx
				t.heightPx = yPx
			}
			changed := rows > 0 && cols > 0 && (rows != prevRows || cols != prevCols)
			sz := event.Size{Cols: cols, Rows: rows, WidthPx: t.widthPx, HeightPx: t.heightPx}
			t.mu.Unlock()
			if changed {
				t.emitEvent(event.ResizeEvent(sz, nil))
			}
		},
		onKeyboardMiss: func() {
			t.mu.Lock()
			kitty := t.kittyActive
			mod := t.modifyOther
			cancel := t.kittyFallbackCancel
			t.mu.Unlock()
			if kitty || mod {
				return
			}
			if cancel != nil {
				cancel()
			}
			_ = t.directWrite([]byte(seqModifyOtherKeysEnable))
			t.mu.Lock()
			t.modifyOther = true
			t.mu.Unlock()
		},
		onCPR: func(pos CursorPosition, ok bool, waiter chan cprResult) {
			if waiter == nil {
				return
			}
			var err error
			if !ok {
				err = errCPRTimeout
			}
			select {
			case waiter <- cprResult{pos: pos, err: err}:
			default:
			}
		},
	}
}

func (t *Terminal) resolvePrivateMode(mode int, supported bool) {
	t.mu.Lock()
	if _, exists := t.privateModeSupport[mode]; exists {
		t.mu.Unlock()
		return
	}
	t.privateModeSupport[mode] = supported
	if mode == modeSyncOutput {
		t.caps.SetSynchronizedOutput(supported)
	}
	t.mu.Unlock()
	if mode == modeInBandResize && supported {
		t.enableInBandResize()
	}
	if mode == modeAppearanceNotif && supported {
		t.stopOsc11Poll()
	}
	// Surface as raw capability event.
	msg := "private-mode:" + itoa(mode) + "="
	if supported {
		msg += "1"
	} else {
		msg += "0"
	}
	t.emitEvent(event.Event{Kind: event.KindRaw, Text: msg, Raw: []byte(msg)})
}

func (t *Terminal) enableInBandResize() {
	t.mu.Lock()
	if t.inBandResize || t.dead || t.stopped {
		t.mu.Unlock()
		return
	}
	t.inBandResize = true
	t.mu.Unlock()
	_ = t.directWrite([]byte(seqInBandResizeEnable))
}

func (t *Terminal) disableXtermScrollToBottom(mode int) {
	t.mu.Lock()
	if _, ok := t.xtermScrollRestore[mode]; ok || t.dead || t.stopped {
		t.mu.Unlock()
		return
	}
	t.xtermScrollRestore[mode] = struct{}{}
	t.mu.Unlock()
	_ = t.directWrite([]byte("\x1b[?" + itoa(mode) + "l"))
}

func (t *Terminal) resolveOsc99(id string, supported bool) {
	if t.probe.osc99PendingID != id {
		return
	}
	t.probe.osc99PendingID = ""
	t.probe.osc99Buf = nil
	t.mu.Lock()
	t.osc99OK = supported
	if !supported {
		t.osc99Caps = make(map[string]string)
	}
	t.mu.Unlock()
}

// Helpers

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func sanitizeTitle(title string) string {
	// Strip BEL, ESC, and other C0 that break OSC.
	b := make([]byte, 0, len(title))
	for i := range len(title) {
		c := title[i]
		if c < 0x20 || c == 0x7f {
			continue
		}
		b = append(b, c)
	}
	return string(b)
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if err == os.ErrDeadlineExceeded {
		return true
	}
	type timeout interface{ Timeout() bool }
	if te, ok := err.(timeout); ok && te.Timeout() {
		return true
	}
	return false
}

func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return err == io.ErrClosedPipe || err == os.ErrClosed
}

func isRetryableWriteErr(err error) bool {
	if err == nil {
		return false
	}
	// EAGAIN / temporary — surface to caller without killing the terminal.
	type temporary interface{ Temporary() bool }
	if te, ok := err.(temporary); ok && te.Temporary() {
		return true
	}
	return false
}
