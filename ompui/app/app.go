package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/autocomplete"
	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/editor"
	"github.com/lyc-aon/ratatui-go/ompui/interact"
	"github.com/lyc-aon/ratatui-go/ompui/keymap"
	"github.com/lyc-aon/ratatui-go/ompui/media"
	"github.com/lyc-aon/ratatui-go/ompui/model"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
	ompruntime "github.com/lyc-aon/ratatui-go/ompui/runtime"
	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
	"github.com/lyc-aon/ratatui-go/ompui/view"
)

// Exit codes.
const (
	ExitOK          = 0
	ExitError       = 1
	ExitInterrupted = 130
)

// App is the single-owner frontend shell.
//
// Run owns the serialized event loop. Background work posts commands onto the
// internal queue and never mutates render state concurrently.
type App struct {
	cfg  Config
	keys *keymap.Registry
	clip *ClipboardHelper

	tty   *ttyFiles
	term  *ompruntime.Terminal
	cli   *client.Client
	sub   *client.Subscription
	state *model.State
	sched *renderer.Scheduler

	root       *component.Container
	transcript *view.Transcript
	status     *view.StatusLine
	footer     *view.Footer
	welcome    *view.Welcome
	working    *view.WorkingIndicator
	errBanner  *view.ErrorBanner
	notice     *view.NoticeBanner
	todos      *view.TodoSummary
	subagents  *view.SubagentSummary
	ed         *editor.Editor
	overlays   *interact.OverlayStack
	ac         *autocomplete.Provider
	budget     *media.ImageBudget
	images     *imageCache
	themes     themeBundle
	viewOpts   view.Options // stable; ImageAdapter set once at buildUI

	// Remote component hosts by id + slot mounts.
	remotes              map[string]*component.Remote
	remoteSlots          map[string]protocol.ComponentSlot
	remoteGen            map[string]uint64 // last render request generation
	remoteResultG        map[string]uint64 // last applied result generation
	headerRemote         component.Component
	footerRemote         component.Component
	editorRemote         component.Component // SlotEditor replaces local editor in root
	widgetAbove          []component.Component
	widgetBelow          []component.Component
	protocolOverlays     map[string]interact.OverlayHandle // overlay_mount ids
	extensionWidgets     map[string]*component.Remote
	extensionWidgetSlots map[string]string

	extDialogs map[string]*extDialog

	cmds   chan command
	cancel context.CancelFunc
	ctx    context.Context

	// Serial RPC worker: one goroutine drains ordered jobs (bootstrap + calls).
	rpcJobs chan rpcJob

	shuttingDown atomic.Bool
	lastSigint   time.Time
	lastLeftTap  time.Time
	leftTapCount int

	exitCode    int
	runErr      error
	quitReason  string
	initialSent bool

	needRender   bool
	forceRender  bool
	renderReason renderer.Reason
	width        int
	height       int
	toolsExpand  bool // local override after user ctrl+o
	toolsForced  bool
	title        string
	animFrame    int

	followUpPrefer      bool
	editorDirty         bool  // need editor_state push
	pendingPromptImages []any // loop-owned protocol image objects for the next prompt

	// terminal_input bridge (extension onTerminalInput).
	// Loop-owned only — no mutex; one in-flight + FIFO queue.
	terminalInputActive   bool
	terminalInputSeq      uint64
	terminalInputInFlight *pendingTerminalInput
	terminalInputQueue    []pendingTerminalInput // capped at protocol.MaxTerminalInputQueue
	terminalInputTimer    *time.Timer

	trace io.Writer
}

type rpcJob struct {
	op       string
	fn       func(context.Context) (client.Response, error)
	restore  string
	complete rpcCompletion
}

// New constructs an App. Call Run to start.
func New(cfg Config) *App {
	cfg = cfg.withDefaults()
	keys := keymap.NewRegistry()
	if len(cfg.UserKeyBindings) > 0 {
		keys.SetUserBindings(cfg.UserKeyBindings)
	} else if cfg.ConfigDir != "" {
		if userMap, err := keymap.LoadConfig(cfg.ConfigDir); err == nil && len(userMap) > 0 {
			keys.SetUserBindings(userMap)
		}
	}

	a := &App{
		cfg:                  cfg,
		keys:                 keys,
		clip:                 newClipboardHelper(),
		state:                model.NewState(),
		cmds:                 make(chan command, 256),
		rpcJobs:              make(chan rpcJob, 64),
		remotes:              make(map[string]*component.Remote),
		remoteSlots:          make(map[string]protocol.ComponentSlot),
		remoteGen:            make(map[string]uint64),
		remoteResultG:        make(map[string]uint64),
		protocolOverlays:     make(map[string]interact.OverlayHandle),
		extDialogs:           make(map[string]*extDialog),
		extensionWidgets:     make(map[string]*component.Remote),
		extensionWidgetSlots: make(map[string]string),
		exitCode:             ExitOK,
	}
	if cfg.Trace {
		a.trace = cfg.Stderr
	}
	return a
}

// KeyRegistry returns the app's keybinding action registry.
func (a *App) KeyRegistry() *keymap.Registry {
	return a.keys
}

// Run starts the TTY, core client, bootstraps state, and runs the event loop.
func (a *App) Run(parent context.Context) (exitCode int) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.ctx = ctx
	a.cancel = cancel

	defer func() {
		if r := recover(); r != nil {
			a.emergencyCleanup()
			panic(r)
		}
	}()
	defer func() {
		code := a.shutdown()
		if a.exitCode != ExitOK {
			exitCode = a.exitCode
		} else {
			exitCode = code
		}
	}()

	if err := a.startTerminal(ctx); err != nil {
		a.logf("terminal start: %v", err)
		a.exitCode = ExitError
		a.runErr = err
		fmt.Fprintf(a.cfg.Stderr, "omp-tui: %v\n", err)
		return a.exitCode
	}
	if err := a.startClient(ctx); err != nil {
		a.logf("core start: %v", err)
		a.exitCode = ExitError
		a.runErr = err
		fmt.Fprintf(a.cfg.Stderr, "omp-tui: core start: %v\n", err)
		return a.exitCode
	}

	a.buildUI()
	a.wireSignals(ctx)
	a.startRPCWorker()

	if err := a.bootstrap(ctx); err != nil {
		a.logf("bootstrap: %v", err)
		a.setLocalError("bootstrap: " + err.Error())
	}
	a.drainStartupRPC()
	a.queueInitialPrompts()

	a.requestRender(renderer.ReasonForce)
	a.loop(ctx)
	return a.exitCode
}

// Err returns the last run error, if any.
func (a *App) Err() error { return a.runErr }

// ExitCode returns the process exit code decided by the last Run.
func (a *App) ExitCode() int { return a.exitCode }

func (a *App) startTerminal(ctx context.Context) error {
	tty, err := openTTY(a.cfg.Stdin, a.cfg.Stdout)
	if err != nil {
		return fmt.Errorf("open tty: %w", err)
	}
	a.tty = tty

	opts := ompruntime.Options{
		Env:                termcaps.ProcessEnv(),
		UseProcessPlatform: true,
		EnterAltScreen:     false,
	}
	term, err := ompruntime.New(tty.In, tty.Out, opts)
	if err != nil {
		tty.Close()
		return fmt.Errorf("runtime.New: %w", err)
	}
	if err := term.Start(ctx); err != nil {
		tty.Close()
		return fmt.Errorf("runtime.Start: %w", err)
	}
	a.term = term
	sz := term.Size()
	a.width, a.height = sz.Cols, sz.Rows
	if a.width < 1 {
		a.width = 80
	}
	if a.height < 1 {
		a.height = 24
	}

	caps := renderer.CapsFromSnapshot(term.Capabilities(), opts.Env)
	eng := renderer.New(term, caps)
	if a.cfg.Trace {
		eng.SetTrace(&renderer.WriterTrace{W: a.cfg.Stderr})
	}
	a.sched = renderer.NewScheduler(eng, renderer.DefaultScheduler())
	a.budget = media.NewImageBudget(media.DefaultMaxInlineImages, func() {
		a.post(command{kind: cmdForceRender})
	}, nil)
	a.images = newImageCache(a.budget,
		func() termcaps.ImageProtocol {
			if a.term == nil {
				return ""
			}
			if s := a.term.Capabilities(); s != nil {
				return s.ImageProtocol
			}
			return ""
		},
		func() termcaps.CellDimensions {
			if a.term == nil {
				return termcaps.CellDimensions{}
			}
			return a.term.CellDimensions()
		},
	)
	a.themes = buildTheme(term.Capabilities(), appearanceFromRuntime(term.Appearance()))
	return nil
}

func frontendHello() protocol.HelloPayload {
	return protocol.NewHello(
		protocol.RoleFrontend,
		protocol.CapJSONL,
		protocol.CapLengthPrefix,
		protocol.CapRPC,
		protocol.CapSessionEvents,
		protocol.CapExtensionUI,
		protocol.CapRemoteComponents,
		protocol.CapEditorSync,
		protocol.CapOverlays,
	)
}

func (a *App) startClient(ctx context.Context) error {
	opts := client.Options{
		Command:              a.cfg.Core,
		Stderr:               a.cfg.Stderr,
		ProcessFactory:       a.cfg.ProcessFactory,
		ReadyTimeout:         a.cfg.ReadyTimeout,
		ShutdownTimeout:      a.cfg.ShutdownTimeout,
		SendHelloOnStart:     false,
		Hello:                frontendHello(),
		SubscribeBeforeReady: true,
		EventBuffer:          1024,
	}
	cli, err := client.Start(ctx, opts)
	if err != nil {
		return err
	}
	a.cli = cli
	a.sub = cli.InitialSubscription()
	if a.sub == nil {
		a.sub = cli.Subscribe(0)
	}
	return nil
}

func (a *App) buildUI() {
	if a.themes.theme.Accent == nil {
		a.themes = buildTheme(nil, view.AppearanceDark)
	}
	// Stable Options: ImageAdapter set once — never rebuild each frame.
	a.viewOpts = view.Options{
		ExpandHint:   "ctrl+o",
		ImageAdapter: a.images.Adapter(),
	}
	th := a.themes.theme
	a.transcript = view.NewTranscript(th, a.viewOpts)
	a.status = view.NewStatusLine(th, a.viewOpts)
	a.footer = view.NewFooter(th, a.viewOpts)
	a.footer.SetInfo(defaultFooterInfo(a.cfg.Core.Dir))
	a.welcome = view.NewWelcome(th, a.viewOpts, defaultWelcomeInfo(a.cfg.Core.Dir))
	a.working = view.NewWorkingIndicator(th, a.viewOpts)
	a.errBanner = view.NewErrorBanner(th, a.viewOpts)
	a.notice = view.NewNoticeBanner(th, a.viewOpts)
	a.todos = view.NewTodoSummary(th, a.viewOpts)
	a.subagents = view.NewSubagentSummary(th, a.viewOpts)
	a.overlays = interact.NewOverlayStack()

	base := a.cfg.Core.Dir
	if base == "" {
		if w, err := os.Getwd(); err == nil {
			base = w
		}
	}
	a.ac = autocomplete.New(autocomplete.Options{
		BasePath: base,
		Files:    autocomplete.NewFSSource(),
	})

	a.ed = editor.New(
		editor.WithKeyMatcher(a.keys),
		editor.WithPlaceholder("Type a message…"),
		editor.WithPromptPrefix("> "),
		editor.WithBorder(true),
		editor.WithBorderColor(th.Border),
		editor.WithAutocompleteProvider(a.ac),
		editor.WithOnSubmit(func(text string) { a.submitEditor(text) }),
		editor.WithOnInterrupt(func() { a.handleCtrlC() }),
		editor.WithOnEOF(func() { a.handleCtrlD() }),
		editor.WithOnChange(func(string) {
			a.editorDirty = true
			a.post(command{kind: cmdTick})
		}),
	)
	a.ed.OnAutocompleteUpdate = func() {
		a.post(command{kind: cmdTick})
	}
	a.ed.SetFocused(true)
	a.ed.SetUseTerminalCursor(false)

	a.rebuildRoot()
	a.overlays.SetBaseFocus(a.ed)
}

func (a *App) rebuildRoot() {
	children := make([]component.Component, 0, 16)
	if a.headerRemote != nil {
		children = append(children, a.headerRemote)
	}
	snap := a.state.Snapshot()
	if len(snap.Messages) == 0 && a.welcome != nil {
		children = append(children, a.welcome)
	}
	children = append(children, a.transcript)
	if a.todos != nil {
		children = append(children, a.todos)
	}
	if a.subagents != nil {
		children = append(children, a.subagents)
	}
	children = append(children, a.notice, a.errBanner, a.working, a.status)
	children = append(children, a.widgetAbove...)
	// SlotEditor replaces local editor when mounted.
	if a.editorRemote != nil {
		children = append(children, a.editorRemote)
	} else {
		children = append(children, a.ed)
	}
	children = append(children, a.widgetBelow...)
	if a.footerRemote != nil {
		children = append(children, a.footerRemote)
	} else {
		children = append(children, a.footer)
	}
	a.root = component.NewContainer(children...)
	if a.editorRemote != nil {
		a.root.SetFocusTarget(a.editorRemote)
		a.overlays.SetBaseFocus(a.editorRemote)
	} else {
		a.root.SetFocusTarget(a.ed)
		a.overlays.SetBaseFocus(a.ed)
		a.ed.SetFocused(true)
	}
}

func (a *App) applyThemeAll() {
	th := a.themes.theme
	if a.transcript != nil {
		a.transcript.SetTheme(th)
	}
	setters := []interface{ SetTheme(view.Theme) }{
		a.status, a.footer, a.welcome, a.working,
		a.errBanner, a.notice, a.todos, a.subagents,
	}
	for _, c := range setters {
		if c != nil {
			c.SetTheme(th)
		}

	}
	if a.ed != nil {
		a.ed.SetBorderColor(th.Border)
	}
	if a.root != nil {
		a.root.Invalidate()
	}
}

// drainStartupRPC applies frames captured by the pre-ready subscription before
// queued prompts or terminal input can observe stale extension/theme state.
// Bounded so a noisy peer cannot keep startup from reaching the main loop.
func (a *App) drainStartupRPC() {
	if a.sub == nil {
		return
	}
	for range 1024 {
		select {
		case ev, ok := <-a.sub.C:
			if !ok {
				return
			}
			if ev.Kind == protocol.KindRPCResponse {
				// bootstrap already applied its three synchronous responses.
				continue
			}
			a.handleRPCEvent(ev)
		default:
			return
		}
	}
}

func (a *App) bootstrap(ctx context.Context) error {
	var first error
	call := func(name string, fn func(context.Context) (client.Response, error)) {
		resp, err := fn(ctx)
		if err != nil {
			if first == nil {
				first = fmt.Errorf("%s: %w", name, err)
			}
			return
		}
		a.applyResponse(resp)
	}
	call("get_state", a.cli.GetState)
	call("get_messages", func(c context.Context) (client.Response, error) {
		return a.cli.Call(c, protocol.BuildRPCCommand(protocol.CmdGetMessages, "", nil))
	})
	call("get_available_commands", a.cli.GetAvailableCommands)
	a.syncFromSnapshot(true)
	// Pull configured/custom theme before extension setTheme can race.
	// Bare additive frame — not an RpcCommand waiter. Nonfatal.
	if a.cli != nil {
		if err := a.cli.SendRaw([]byte(`{"v":1,"type":"theme_query","id":"theme-initial"}`)); err != nil {
			a.logf("theme_query initial: %v", err)
		}
	}
	return first
}

// queueInitialPrompts enqueues bootstrap prompts on the serial RPC worker.
// Primary then queuedMessages, strictly ordered.
func (a *App) queueInitialPrompts() {
	if a.initialSent {
		return
	}
	a.initialSent = true
	boot := a.cfg.Bootstrap
	if !boot.HasPrompt() {
		return
	}
	primary := boot.PrimaryMessage()
	images := boot.AllImages()
	if primary != "" || len(images) > 0 {
		msg := primary
		imgs := images
		a.enqueueRPC("prompt", func(ctx context.Context) (client.Response, error) {
			opts := []client.PromptOption{}
			if len(imgs) > 0 {
				opts = append(opts, client.WithPromptImages(imgs...))
			}
			return a.cli.Prompt(ctx, msg, opts...)
		}, "")
	}
	for _, m := range boot.AllQueued() {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		text := m
		a.enqueueRPC("prompt", func(ctx context.Context) (client.Response, error) {
			return a.cli.Prompt(ctx, text, client.WithStreamingBehavior("followUp"))
		}, "")
	}
}

func (a *App) startRPCWorker() {
	go func() {
		for {
			select {
			case <-a.ctx.Done():
				return
			case job, ok := <-a.rpcJobs:
				if !ok {
					return
				}
				ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
				resp, err := job.fn(ctx)
				cancel()
				a.post(command{kind: cmdRPCDone, rpcDone: rpcDone{
					op: job.op, resp: resp, err: err, restore: job.restore, complete: job.complete,
				}})
			}
		}
	}()
}

func (a *App) enqueueRPC(op string, fn func(context.Context) (client.Response, error), restore string) {
	_ = a.enqueueRPCWithCompletion(op, fn, restore, nil)
}

func (a *App) enqueueRPCWithCompletion(
	op string,
	fn func(context.Context) (client.Response, error),
	restore string,
	complete rpcCompletion,
) bool {
	if a.cli == nil || a.ctx == nil {
		return false
	}
	select {
	case a.rpcJobs <- rpcJob{op: op, fn: fn, restore: restore, complete: complete}:
		return true
	case <-a.ctx.Done():
		return false
	}
}

// bgCall routes to the serial RPC worker (no concurrent Calls from app).
func (a *App) bgCall(op string, fn func(context.Context) (client.Response, error), restore string) {
	_ = a.enqueueRPCWithCompletion(op, fn, restore, nil)
}

// bgCallWithCompletion runs complete on the serialized app loop after the RPC
// has finished. The worker goroutine only transports rpcDone back to that loop.
func (a *App) bgCallWithCompletion(
	op string,
	fn func(context.Context) (client.Response, error),
	restore string,
	complete rpcCompletion,
) bool {
	return a.enqueueRPCWithCompletion(op, fn, restore, complete)
}

func (a *App) wireSignals(ctx context.Context) {
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	if runtime.GOOS != "windows" {
		signal.Notify(ch, syscall.SIGHUP)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				signal.Stop(ch)
				return
			case sig, ok := <-ch:
				if !ok {
					return
				}
				a.post(command{kind: cmdSignal, signal: sig.String()})
			}
		}
	}()
}

func (a *App) loop(ctx context.Context) {
	tick := time.NewTicker(a.cfg.TickInterval)
	defer tick.Stop()

	termEvents := a.term.Events()
	var rpcCh <-chan client.Event
	if a.sub != nil {
		rpcCh = a.sub.C
	}
	coreDone := a.cli.Done()

	for {
		if a.needRender || a.forceRender {
			a.paint()
		}
		select {
		case <-ctx.Done():
			a.quitReason = "context"
			return
		case <-coreDone:
			a.handleCoreDied()
			return
		case ev, ok := <-termEvents:
			if !ok {
				a.quitReason = "tty-closed"
				return
			}
			a.handleTermEvent(ev)
		case ev, ok := <-rpcCh:
			if !ok {
				rpcCh = nil
				continue
			}
			a.handleRPCEvent(ev)
		case cmd := <-a.cmds:
			a.handleCommand(cmd)
		case <-a.terminalInputTimeoutC():
			// A broken/slow extension must never hold the terminal hostage.
			// Open the circuit and replay every held event in FIFO order.
			a.failOpenTerminalInput("response timeout")
		case <-tick.C:
			a.onTick()
		}
	}
}

func (a *App) onTick() {
	// AnimationFrame only while working indicator is active.
	if a.working != nil && a.working.Active() {
		a.animFrame++
		if a.animFrame <= 0 {
			a.animFrame = 1
		}
		opts := a.viewOpts
		opts.AnimationFrame = a.animFrame
		a.working.SetOptions(opts)
		a.requestRender(renderer.ReasonUpdate)
	}
	if a.editorDirty {
		a.pushEditorState()
	}
	if a.term != nil && !a.themes.remoteOwned {
		ap := appearanceFromRuntime(a.term.Appearance())
		if ap != a.themes.appearance {
			a.themes = buildTheme(a.term.Capabilities(), ap)
			a.applyThemeAll()
			a.requestRender(renderer.ReasonForce)
		}
	}
}

func (a *App) handleCommand(cmd command) {
	switch cmd.kind {
	case cmdQuit:
		a.quitReason = "quit"
		a.cancel()
	case cmdForceRender:
		a.requestRender(renderer.ReasonForce)
	case cmdRPCDone:
		a.handleRPCDone(cmd.rpcDone)
	case cmdOpenURLDone:
		if cmd.openURLErr != nil {
			a.setLocalError(fmt.Sprintf("open url: %v", cmd.openURLErr))
		} else if cmd.openURL != "" {
			a.setLocalNotice("opened " + cmd.openURL)
		}
		a.requestRender(renderer.ReasonUpdate)
	case cmdSignal:
		a.handleSignal(cmd.signal)
	case cmdCoreDied:
		a.handleCoreDied()
		a.cancel()
	case cmdTick:
		if a.editorDirty {
			a.pushEditorState()
		}
		a.requestRender(renderer.ReasonUpdate)
	}
}

func (a *App) handleSignal(sig string) {
	switch sig {
	case os.Interrupt.String(), "interrupt":
		a.handleCtrlC()
	case syscall.SIGTERM.String(), "terminated":
		a.exitCode = ExitError
		a.quitReason = "sigterm"
		a.cancel()
	case "hangup":
		a.quitReason = "sighup"
		a.cancel()
	default:
		a.logf("signal: %s", sig)
	}
}

func (a *App) handleCoreDied() {
	if a.cli == nil {
		return
	}
	code := a.cli.ExitCode()
	err := a.cli.Err()
	if code > 0 {
		a.exitCode = code
	} else if err != nil {
		a.exitCode = ExitError
	}
	if err != nil {
		a.runErr = err
		a.logf("core exited: %v (code=%d)", err, code)
	}
	a.quitReason = "core-exit"
	a.cancel()
}

func (a *App) applyResponse(resp client.Response) {
	var body []byte
	if len(resp.Raw) > 0 {
		body = resp.Raw
	} else {
		var err error
		body, err = json.Marshal(protocol.RPCResponse{
			ID: resp.ID, Type: protocol.MsgRPCResponse, Command: resp.Command,
			Success: resp.Success, Data: resp.Data, Error: resp.Error,
		})
		if err != nil {
			return
		}
	}
	env, err := protocol.WrapHistorical(body)
	if err != nil {
		return
	}
	_, _ = a.state.Apply(env)
	a.syncFromSnapshot(false)
}

// syncFromSnapshot: SetSnapshot FIRST (adopts ToolsExpanded), then optional override.
func (a *App) syncFromSnapshot(force bool) {
	snap := a.state.Snapshot()
	a.transcript.SetSnapshot(snap) // may SetOptions internally for ToolsExpanded
	if a.toolsForced {
		opts := a.viewOpts
		opts.ToolsExpanded = a.toolsExpand
		a.transcript.SetOptions(opts)
	}
	a.status.SetSnapshot(snap)
	a.footer.SetSnapshot(snap)
	a.welcome.SetSnapshot(snap)
	a.working.SetSnapshot(snap)
	a.errBanner.SetSnapshot(snap)
	a.notice.SetSnapshot(snap)
	a.todos.SetSnapshot(snap)
	a.subagents.SetSnapshot(snap)
	if a.ac != nil {
		a.ac.SetModelCommands(snap.AvailableCommands)
	}
	a.rebuildRoot()
	if force {
		a.transcript.Invalidate()
		a.status.Invalidate()
		a.footer.Invalidate()
	}
}

func utf16OffsetFromByte(text string, byteOffset int) int {
	if byteOffset < 0 {
		byteOffset = 0
	}
	if byteOffset > len(text) {
		byteOffset = len(text)
	}
	for byteOffset > 0 && byteOffset < len(text) && !utf8.RuneStart(text[byteOffset]) {
		byteOffset--
	}
	units := 0
	for _, r := range text[:byteOffset] {
		width := utf16.RuneLen(r)
		if width < 1 {
			width = 1
		}
		units += width
	}
	return units
}

func editorCursorFromUTF16Offset(text string, offset int) editor.CursorPos {
	if offset < 0 {
		offset = 0
	}
	byteOffset := len(text)
	units := 0
	for index, r := range text {
		width := utf16.RuneLen(r)
		if width < 1 {
			width = 1
		}
		if units+width > offset {
			byteOffset = index
			break
		}
		units += width
		if units == offset {
			byteOffset = index + utf8.RuneLen(r)
			break
		}
	}
	prefix := text[:byteOffset]
	line := strings.Count(prefix, "\n")
	lastNewline := strings.LastIndexByte(prefix, '\n')
	column := byteOffset
	if lastNewline >= 0 {
		column = byteOffset - lastNewline - 1
	}
	return editor.CursorPos{Line: line, Col: column}
}

func truncateUTF8(text string, maxBytes int) string {
	if maxBytes < 0 {
		maxBytes = 0
	}
	if len(text) <= maxBytes {
		return text
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}

func marshalEditorStateFrame(text string, cursor int, placeholder string) ([]byte, error) {
	return json.Marshal(struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Cursor      int    `json:"cursor,omitempty"`
		Placeholder string `json:"placeholder,omitempty"`
	}{
		Type:        protocol.MsgEditorState,
		Text:        text,
		Cursor:      cursor,
		Placeholder: placeholder,
	})
}

func (a *App) pushEditorState() {
	a.editorDirty = false
	if a.cli == nil || a.ed == nil {
		return
	}
	text := a.ed.Text()
	const maxPush = 64 << 10
	text = truncateUTF8(text, maxPush)
	cur := a.ed.Cursor()
	offset := 0
	lines := a.ed.Lines()
	for i := 0; i < cur.Line && i < len(lines); i++ {
		offset += len(lines[i]) + 1
	}
	offset += cur.Col
	offset = minInt(offset, len(text))
	body, err := marshalEditorStateFrame(text, utf16OffsetFromByte(text, offset), a.ed.Placeholder())
	if err != nil {
		return
	}
	_ = a.cli.SendRaw(body)
}

func (a *App) isStreaming() bool {
	snap := a.state.Snapshot()
	return snap.Status.Streaming || snap.Status.AgentRunning || snap.Session.IsStreaming
}

func (a *App) setLocalError(msg string) {
	payload, _ := json.Marshal(map[string]any{
		"type": protocol.EventNotice, "level": "error", "message": msg, "source": "omp-tui",
	})
	if env, err := protocol.WrapHistorical(payload); err == nil {
		_, _ = a.state.Apply(env)
	}
	a.syncFromSnapshot(false)
}

func (a *App) setLocalNotice(msg string) {
	payload, _ := json.Marshal(map[string]any{
		"type": protocol.EventNotice, "level": "info", "message": msg, "source": "omp-tui",
	})
	if env, err := protocol.WrapHistorical(payload); err == nil {
		_, _ = a.state.Apply(env)
	}
	a.syncFromSnapshot(false)
}

func (a *App) logf(format string, args ...any) {
	if a.trace == nil {
		return
	}
	fmt.Fprintf(a.trace, "omp-tui: "+format+"\n", args...)
}

func (a *App) post(cmd command) {
	if a.ctx == nil {
		return
	}
	select {
	case a.cmds <- cmd:
	case <-a.ctx.Done():
	default:
		switch cmd.kind {
		case cmdQuit, cmdSignal, cmdCoreDied, cmdForceRender, cmdRPCDone:
			select {
			case a.cmds <- cmd:
			case <-a.ctx.Done():
			case <-time.After(2 * time.Second):
				a.logf("command queue stuck dropping %d", cmd.kind)
			}
		}
	}
}

func (a *App) emergencyCleanup() {
	ompruntime.EmergencyRestore()
	if a.cli != nil {
		_ = a.cli.Close()
	}
	if a.tty != nil {
		a.tty.Close()
	}
}

func (a *App) shutdown() int {
	if !a.shuttingDown.CompareAndSwap(false, true) {
		if a.cli != nil {
			return a.cli.ExitCode()
		}
		return a.exitCode
	}
	a.logf("shutdown (%s)", a.quitReason)

	if a.sched != nil {
		a.sched.Stop()
	}
	// Dispose all remotes with protocol dispose/focus-out (no leak).
	ids := make([]string, 0, len(a.remotes))
	for id := range a.remotes {
		ids = append(ids, id)
	}
	for _, id := range ids {
		a.disposeRemote(id, true)
	}
	for id := range a.extDialogs {
		if a.cli != nil {
			_ = a.cli.ExtensionUIResponseCancelled(id, false)
		}
	}
	if a.overlays != nil {
		a.overlays.DisposeAll()
	}
	if a.ac != nil {
		a.ac.Cancel()
	}
	if a.sub != nil {
		a.sub.Unsubscribe()
		a.sub = nil
	}
	if a.images != nil {
		a.images.Clear()
	}

	code := ExitOK
	if a.cli != nil {
		ctx, cancel := context.WithTimeout(context.Background(), client.DefaultShutdownTimeout)
		_ = a.cli.Shutdown(ctx)
		cancel()
		if c := a.cli.ExitCode(); c > 0 {
			code = c
		} else if err := a.cli.Err(); err != nil && a.exitCode == ExitOK {
			code = ExitError
			a.runErr = err
		}
	}
	if a.term != nil {
		drainCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_ = a.term.DrainInput(drainCtx, 200*time.Millisecond, 30*time.Millisecond)
		cancel()
		if err := a.term.Stop(); err != nil {
			a.logf("terminal stop: %v", err)
			if a.runErr == nil {
				a.runErr = err
			}
			if code == ExitOK {
				code = ExitError
			}
		}
		a.term = nil
	}
	if a.tty != nil {
		a.tty.Close()
		a.tty = nil
	}
	if a.exitCode != ExitOK {
		return a.exitCode
	}
	return code
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
