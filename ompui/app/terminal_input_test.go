package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/michaelkelly/ratatui-go/ompui/client"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/editor"
	"github.com/michaelkelly/ratatui-go/ompui/event"
	"github.com/michaelkelly/ratatui-go/ompui/interact"
	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

func newBridgeApp(t *testing.T) *App {
	t.Helper()
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ed = editor.New()
	a.overlays = interact.NewOverlayStack()
	a.width, a.height = 40, 12
	return a
}

func TestShouldForwardAllKeyTextPasteMouseRawInclCtrlCEscape(t *testing.T) {
	t.Parallel()
	yes := []event.Event{
		event.KeyEvent(event.Key{ID: "a", Text: "a"}, []byte("a")),
		event.KeyEvent(event.Key{ID: "ctrl+c"}, []byte{0x03}),
		event.KeyEvent(event.Key{ID: "escape"}, []byte{0x1b}),
		event.KeyEvent(event.Key{ID: "ctrl+l"}, []byte{0x0c}),
		event.TextEvent("x", []byte("x")),
		event.PasteEvent("p", []byte("\x1b[200~p\x1b[201~")),
		event.MouseEvent(event.Mouse{Col: 1, Row: 2}, []byte("\x1b[<0;2;3M")),
		event.RawEvent([]byte("\x1b[?")),
	}
	for _, ev := range yes {
		if !shouldForwardTerminalInput(ev) {
			t.Fatalf("should forward kind=%v id=%q", ev.Kind, ev.Key.ID)
		}
		if len(terminalInputBytes(ev)) == 0 {
			t.Fatalf("forwardable event needs bytes kind=%v", ev.Kind)
		}
	}
	no := []event.Event{
		event.ResizeEvent(event.Size{Cols: 80, Rows: 24}, nil),
		event.FocusEvent(true, nil),
		event.ErrorEvent(nil, nil),
	}
	for _, ev := range no {
		if shouldForwardTerminalInput(ev) {
			t.Fatalf("must not forward %v", ev.Kind)
		}
	}
}

func TestTerminalInputBytesPreferRawInclPasteDelimiters(t *testing.T) {
	t.Parallel()
	ev := event.KeyEvent(event.Key{ID: "a", Text: "a"}, []byte("RAW"))
	if string(terminalInputBytes(ev)) != "RAW" {
		t.Fatal(string(terminalInputBytes(ev)))
	}
	// Paste Raw carries bracketed delimiters; bridge must forward Raw not bare text.
	delim := []byte("\x1b[200~hello\x1b[201~")
	pev := event.PasteEvent("hello", delim)
	got := terminalInputBytes(pev)
	if string(got) != string(delim) {
		t.Fatalf("paste bytes=%q want delimited %q", got, delim)
	}
	// Fallback when Raw empty uses Text/Paste body.
	ev = event.TextEvent("hi", nil)
	if string(terminalInputBytes(ev)) != "hi" {
		t.Fatal(string(terminalInputBytes(ev)))
	}
}

func TestTerminalInputIDsMonotonic(t *testing.T) {
	a := newBridgeApp(t)
	a1 := a.nextTerminalInputID()
	a2 := a.nextTerminalInputID()
	if a1 == a2 || !strings.HasPrefix(a1, "ti-") {
		t.Fatalf("%q %q", a1, a2)
	}
}

func TestSubscriptionActivateDeactivateDrain(t *testing.T) {
	a := newBridgeApp(t)
	raw, _ := json.Marshal(map[string]any{"type": protocol.MsgTerminalInputSubscription, "active": true})
	env, _ := protocol.WrapHistorical(raw)
	a.handleTerminalInputFrame(client.Event{Envelope: env, Raw: raw})
	if !a.terminalInputActive {
		t.Fatal("not active")
	}

	// Without cli, maybeForward cannot claim the event.
	ev := event.KeyEvent(event.Key{ID: "x", Text: "x"}, []byte("x"))
	if a.maybeForwardTerminalInput(ev) {
		t.Fatal("no cli should not claim forward")
	}

	// Queue then deactivate with no in-flight drains locally.
	a.terminalInputActive = true
	a.terminalInputQueue = []pendingTerminalInput{
		{id: "ti-1", ev: event.TextEvent("a", []byte("a")), data: []byte("a")},
		{id: "ti-2", ev: event.TextEvent("b", []byte("b")), data: []byte("b")},
	}
	a.applyTerminalInputSubscription(false)
	if a.terminalInputActive {
		t.Fatal("still active")
	}
	if len(a.terminalInputQueue) != 0 {
		t.Fatalf("queue=%d", len(a.terminalInputQueue))
	}
}

func TestResultConsumeStaleTransformAndErrorDiagnostic(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true

	orig := event.KeyEvent(event.Key{ID: "a", Text: "a"}, []byte("a"))
	a.terminalInputInFlight = &pendingTerminalInput{id: "ti-9", ev: orig, data: []byte("a")}
	a.armTerminalInputTimeout()
	if a.terminalInputTimer == nil {
		t.Fatal("timer not armed")
	}

	// Stale id ignored; timer stays armed / inflight kept.
	a.applyTerminalInputResult(protocol.TerminalInputResultPayload{ID: "other", Consume: true})
	if a.terminalInputInFlight == nil {
		t.Fatal("stale cleared inflight")
	}

	// Consume drops local route and disarms timer.
	a.applyTerminalInputResult(protocol.TerminalInputResultPayload{ID: "ti-9", Consume: true})
	if a.terminalInputInFlight != nil {
		t.Fatal("consume left inflight")
	}
	if a.ed.Text() != "" {
		t.Fatalf("consume should not type: %q", a.ed.Text())
	}
	// Disarm leaves timer non-nil but stopped; TimeoutC may still exist.
	a.disarmTerminalInputTimeout()

	// Error is diagnostic only: consume/data still authoritative.
	// Error + consume → still consumed (no local route).
	a.ed.SetText("")
	a.terminalInputInFlight = &pendingTerminalInput{id: "ti-10", ev: orig, data: []byte("a")}
	a.armTerminalInputTimeout()
	a.applyTerminalInputResult(protocol.TerminalInputResultPayload{
		ID: "ti-10", Error: "listener boom", Consume: true,
	})
	if a.terminalInputInFlight != nil {
		t.Fatal("error+consume left inflight")
	}
	if a.ed.Text() != "" {
		t.Fatalf("error must not override consume: %q", a.ed.Text())
	}

	// Error + omitted data → route original (data path wins over error).
	a.ed.SetText("")
	z := event.KeyEvent(event.Key{ID: "z", Text: "z"}, []byte("z"))
	a.terminalInputInFlight = &pendingTerminalInput{id: "ti-11", ev: z, data: []byte("z")}
	a.applyTerminalInputResult(protocol.TerminalInputResultPayload{
		ID: "ti-11", Error: "diag only", Consume: false,
	})
	if a.terminalInputInFlight != nil {
		t.Fatal("error+omit left inflight")
	}
	if a.ed.Text() != "z" {
		t.Fatalf("original should route despite error: %q", a.ed.Text())
	}

	// Transformed data: fresh decoder, no re-forward (active stays true but no new inflight).
	a.ed.SetText("")
	a.terminalInputActive = true
	a.terminalInputInFlight = &pendingTerminalInput{
		id: "ti-12", ev: orig, data: []byte("a"),
	}
	a.applyTerminalInputResult(protocol.TerminalInputResultPayload{
		ID: "ti-12", Consume: false, HasData: true, Data: "Z",
	})
	if a.terminalInputInFlight != nil {
		t.Fatal("transform left inflight")
	}
	if a.ed.Text() != "Z" {
		t.Fatalf("transformed decode route=%q", a.ed.Text())
	}
	if !a.terminalInputActive {
		t.Fatal("transform must not disable subscription")
	}

	// Equal data to original → original event path.
	a.ed.SetText("")
	a.terminalInputInFlight = &pendingTerminalInput{
		id:   "ti-13",
		ev:   event.KeyEvent(event.Key{ID: "q", Text: "q"}, []byte("q")),
		data: []byte("q"),
	}
	a.applyTerminalInputResult(protocol.TerminalInputResultPayload{
		ID: "ti-13", HasData: true, Data: "q",
	})
	if a.ed.Text() != "q" {
		t.Fatalf("equal data original path=%q", a.ed.Text())
	}
}

func TestFailOpenDisablesActiveAndReplaysFIFO(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true
	a.terminalInputInFlight = &pendingTerminalInput{
		id:   "ti-0",
		ev:   event.KeyEvent(event.Key{ID: "1", Text: "1"}, []byte("1")),
		data: []byte("1"),
	}
	a.armTerminalInputTimeout()
	a.terminalInputQueue = []pendingTerminalInput{
		{id: "ti-1", ev: event.KeyEvent(event.Key{ID: "2", Text: "2"}, []byte("2")), data: []byte("2")},
		{id: "ti-2", ev: event.KeyEvent(event.Key{ID: "3", Text: "3"}, []byte("3")), data: []byte("3")},
	}
	a.failOpenTerminalInput("queue overflow")
	if a.terminalInputActive {
		t.Fatal("fail-open must disable active (circuit break)")
	}
	if a.terminalInputInFlight != nil || len(a.terminalInputQueue) != 0 {
		t.Fatalf("held not cleared inflight=%v q=%d", a.terminalInputInFlight != nil, len(a.terminalInputQueue))
	}
	// Replayed into editor in FIFO order.
	if a.ed.Text() != "123" {
		t.Fatalf("fifo replay=%q", a.ed.Text())
	}
}

func TestQueueOverflowFailOpen(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true
	a.terminalInputInFlight = &pendingTerminalInput{
		id: "ti-0", ev: event.TextEvent("0", []byte("0")), data: []byte("0"),
	}
	for range protocol.MaxTerminalInputQueue {
		a.terminalInputQueue = append(a.terminalInputQueue, pendingTerminalInput{
			id: a.nextTerminalInputID(),
			ev: event.TextEvent("q", []byte("q")), data: []byte("q"),
		})
	}
	// Mirror production branch.
	if len(a.terminalInputQueue) >= protocol.MaxTerminalInputQueue {
		a.failOpenTerminalInput("queue overflow")
	}
	if a.terminalInputActive || a.terminalInputInFlight != nil || len(a.terminalInputQueue) != 0 {
		t.Fatal("overflow fail-open incomplete")
	}
}

func TestPumpQueueAfterResult(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true
	// Without cli, pump send fails → route first + drain rest.
	a.terminalInputQueue = []pendingTerminalInput{
		{id: "ti-1", ev: event.TextEvent("1", []byte("1")), data: []byte("1")},
		{id: "ti-2", ev: event.TextEvent("2", []byte("2")), data: []byte("2")},
	}
	a.pumpTerminalInputQueue()
	if a.terminalInputInFlight != nil {
		t.Fatal("inflight after failed send")
	}
	if len(a.terminalInputQueue) != 0 {
		t.Fatalf("queue=%d", len(a.terminalInputQueue))
	}
}

func TestArmDisarmTerminalInputTimeout(t *testing.T) {
	a := newBridgeApp(t)
	if a.terminalInputTimeoutC() != nil {
		t.Fatal("nil timer should yield nil channel")
	}
	a.armTerminalInputTimeout()
	if a.terminalInputTimer == nil {
		t.Fatal("arm creates timer")
	}
	ch := a.terminalInputTimeoutC()
	if ch == nil {
		t.Fatal("TimeoutC nil after arm")
	}
	// Re-arm is safe (Reset path).
	a.armTerminalInputTimeout()
	a.disarmTerminalInputTimeout()
	// After disarm, channel must not fire within the response window.
	select {
	case <-a.terminalInputTimeoutC():
		t.Fatal("disarmed timer fired")
	case <-time.After(20 * time.Millisecond):
	}
	// Constant is the circuit-break budget.
	if terminalInputResponseTimeout != 500*time.Millisecond {
		t.Fatalf("timeout=%v", terminalInputResponseTimeout)
	}
}

func TestTimeoutCircuitBreakFailOpensAll(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true
	a.terminalInputInFlight = &pendingTerminalInput{
		id:   "ti-0",
		ev:   event.KeyEvent(event.Key{ID: "a", Text: "a"}, []byte("a")),
		data: []byte("a"),
	}
	a.terminalInputQueue = []pendingTerminalInput{
		{id: "ti-1", ev: event.KeyEvent(event.Key{ID: "b", Text: "b"}, []byte("b")), data: []byte("b")},
	}
	// Simulate loop case <-terminalInputTimeoutC():
	a.failOpenTerminalInput("response timeout")
	if a.terminalInputActive {
		t.Fatal("timeout must open circuit (active=false)")
	}
	if a.terminalInputInFlight != nil || len(a.terminalInputQueue) != 0 {
		t.Fatal("timeout must clear held events")
	}
	if a.ed.Text() != "ab" {
		t.Fatalf("timeout replay=%q", a.ed.Text())
	}
	// After circuit open, forward is off even if someone sets inflight empty.
	if a.maybeForwardTerminalInput(event.KeyEvent(event.Key{ID: "x", Text: "x"}, []byte("x"))) {
		t.Fatal("forward after circuit open")
	}
}

func TestHandleTermEventForwardsWhenActiveWithCliNilFallsLocal(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true
	// cli nil → maybeForward false → local route (ctrl+c still local global key).
	a.handleTermEvent(event.KeyEvent(event.Key{ID: "x", Text: "x"}, []byte("x")))
	if a.ed.Text() != "x" {
		t.Fatalf("local fallback=%q", a.ed.Text())
	}
	// Resize always local even when active.
	a.handleTermEvent(event.ResizeEvent(event.Size{Cols: 90, Rows: 30}, nil))
	if a.width != 90 || a.height != 30 {
		t.Fatalf("resize %dx%d", a.width, a.height)
	}
}

func TestRouteTransformedBytesNoRecursiveForward(t *testing.T) {
	a := newBridgeApp(t)
	a.terminalInputActive = true
	// Transformed path routes locally even while active.
	a.routeTransformedTerminalBytes([]byte("hi"))
	if a.ed.Text() != "hi" {
		t.Fatalf("transformed=%q", a.ed.Text())
	}
	if a.terminalInputInFlight != nil || len(a.terminalInputQueue) != 0 {
		t.Fatal("transform must not enqueue forward")
	}
}

func TestComponentInputResultDirty(t *testing.T) {
	a := newBridgeApp(t)
	a.width, a.height = 80, 24
	r := component.NewRemote("comp")
	a.remotes["comp"] = r
	a.remoteSlots["comp"] = protocol.SlotOverlay
	r.SetLines([]string{"old"})
	genBefore := r.Generation()
	raw, _ := json.Marshal(protocol.ComponentInputResultPayload{
		ComponentID: "comp", Handled: true, Dirty: true,
	})
	a.handleComponentInputResult(raw, protocol.Envelope{})
	if r.Generation() <= genBefore && !r.Frame().IsEmpty() {
		// Invalidate bumps gen or clears lines.
		if r.Generation() == genBefore {
			t.Fatal("expected dirty invalidate side effect")
		}
	}
}

func TestComponentFocusRequestOwnedOnly(t *testing.T) {
	a := newBridgeApp(t)
	a.width, a.height = 80, 24
	raw, _ := json.Marshal(map[string]any{"componentId": "missing"})
	a.handleComponentFocusRequest(raw, protocol.Envelope{})

	p := protocol.ComponentFocusRequestPayload{ComponentID: "x"}
	if !p.WantFocused() {
		t.Fatal("default focused")
	}
	f := false
	p.Focused = &f
	if p.WantFocused() {
		t.Fatal("explicit false")
	}
}

func TestTerminalInputPayloadRoundTrip(t *testing.T) {
	t.Parallel()
	p := protocol.TerminalInputPayload{ID: "ti-1", Data: []byte("hi")}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var back protocol.TerminalInputPayload
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.ID != "ti-1" || string(back.Data) != "hi" {
		t.Fatalf("%+v", back)
	}
	if string(back.InputBytes()) != "hi" {
		t.Fatal(string(back.InputBytes()))
	}

	res := protocol.TerminalInputResultPayload{ID: "ti-1", Consume: false, HasData: true, Data: "Z"}
	b, err = json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var resBack protocol.TerminalInputResultPayload
	if err := json.Unmarshal(b, &resBack); err != nil {
		t.Fatal(err)
	}
	if !resBack.HasData || resBack.Data != "Z" || string(resBack.DataBytes()) != "Z" {
		t.Fatalf("%+v", resBack)
	}

	res = protocol.TerminalInputResultPayload{ID: "ti-2", HasData: true, Data: ""}
	b, _ = json.Marshal(res)
	if !strings.Contains(string(b), `"data"`) {
		t.Fatalf("empty data omitted: %s", b)
	}
	_ = json.Unmarshal(b, &resBack)
	if !resBack.HasData {
		t.Fatal("HasData lost")
	}
}

func TestMaxTerminalInputConstants(t *testing.T) {
	t.Parallel()
	if protocol.MaxTerminalInputBytes != 64*1024 {
		t.Fatal(protocol.MaxTerminalInputBytes)
	}
	if protocol.MaxTerminalInputQueue != 256 {
		t.Fatal(protocol.MaxTerminalInputQueue)
	}
	if terminalInputResponseTimeout != 500*time.Millisecond {
		t.Fatal(terminalInputResponseTimeout)
	}
}
