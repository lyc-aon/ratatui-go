package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/michaelkelly/ratatui-go/ompui/client"
	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

// fakeProcess is an in-memory Bun core stand-in. No shell, no OS child.
type fakeProcess struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter

	started atomic.Bool
	killed  atomic.Bool
	done    chan struct{}
	exit    atomic.Int32
	waitErr error
	mu      sync.Mutex
	onStart func()
}

func newFakeProcess() *fakeProcess {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	p := &fakeProcess{
		stdinR:  inR,
		stdinW:  inW,
		stdoutR: outR,
		stdoutW: outW,
		done:    make(chan struct{}),
	}
	p.exit.Store(-1)
	return p
}

func (p *fakeProcess) Start() error {
	p.started.Store(true)
	if p.onStart != nil {
		p.onStart()
	}
	return nil
}
func (p *fakeProcess) Wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}
func (p *fakeProcess) Kill() error {
	p.killed.Store(true)
	p.finish(1, errors.New("killed"))
	return nil
}
func (p *fakeProcess) Stdin() io.WriteCloser { return p.stdinW }
func (p *fakeProcess) Stdout() io.ReadCloser { return p.stdoutR }
func (p *fakeProcess) PID() int              { return 4242 }
func (p *fakeProcess) ExitCode() int         { return int(p.exit.Load()) }
func (p *fakeProcess) writeLine(s string) {
	_, _ = io.WriteString(p.stdoutW, s)
	if !strings.HasSuffix(s, "\n") {
		_, _ = io.WriteString(p.stdoutW, "\n")
	}
}
func (p *fakeProcess) finish(code int, err error) {
	p.mu.Lock()
	select {
	case <-p.done:
		p.mu.Unlock()
		return
	default:
		p.exit.Store(int32(code))
		p.waitErr = err
		close(p.done)
		p.mu.Unlock()
	}
	_ = p.stdoutW.Close()
	_ = p.stdinR.Close()
}

func startWithFake(t *testing.T, readyLine string, configure func(*fakeProcess, *client.Options)) (*client.Client, *fakeProcess) {
	t.Helper()
	fp := newFakeProcess()
	opts := client.Options{
		Command:         client.Command{Path: "fake-core"},
		ReadyTimeout:    2 * time.Second,
		ShutdownTimeout: time.Second,
		EventBuffer:     8,
		DispatchBuffer:  16,
		ProcessFactory: func(ctx context.Context, spec client.ProcessSpec) (client.Process, error) {
			return fp, nil
		},
	}
	if configure != nil {
		configure(fp, &opts)
	}
	if readyLine != "" {
		fp.onStart = func() {
			go fp.writeLine(readyLine)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	cli, err := client.Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return cli, fp
}

func readCmd(t *testing.T, fp *fakeProcess, timeout time.Duration) map[string]any {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 256)
		for {
			n, err := fp.stdinR.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				if i := bytes.IndexByte(buf, '\n'); i >= 0 {
					ch <- result{line: string(buf[:i])}
					return
				}
			}
			if err != nil {
				ch <- result{err: err}
				return
			}
		}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("read cmd: %v", r.err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(r.line), &obj); err != nil {
			t.Fatalf("cmd json %q: %v", r.line, err)
		}
		return obj
	case <-time.After(timeout):
		t.Fatal("timeout reading command")
		return nil
	}
}

func sendAndReadCmd(t *testing.T, fp *fakeProcess, send func() error) map[string]any {
	t.Helper()
	cmdCh := make(chan map[string]any, 1)
	go func() {
		cmdCh <- readCmd(t, fp, time.Second)
	}()
	if err := send(); err != nil {
		t.Fatal(err)
	}
	select {
	case cmd := <-cmdCh:
		return cmd
	case <-time.After(time.Second):
		t.Fatal("timeout capturing command")
		return nil
	}
}

func TestReadyHistoricalCallAndShutdown(t *testing.T) {
	cli, fp := startWithFake(t, `{"type":"ready"}`, nil)
	if !cli.Ready() {
		t.Fatal("not ready")
	}
	if cli.PeerV1() {
		t.Fatal("historical ready must not mark v1")
	}
	if _, ok := cli.PeerHello(); ok {
		t.Fatal("no peer hello expected")
	}

	sub := cli.Subscribe(4)
	t.Cleanup(sub.Unsubscribe)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Fire Call: fake answers with correlated response.
	done := make(chan client.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetState, "req-1", nil))
		if err != nil {
			errCh <- err
			return
		}
		done <- resp
	}()

	// Read outbound bare command (compat mode: no v1 wrapper).
	var cmd map[string]any
	deadline := time.After(time.Second)
waitCmd:
	for {
		select {
		case <-deadline:
			t.Fatal("no outbound command")
		default:
			cmd = readCmd(t, fp, 200*time.Millisecond)
			if cmd["type"] == protocol.CmdGetState {
				break waitCmd
			}
		}
	}
	if _, hasV := cmd["v"]; hasV {
		t.Fatalf("compat mode must not wrap v1: %v", cmd)
	}
	if cmd["id"] != "req-1" {
		t.Fatalf("id=%v", cmd["id"])
	}

	fp.writeLine(`{"type":"response","id":"req-1","command":"get_state","success":true,"data":{"sessionId":"s1","isStreaming":false}}`)

	select {
	case resp := <-done:
		if !resp.Success || resp.Command != protocol.CmdGetState || resp.ID != "req-1" {
			t.Fatalf("resp=%+v", resp)
		}
	case err := <-errCh:
		t.Fatalf("Call: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Call timeout")
	}

	// Session event delivered ordered on subscription.
	fp.writeLine(`{"type":"agent_start"}`)
	eventDeadline := time.After(time.Second)
	for {
		select {
		case ev := <-sub.C:
			if ev.Envelope.Type != protocol.EventAgentStart {
				continue
			}
			if ev.Kind != protocol.KindSessionEvent || ev.Seq < 1 {
				t.Fatalf("event=%+v kind=%v seq=%d", ev.Envelope, ev.Kind, ev.Seq)
			}
			goto eventReceived
		case <-eventDeadline:
			t.Fatal("no agent_start event")
		}
	}
eventReceived:

	// Shutdown reaps child.
	shCtx, shCancel := context.WithTimeout(context.Background(), time.Second)
	defer shCancel()
	go func() {
		// Simulate clean child exit when stdin closes / shutdown.
		time.AfterFunc(20*time.Millisecond, func() {
			fp.finish(0, nil)
		})
	}()
	if err := cli.Shutdown(shCtx); err != nil {
		// Kill path may surface; ensure no hang and Done closed.
		t.Logf("Shutdown err (may be exit wrap): %v", err)
	}
	select {
	case <-cli.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed")
	}
	if cli.Ready() {
		t.Fatal("Ready after shutdown")
	}
}

func TestV1HelloExchange(t *testing.T) {
	fp := newFakeProcess()
	helloPeer := protocol.MustEnvelope(protocol.MsgHello, "", protocol.NewHello(protocol.RoleCore, protocol.CapJSONL, protocol.CapRPC))
	line, err := protocol.MarshalEnvelopeJSONL(helloPeer)
	if err != nil {
		t.Fatal(err)
	}
	opts := client.Options{
		Command:          client.Command{Path: "fake"},
		ReadyTimeout:     2 * time.Second,
		ShutdownTimeout:  time.Second,
		SendHelloOnStart: true,
		Framing:          protocol.FramingJSONL,
		ProcessFactory: func(ctx context.Context, spec client.ProcessSpec) (client.Process, error) {
			return fp, nil
		},
	}
	fp.onStart = func() {
		go func() {
			_, _ = fp.stdoutW.Write(line)
		}()
	}
	outboundHello := make(chan map[string]any, 1)
	go func() {
		outboundHello <- readCmd(t, fp, time.Second)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cli, err := client.Start(ctx, opts)
	if err != nil {
		t.Fatalf("Start v1: %v", err)
	}
	t.Cleanup(func() {
		fp.finish(0, nil)
		_ = cli.Close()
	})
	if !cli.PeerV1() {
		t.Fatal("expected peer v1")
	}
	h, ok := cli.PeerHello()
	if !ok || h.Role != protocol.RoleCore {
		t.Fatalf("peer hello=%+v ok=%v", h, ok)
	}
	// Local hello should have been written.
	var cmd map[string]any
	select {
	case cmd = <-outboundHello:
	case <-time.After(time.Second):
		t.Fatal("no outbound hello")
	}
	if cmd["type"] != protocol.MsgHello {
		t.Fatalf("first outbound=%v", cmd)
	}
}

func TestReadyTimeout(t *testing.T) {
	fp := newFakeProcess()
	opts := client.Options{
		Command:         client.Command{Path: "fake"},
		ReadyTimeout:    50 * time.Millisecond,
		ShutdownTimeout: 100 * time.Millisecond,
		ProcessFactory: func(ctx context.Context, spec client.ProcessSpec) (client.Process, error) {
			return fp, nil
		},
	}
	ctx := context.Background()
	_, err := client.Start(ctx, opts)
	if err == nil {
		t.Fatal("expected ready timeout")
	}
	if !errors.Is(err, client.ErrReadyTimeout) && !strings.Contains(err.Error(), "ready") {
		t.Fatalf("err=%v", err)
	}
	// Failed start must reap — finish so Wait unblocks if client called it.
	fp.finish(1, errors.New("reaped"))
}

func TestCallContextCancelDropsLateResponse(t *testing.T) {
	cli, fp := startWithFake(t, `{"type":"ready"}`, nil)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdAbort, "late-1", nil))
		errCh <- err
	}()
	_ = readCmd(t, fp, time.Second)
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(time.Second):
		t.Fatal("Call did not return on cancel")
	}
	// Late response must not panic or block a later Call.
	fp.writeLine(`{"type":"response","id":"late-1","command":"abort","success":true}`)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	errCh2 := make(chan error, 1)
	go func() {
		_, err := cli.Call(ctx2, protocol.BuildRPCCommand(protocol.CmdAbort, "late-2", nil))
		errCh2 <- err
	}()
	// Drain until we see late-2 command.
	for {
		cmd := readCmd(t, fp, time.Second)
		if cmd["id"] == "late-2" {
			break
		}
	}
	fp.writeLine(`{"type":"response","id":"late-2","command":"abort","success":true}`)
	select {
	case err := <-errCh2:
		if err != nil {
			t.Fatalf("follow-up Call: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("follow-up Call timeout")
	}
}

func TestSubscribeBackpressureStillDeliversCritical(t *testing.T) {
	cli, fp := startWithFake(t, `{"type":"ready"}`, func(fp *fakeProcess, o *client.Options) {
		o.EventBuffer = 1
		o.DispatchBuffer = 2
	})
	sub := cli.Subscribe(1) // tiny buffer
	t.Cleanup(sub.Unsubscribe)

	// Flood critical session events; must not drop (may block reader briefly).
	const n = 5
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			fp.writeLine(`{"type":"agent_start"}`)
		}
	}()

	got := 0
	deadline := time.After(2 * time.Second)
	for got < n {
		select {
		case <-sub.C:
			got++
		case <-deadline:
			t.Fatalf("only got %d/%d events (dropped?)", got, n)
		}
	}
	wg.Wait()
}

func TestSendHelpersAndRaw(t *testing.T) {
	cli, fp := startWithFake(t, `{"type":"ready"}`, nil)

	cmd := sendAndReadCmd(t, fp, func() error {
		return cli.Send(protocol.BuildRPCCommand(protocol.CmdSteer, "", map[string]any{"message": "go"}))
	})
	if cmd["type"] != protocol.CmdSteer {
		t.Fatalf("steer=%v", cmd)
	}

	cmd = sendAndReadCmd(t, fp, func() error {
		return cli.SendRaw([]byte(`{"type":"abort","id":"raw1"}`))
	})
	if cmd["type"] != "abort" {
		t.Fatalf("raw=%v", cmd)
	}

	cmd = sendAndReadCmd(t, fp, func() error {
		return cli.ExtensionUIResponseValue("e1", "picked")
	})
	if cmd["type"] != protocol.MsgExtensionUIResponse {
		t.Fatalf("ext=%v", cmd)
	}
}

func TestNextIDAndPID(t *testing.T) {
	cli, _ := startWithFake(t, `{"type":"ready"}`, nil)
	a, b := cli.NextID(), cli.NextID()
	if a == "" || a == b {
		t.Fatalf("ids %q %q", a, b)
	}
	if cli.PID() != 4242 {
		t.Fatalf("pid=%d", cli.PID())
	}
}

func TestPromptHelperShapesCommand(t *testing.T) {
	cli, fp := startWithFake(t, `{"type":"ready"}`, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, err := cli.Prompt(ctx, "hello", client.WithPromptID("p9"), client.WithStreamingBehavior("steer"))
		errCh <- err
	}()
	cmd := readCmd(t, fp, time.Second)
	if cmd["type"] != protocol.CmdPrompt || cmd["message"] != "hello" || cmd["id"] != "p9" {
		t.Fatalf("prompt cmd=%v", cmd)
	}
	if cmd["streamingBehavior"] != "steer" {
		t.Fatalf("streamingBehavior=%v", cmd["streamingBehavior"])
	}
	fp.writeLine(`{"type":"response","id":"p9","command":"prompt","success":true,"data":{"agentInvoked":true}}`)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("prompt timeout")
	}
}

func TestSubscribeBeforeReadyCapturesImmediateFrames(t *testing.T) {
	cli, _ := startWithFake(t, "", func(fp *fakeProcess, opts *client.Options) {
		opts.SubscribeBeforeReady = true
		fp.onStart = func() {
			go func() {
				fp.writeLine(`{"type":"ready"}`)
				fp.writeLine(`{"type":"theme_sync","name":"dark","appearance":"dark"}`)
			}()
		}
	})
	sub := cli.InitialSubscription()
	if sub == nil {
		t.Fatal("missing initial subscription")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case ev, ok := <-sub.C:
			if !ok {
				t.Fatal("initial subscription closed")
			}
			if ev.Envelope.Type == protocol.MsgThemeSync {
				return
			}
		case <-deadline:
			t.Fatal("theme_sync raced the initial subscription")
		}
	}
}
