package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

// Response is the typed view of a correlated RPC response delivered to Call.
type Response struct {
	ID      string
	Command string
	Success bool
	Data    json.RawMessage
	Error   string
	// Raw is the full response JSON object (no re-encode needed to forward).
	Raw protocol.RawPayload
	// Envelope is the normalized envelope the response arrived in.
	Envelope protocol.Envelope
}

// Client is a live connection to one Bun OMP core process.
//
// Zero value is invalid; obtain one via [Start].
// All exported methods are safe for concurrent use unless noted.
type Client struct {
	opts Options

	proc   Process
	stdin  io.WriteCloser
	stdout io.ReadCloser
	writer *writer

	ids *protocol.IDGenerator

	startCtx    context.Context
	startCancel context.CancelFunc
	closed      atomic.Bool
	ready       atomic.Bool
	peerV1      atomic.Bool
	peerHello   protocol.HelloPayload
	helloMu     sync.RWMutex

	readyCh   chan struct{}
	readyOnce sync.Once
	readyErr  atomicError

	pendingMu sync.Mutex
	pending   map[string]*pendingCall

	seq          atomic.Uint64
	dispatchCh   chan Event
	subMu        sync.Mutex
	subs         map[uint64]*Subscription
	subSeq       atomic.Uint64
	initialSub   *Subscription
	dispatchDone chan struct{}
	readerDone   chan struct{}
	waitDone     chan struct{}
	readerErr    atomicError
	exitErr      atomicError
	exitCode     atomic.Int32
	shutdownOnce sync.Once
	closeOnce    sync.Once
	fatalOnce    sync.Once
}

type pendingCall struct {
	id     string
	ch     chan Response // capacity 1
	cancel context.CancelCauseFunc
}

type errorValue struct {
	err error
}

// atomicError retains the first non-nil error. Wrapping the interface in a
// stable pointer type avoids atomic.Value's concrete-type restriction when
// different error implementations race during startup and process exit.
type atomicError struct {
	value atomic.Pointer[errorValue]
}

func (a *atomicError) Load() error {
	if value := a.value.Load(); value != nil {
		return value.err
	}
	return nil
}

func (a *atomicError) Store(err error) {
	if err != nil {
		a.value.CompareAndSwap(nil, &errorValue{err: err})
	}
}

// Start spawns the core process (or uses ProcessFactory), wires pipes, starts
// the reader/dispatcher, and blocks until the peer is ready or ctx/ReadyTimeout
// expires.
//
// On success the returned Client is ready for Call/Send/Subscribe.
// On failure any partial process is cleaned up (Kill + Wait) so no zombies.
func Start(ctx context.Context, opts Options) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	o := opts.withDefaults()

	// Process lifetime is bound to startCtx so Shutdown/fatal can cancel the
	// CommandContext and ensure no orphan after Kill races.
	startCtx, startCancel := context.WithCancel(context.Background())

	spec := ProcessSpec{
		Command: o.Command,
		Stderr:  o.Stderr,
	}
	proc, err := o.ProcessFactory(startCtx, spec)
	if err != nil {
		startCancel()
		return nil, err
	}
	if proc == nil {
		startCancel()
		return nil, ErrNoProcess
	}

	if err := proc.Start(); err != nil {
		startCancel()
		_ = proc.Kill()
		_ = proc.Wait()
		return nil, err
	}

	stdin := proc.Stdin()
	stdout := proc.Stdout()
	if stdin == nil || stdout == nil {
		startCancel()
		_ = proc.Kill()
		_ = proc.Wait()
		return nil, ErrNoProcess
	}

	enc := protocol.NewEncoder(stdin)
	w := newWriter(enc, o.Framing, o.OnRawWrite)

	c := &Client{
		opts:         o,
		proc:         proc,
		stdin:        stdin,
		stdout:       stdout,
		writer:       w,
		ids:          o.IDs,
		startCtx:     startCtx,
		startCancel:  startCancel,
		readyCh:      make(chan struct{}),
		pending:      make(map[string]*pendingCall),
		dispatchCh:   make(chan Event, o.DispatchBuffer),
		subs:         make(map[uint64]*Subscription),
		dispatchDone: make(chan struct{}),
		readerDone:   make(chan struct{}),
		waitDone:     make(chan struct{}),
	}
	c.exitCode.Store(-1)
	if o.SubscribeBeforeReady {
		c.initialSub = c.Subscribe(o.EventBuffer)
	}

	go c.waitLoop()
	go c.dispatchLoop()
	go c.readLoop()

	if o.SendHelloOnStart {
		if err := c.writer.writeHello(o.Hello); err != nil {
			_ = c.cleanupFailedStart(fmt.Errorf("send hello: %w", err))
			return nil, c.startErr()
		}
	}

	if err := c.awaitReady(ctx, o.ReadyTimeout); err != nil {
		_ = c.cleanupFailedStart(err)
		return nil, err
	}
	return c, nil
}

// cleanupFailedStart reaps the child after a failed Start so callers never
// observe zombies or a half-live Client.
func (c *Client) cleanupFailedStart(cause error) error {
	c.readyErr.Store(cause)
	c.markReady()
	c.fatal(cause)
	if c.proc != nil {
		_ = c.proc.Kill()
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	select {
	case <-c.waitDone:
	case <-time.After(2 * time.Second):
		if c.proc != nil {
			_ = c.proc.Kill()
		}
		<-c.waitDone
	}
	c.finishClose()
	return cause
}

func (c *Client) awaitReady(ctx context.Context, timeout time.Duration) error {
	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}
	select {
	case <-c.readyCh:
		if err := c.readyErr.Load(); err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.waitDone:
		if err := c.readyErr.Load(); err != nil {
			return err
		}
		if err := c.exitErr.Load(); err != nil {
			return err
		}
		return fmt.Errorf("%w: exited before ready", ErrChildExit)
	case <-timer:
		return ErrReadyTimeout
	}
}

func (c *Client) startErr() error {
	if err := c.readyErr.Load(); err != nil {
		return err
	}
	return ErrNotReady
}

func (c *Client) markReady() {
	c.readyOnce.Do(func() {
		c.ready.Store(true)
		close(c.readyCh)
	})
}

// Ready reports whether startup negotiation completed successfully.
func (c *Client) Ready() bool {
	return c != nil && c.ready.Load() && !c.closed.Load() && c.readyErr.Load() == nil
}

// PeerHello returns the peer hello when speaking v1. ok is false in historical
// Bun compatibility mode (ready only, no hello).
func (c *Client) PeerHello() (protocol.HelloPayload, bool) {
	if c == nil {
		return protocol.HelloPayload{}, false
	}
	c.helloMu.RLock()
	defer c.helloMu.RUnlock()
	if !c.peerV1.Load() {
		return protocol.HelloPayload{}, false
	}
	return c.peerHello, true
}

// PeerV1 reports whether the peer completed a v1 hello exchange.
func (c *Client) PeerV1() bool {
	return c != nil && c.peerV1.Load()
}

// PID returns the child process id, or 0.
func (c *Client) PID() int {
	if c == nil || c.proc == nil {
		return 0
	}
	return c.proc.PID()
}

// NextID returns a fresh correlation id from the client generator.
func (c *Client) NextID() string {
	if c == nil || c.ids == nil {
		return protocol.NextID()
	}
	return c.ids.Next()
}

// ---------------------------------------------------------------------------
// Calls / sends
// ---------------------------------------------------------------------------

// Call sends a command and waits for the matching response or ctx cancel.
//
// cmd.ID is filled when empty. The wire form in compatibility mode is the bare
// command JSON object (no v1 envelope). Concurrent Calls are supported; each
// correlates on its id.
//
// On ctx cancel the pending slot is removed so a late response is discarded
// (not delivered to another caller). Cancellation does not abort the in-flight
// core command unless the caller also sends an abort command.
func (c *Client) Call(ctx context.Context, cmd protocol.RPCCommand) (Response, error) {
	if c == nil || c.closed.Load() {
		return Response{}, ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return Response{}, ErrNotReady
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cmd.Type == "" {
		return Response{}, fmt.Errorf("%w: command missing type", protocol.ErrInvalidEnvelope)
	}
	if cmd.ID == "" {
		cmd.ID = c.NextID()
	}

	pctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	p := &pendingCall{
		id:     cmd.ID,
		ch:     make(chan Response, 1),
		cancel: cancel,
	}
	c.pendingMu.Lock()
	if c.closed.Load() {
		c.pendingMu.Unlock()
		return Response{}, ErrClosed
	}
	if _, exists := c.pending[cmd.ID]; exists {
		c.pendingMu.Unlock()
		return Response{}, fmt.Errorf("%w: %s", ErrDuplicateRequestID, cmd.ID)
	}
	c.pending[cmd.ID] = p
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		if cur, ok := c.pending[cmd.ID]; ok && cur == p {
			delete(c.pending, cmd.ID)
		}
		c.pendingMu.Unlock()
	}()

	if err := c.writer.writeCommand(cmd); err != nil {
		return Response{}, err
	}

	select {
	case resp := <-p.ch:
		if !resp.Success {
			return resp, &RPCError{
				Command:  resp.Command,
				Message:  resp.Error,
				Response: resp,
			}
		}
		return resp, nil
	case <-pctx.Done():
		err := context.Cause(pctx)
		if err == nil {
			err = pctx.Err()
		}
		return Response{}, err
	case <-c.waitDone:
		if err := c.exitErr.Load(); err != nil {
			return Response{}, err
		}
		if err := c.readerErr.Load(); err != nil {
			return Response{}, err
		}
		return Response{}, ErrClosed
	}
}

// Send fires a command without waiting for a response.
// An id is still assigned when empty so the core can echo it on events.
// Late responses for this id are discarded (no pending waiter).
func (c *Client) Send(cmd protocol.RPCCommand) error {
	if c == nil || c.closed.Load() {
		return ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return ErrNotReady
	}
	if cmd.Type == "" {
		return fmt.Errorf("%w: command missing type", protocol.ErrInvalidEnvelope)
	}
	if cmd.ID == "" {
		cmd.ID = c.NextID()
	}
	return c.writer.writeCommand(cmd)
}

// SendRaw writes pre-marshaled JSON bytes to the core without re-encoding.
// body must be a complete JSON object. A trailing newline is added in JSONL
// mode. Prefer typed helpers when possible.
func (c *Client) SendRaw(body []byte) error {
	if c == nil || c.closed.Load() {
		return ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return ErrNotReady
	}
	return c.writer.writePreMarshaled(body)
}

// ReplyExtensionUI sends an extension_ui_response to the core.
func (c *Client) ReplyExtensionUI(resp protocol.ExtensionUIResponse) error {
	if c == nil || c.closed.Load() {
		return ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return ErrNotReady
	}
	if resp.ID == "" {
		return fmt.Errorf("%w: extension_ui_response missing id", protocol.ErrInvalidEnvelope)
	}
	return c.writer.writeExtensionUIResponse(resp)
}

// SendHostToolResult sends a host_tool_result frame.
func (c *Client) SendHostToolResult(res protocol.HostToolResult) error {
	if c == nil || c.closed.Load() {
		return ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return ErrNotReady
	}
	return c.writer.writeHostToolResult(res)
}

// SendHostToolUpdate sends a host_tool_update frame.
func (c *Client) SendHostToolUpdate(u protocol.HostToolUpdate) error {
	if c == nil || c.closed.Load() {
		return ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return ErrNotReady
	}
	return c.writer.writeHostToolUpdate(u)
}

// SendHostURIResult sends a host_uri_result frame.
func (c *Client) SendHostURIResult(res protocol.HostURIResult) error {
	if c == nil || c.closed.Load() {
		return ErrClosed
	}
	if !c.ready.Load() || c.readyErr.Load() != nil {
		return ErrNotReady
	}
	return c.writer.writeHostURIResult(res)
}

// ---------------------------------------------------------------------------
// Subscriptions
// ---------------------------------------------------------------------------

// Subscribe registers an ordered event listener.
// The returned Subscription's channel is closed on Unsubscribe or client close.
// buffer <= 0 uses Options.EventBuffer.
//
// Delivery is ordered with respect to the reader. Critical frames never drop;
// a full subscriber channel blocks the dispatcher (applying backpressure back
// to the reader) until the subscriber drains, unsubscribes, or the client closes.
func (c *Client) Subscribe(buffer int) *Subscription {
	if c == nil {
		ch := make(chan Event)
		close(ch)
		return &Subscription{C: ch, ch: ch, quit: make(chan struct{})}
	}
	if buffer <= 0 {
		buffer = c.opts.EventBuffer
	}
	id := c.subSeq.Add(1)
	ch := make(chan Event, buffer)
	s := &Subscription{
		C:      ch,
		id:     id,
		client: c,
		ch:     ch,
		quit:   make(chan struct{}),
	}
	c.subMu.Lock()
	if c.closed.Load() {
		c.subMu.Unlock()
		s.closeCh()
		return s
	}
	c.subs[id] = s
	c.subMu.Unlock()
	return s
}

// InitialSubscription returns the subscription created by
// Options.SubscribeBeforeReady, or nil when the option was false.
func (c *Client) InitialSubscription() *Subscription {
	if c == nil {
		return nil
	}
	return c.initialSub
}

func (c *Client) unsubscribe(s *Subscription) {
	if c == nil || s == nil {
		return
	}
	c.subMu.Lock()
	cur, ok := c.subs[s.id]
	if ok && cur == s {
		delete(c.subs, s.id)
	}
	c.subMu.Unlock()
	// Signal quit first so any blocked deliverOne unblocks, then close C.
	s.closeCh()
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Shutdown requests a graceful stop: optional v1 shutdown frame, stdin close,
// wait for child exit up to ShutdownTimeout, then Kill if needed.
// Idempotent. Always reaps the child (no zombies).
func (c *Client) Shutdown(ctx context.Context) error {
	if c == nil {
		return ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.shutdownOnce.Do(func() {
		_ = c.writer.writeShutdown(protocol.ShutdownPayload{Reason: "host_shutdown"})
		c.writer.close()
		if c.stdin != nil {
			_ = c.stdin.Close()
		}

		timeout := c.opts.ShutdownTimeout
		var timer <-chan time.Time
		if timeout > 0 {
			t := time.NewTimer(timeout)
			defer t.Stop()
			timer = t.C
		}

		select {
		case <-c.waitDone:
		case <-ctx.Done():
			_ = c.proc.Kill()
			c.startCancel()
			<-c.waitDone
		case <-timer:
			_ = c.proc.Kill()
			c.startCancel()
			<-c.waitDone
		}

		c.finishClose()
	})
	if err := c.exitErr.Load(); err != nil {
		return err
	}
	return nil
}

// Close is an alias for Shutdown with a background context.
func (c *Client) Close() error {
	return c.Shutdown(context.Background())
}

// Wait blocks until the child exits and returns the exit error (nil on clean 0).
// After Wait returns the client is fully closed.
func (c *Client) Wait() error {
	if c == nil {
		return ErrClosed
	}
	<-c.waitDone
	c.finishClose()
	if err := c.exitErr.Load(); err != nil {
		return err
	}
	return nil
}

// Err returns the terminal error if the client has failed or the child has
// exited non-cleanly; nil while still running or after a clean exit.
func (c *Client) Err() error {
	if c == nil {
		return ErrClosed
	}
	if err := c.readerErr.Load(); err != nil {
		return err
	}
	if err := c.exitErr.Load(); err != nil {
		return err
	}
	if err := c.readyErr.Load(); err != nil {
		return err
	}
	return nil
}

// ExitCode returns the child exit code after Wait/Shutdown, or -1 if unknown.
func (c *Client) ExitCode() int {
	if c == nil {
		return -1
	}
	return int(c.exitCode.Load())
}

// Done returns a channel closed when the child has been reaped.
func (c *Client) Done() <-chan struct{} {
	if c == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return c.waitDone
}

func (c *Client) finishClose() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.startCancel()

		c.pendingMu.Lock()
		for id, p := range c.pending {
			if p.cancel != nil {
				p.cancel(ErrClosed)
			}
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()

		// Wait for reader to finish and close dispatchCh, then dispatcher.
		<-c.readerDone
		<-c.dispatchDone

		c.subMu.Lock()
		for id, s := range c.subs {
			s.closeCh()
			delete(c.subs, id)
		}
		c.subMu.Unlock()
	})
}

// fatal records a terminal fault, cancels pending calls, and kicks shutdown.
func (c *Client) fatal(err error) {
	if err == nil {
		return
	}
	c.fatalOnce.Do(func() {
		c.readerErr.Store(err)
		if !c.ready.Load() {
			c.readyErr.Store(err)
			c.markReady()
		}
		c.pendingMu.Lock()
		for _, p := range c.pending {
			if p.cancel != nil {
				p.cancel(err)
			}
		}
		c.pendingMu.Unlock()
		if c.stdin != nil {
			_ = c.stdin.Close()
		}
		if c.writer != nil {
			c.writer.close()
		}
	})
}

// ---------------------------------------------------------------------------
// Reader / dispatcher / waiter
// ---------------------------------------------------------------------------

func (c *Client) waitLoop() {
	err := c.proc.Wait()
	code := c.proc.ExitCode()
	c.exitCode.Store(int32(code))
	if err != nil {
		c.exitErr.Store(&ExitError{Err: err, Code: code})
	} else if code != 0 {
		c.exitErr.Store(&ExitError{Code: code})
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	close(c.waitDone)

	if err != nil || code != 0 {
		exit := c.exitErr.Load()
		if exit == nil {
			exit = ErrChildExit
		}
		c.fatal(exit)
	} else {
		// Clean exit: cancel pending callers without recording a fault in readerErr.
		c.cancelPending(ErrClosed)
		if c.stdin != nil {
			_ = c.stdin.Close()
		}
		if c.writer != nil {
			c.writer.close()
		}
	}
	if !c.ready.Load() {
		if err := c.exitErr.Load(); err != nil {
			c.readyErr.Store(err)
		} else {
			c.readyErr.Store(fmt.Errorf("%w: exited before ready", ErrChildExit))
		}
		c.markReady()
	}
}

func (c *Client) cancelPending(err error) {
	c.pendingMu.Lock()
	for _, p := range c.pending {
		if p.cancel != nil {
			p.cancel(err)
		}
	}
	c.pendingMu.Unlock()
}

func (c *Client) readLoop() {
	defer func() {
		close(c.dispatchCh)
		close(c.readerDone)
	}()

	// Framing is fixed at Start. Default and Bun compat = JSONL.
	// Length-prefix only when the caller explicitly opted in via
	// FramingLengthPrefix + SendHelloOnStart.
	useLP := c.opts.Framing == protocol.FramingLengthPrefix && c.opts.SendHelloOnStart
	if useLP {
		c.readLoopLengthPrefix()
		return
	}
	c.readLoopJSONL()
}

func (c *Client) readLoopJSONL() {
	dec := protocol.NewJSONLDecoder(c.stdout)
	for {
		raw, err := dec.DecodeRaw()
		if err != nil {
			c.onReadError(err)
			return
		}
		if c.opts.OnRawRead != nil {
			c.opts.OnRawRead(raw)
		}
		env, err := protocol.ParseEnvelope(raw)
		if err != nil {
			c.onReadError(err)
			c.emit(Event{
				Kind: protocol.KindError,
				Raw:  append(protocol.RawPayload(nil), raw...),
				Err:  err,
			})
			return
		}
		if err := env.CheckMajor(); err != nil {
			c.onReadError(err)
			c.emit(Event{
				Kind:     protocol.KindError,
				Envelope: env,
				Raw:      append(protocol.RawPayload(nil), raw...),
				Err:      err,
			})
			return
		}
		if err := c.handleFrame(env, raw); err != nil {
			c.onReadError(err)
			c.emit(Event{
				Kind:     protocol.KindError,
				Envelope: env,
				Raw:      append(protocol.RawPayload(nil), raw...),
				Err:      err,
			})
			return
		}
	}
}

func (c *Client) readLoopLengthPrefix() {
	dec := protocol.NewDecoder(c.stdout)
	for {
		raw, err := dec.DecodeRaw()
		if err != nil {
			c.onReadError(err)
			return
		}
		if c.opts.OnRawRead != nil {
			c.opts.OnRawRead(raw)
		}
		env, err := protocol.ParseEnvelope(raw)
		if err != nil {
			c.onReadError(err)
			c.emit(Event{
				Kind: protocol.KindError,
				Raw:  append(protocol.RawPayload(nil), raw...),
				Err:  err,
			})
			return
		}
		if err := env.CheckMajor(); err != nil {
			c.onReadError(err)
			c.emit(Event{
				Kind:     protocol.KindError,
				Envelope: env,
				Raw:      append(protocol.RawPayload(nil), raw...),
				Err:      err,
			})
			return
		}
		if err := c.handleFrame(env, raw); err != nil {
			c.onReadError(err)
			c.emit(Event{
				Kind:     protocol.KindError,
				Envelope: env,
				Raw:      append(protocol.RawPayload(nil), raw...),
				Err:      err,
			})
			return
		}
	}
}

func (c *Client) onReadError(err error) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrClosed) {
		if !c.ready.Load() {
			c.readyErr.Store(fmt.Errorf("%w: eof before ready", ErrChildExit))
			c.markReady()
		}
		// EOF after a live session is normal (stdin closed / process exit).
		// Prefer the real exit error when the waiter already recorded one;
		// otherwise cancel pending without stamping readerErr as a fault.
		if err := c.exitErr.Load(); err != nil {
			c.fatal(err)
		} else {
			c.cancelPending(ErrClosed)
			if c.writer != nil {
				c.writer.close()
			}
		}
		return
	}
	c.fatal(err)
	if !c.ready.Load() {
		c.readyErr.Store(err)
		c.markReady()
	}
}

func (c *Client) handleFrame(env protocol.Envelope, raw []byte) error {
	rawCopy := append(protocol.RawPayload(nil), raw...)

	if !c.ready.Load() {
		switch env.Type {
		case TypeReady:
			c.markReady()
			c.emit(Event{
				Seq:      c.seq.Add(1),
				Kind:     protocol.KindUnknown,
				Envelope: env,
				Raw:      rawCopy,
			})
			return nil
		case protocol.MsgHello:
			return c.handlePeerHello(env, rawCopy)
		default:
			// Core is speaking without a ready banner (or we already sent hello).
			// Accept the stream as live and route the frame.
			c.markReady()
		}
	} else if env.Type == protocol.MsgHello {
		return c.handlePeerHello(env, rawCopy)
	}

	kind := protocol.Classify(env)

	if kind == protocol.KindRPCResponse {
		resp, err := decodeResponse(env, rawCopy)
		if err != nil {
			c.emit(Event{
				Seq:      c.seq.Add(1),
				Kind:     kind,
				Envelope: env,
				Raw:      rawCopy,
				Err:      err,
			})
			return nil
		}
		_ = c.deliverPending(resp)
		rr := protocol.RPCResponse{
			ID:      resp.ID,
			Type:    protocol.MsgRPCResponse,
			Command: resp.Command,
			Success: resp.Success,
			Data:    resp.Data,
			Error:   resp.Error,
			Raw:     resp.Raw,
		}
		c.emit(Event{
			Seq:      c.seq.Add(1),
			Kind:     kind,
			Envelope: env,
			Raw:      rawCopy,
			Response: &rr,
		})
		return nil
	}

	if kind == protocol.KindError {
		var ep protocol.ErrorPayload
		if len(env.Payload) > 0 {
			_ = protocol.DecodePayload(env, &ep)
		} else {
			_ = json.Unmarshal(rawCopy, &ep)
		}
		msg := ep.Message
		if msg == "" {
			msg = "peer error"
		}
		c.emit(Event{
			Seq:      c.seq.Add(1),
			Kind:     kind,
			Envelope: env,
			Raw:      rawCopy,
			Err:      fmt.Errorf("%w: %s", ErrProtocol, msg),
		})
		if ep.Fatal {
			return fmt.Errorf("%w: %s", ErrProtocol, msg)
		}
		return nil
	}

	if kind == protocol.KindShutdown {
		c.emit(Event{
			Seq:      c.seq.Add(1),
			Kind:     kind,
			Envelope: env,
			Raw:      rawCopy,
		})
		if c.stdin != nil {
			_ = c.stdin.Close()
		}
		return nil
	}

	c.emit(Event{
		Seq:      c.seq.Add(1),
		Kind:     kind,
		Envelope: env,
		Raw:      rawCopy,
	})
	return nil
}

func (c *Client) handlePeerHello(env protocol.Envelope, raw protocol.RawPayload) error {
	var peer protocol.HelloPayload
	if err := protocol.DecodePayload(env, &peer); err != nil {
		if err2 := json.Unmarshal(raw, &peer); err2 != nil {
			if err3 := json.Unmarshal(env.HistoricalPayload(), &peer); err3 != nil {
				return fmt.Errorf("%w: decode hello: %v", ErrProtocol, err)
			}
		}
	}
	if err := protocol.AcceptHello(peer); err != nil {
		_ = c.writer.writeEnvelope(protocol.MustEnvelope(protocol.MsgError, "", protocol.ErrorPayload{
			Code:    protocol.ErrCodeMajorMismatch,
			Message: err.Error(),
			Fatal:   true,
		}))
		c.readyErr.Store(err)
		c.markReady()
		return err
	}
	c.helloMu.Lock()
	c.peerHello = peer
	c.helloMu.Unlock()
	c.peerV1.Store(true)
	c.writer.setPeerV1(true)

	if !c.opts.SendHelloOnStart {
		if err := c.writer.writeHello(c.opts.Hello); err != nil {
			return fmt.Errorf("reply hello: %w", err)
		}
	}

	c.markReady()
	c.emit(Event{
		Seq:      c.seq.Add(1),
		Kind:     protocol.KindHello,
		Envelope: env,
		Raw:      raw,
	})
	return nil
}

func decodeResponse(env protocol.Envelope, raw []byte) (Response, error) {
	var rr protocol.RPCResponse
	src := raw
	if len(src) == 0 {
		src = env.HistoricalPayload()
	}
	if len(src) == 0 && len(env.Payload) > 0 {
		src = env.Payload
	}
	if err := json.Unmarshal(src, &rr); err != nil {
		return Response{}, fmt.Errorf("%w: response: %v", protocol.ErrInvalidJSON, err)
	}
	return Response{
		ID:       rr.ID,
		Command:  rr.Command,
		Success:  rr.Success,
		Data:     rr.Data,
		Error:    rr.Error,
		Raw:      append(protocol.RawPayload(nil), src...),
		Envelope: env,
	}, nil
}

func (c *Client) deliverPending(resp Response) bool {
	if resp.ID == "" {
		return false
	}
	c.pendingMu.Lock()
	p, ok := c.pending[resp.ID]
	if ok {
		delete(c.pending, resp.ID)
	}
	c.pendingMu.Unlock()
	if !ok || p == nil {
		return false
	}
	select {
	case p.ch <- resp:
		return true
	default:
		return false
	}
}

// emit pushes an event onto the bounded dispatch queue (single producer: reader).
// Critical frames block until there is room — never silently dropped.
func (c *Client) emit(ev Event) {
	select {
	case c.dispatchCh <- ev:
		return
	default:
	}
	select {
	case c.dispatchCh <- ev:
	case <-c.waitDone:
	}
}

func (c *Client) dispatchLoop() {
	defer close(c.dispatchDone)
	for ev := range c.dispatchCh {
		c.fanout(ev)
	}
}

func (c *Client) fanout(ev Event) {
	c.subMu.Lock()
	if len(c.subs) == 0 {
		c.subMu.Unlock()
		return
	}
	list := make([]*Subscription, 0, len(c.subs))
	for _, s := range c.subs {
		list = append(list, s)
	}
	c.subMu.Unlock()

	for _, s := range list {
		deliverOne(s, ev, c.waitDone)
	}
}

func deliverOne(s *Subscription, ev Event, waitDone <-chan struct{}) {
	if s == nil {
		return
	}
	_ = s.trySend(ev, waitDone)
}
