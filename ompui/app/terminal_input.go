package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/input"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
	"github.com/lyc-aon/ratatui-go/ompui/termcaps"
)

const terminalInputResponseTimeout = 500 * time.Millisecond

// pendingTerminalInput is one host event waiting on (or mid) Bun terminal_input.
type pendingTerminalInput struct {
	id   string
	ev   event.Event
	data []byte // exact bytes forwarded (capped); compare to result.Data
}

// shouldForwardTerminalInput reports whether the event is raw key/paste/mouse
// traffic that extensions may intercept. Resize/focus/error stay local.
func shouldForwardTerminalInput(ev event.Event) bool {
	switch ev.Kind {
	case event.KindKey, event.KindText, event.KindPaste, event.KindMouse, event.KindRaw:
		return true
	default:
		return false
	}
}

// terminalInputBytes extracts the raw bytes to forward for one event.
// Prefer ev.Raw (includes bracketed-paste delimiters when present).
func terminalInputBytes(ev event.Event) []byte {
	if len(ev.Raw) > 0 {
		return append([]byte(nil), ev.Raw...)
	}
	switch ev.Kind {
	case event.KindText:
		if ev.Text != "" {
			return []byte(ev.Text)
		}
	case event.KindPaste:
		if ev.Paste != "" {
			return []byte(ev.Paste)
		}
	case event.KindKey:
		if ev.Key.Text != "" {
			return []byte(ev.Key.Text)
		}
		if ev.Text != "" {
			return []byte(ev.Text)
		}
	}
	return nil
}

// maybeForwardTerminalInput queues or sends a terminal_input frame while the
// subscription is active. Returns true when the event was accepted by the
// bridge (caller must not route locally yet). Fail-open returns false.
func (a *App) maybeForwardTerminalInput(ev event.Event) bool {
	if !a.terminalInputActive || a.cli == nil || !shouldForwardTerminalInput(ev) {
		return false
	}
	data := terminalInputBytes(ev)
	if len(data) == 0 {
		return false
	}
	if len(data) > protocol.MaxTerminalInputBytes {
		data = append([]byte(nil), data[:protocol.MaxTerminalInputBytes]...)
	}

	item := pendingTerminalInput{
		id:   a.nextTerminalInputID(),
		ev:   ev,
		data: data,
	}

	if a.terminalInputInFlight != nil {
		if len(a.terminalInputQueue) >= protocol.MaxTerminalInputQueue {
			a.failOpenTerminalInput("queue overflow")
			return false
		}
		a.terminalInputQueue = append(a.terminalInputQueue, item)
		return true
	}
	if err := a.sendTerminalInput(item); err != nil {
		a.logf("terminal_input send: %v", err)
		return false
	}
	a.terminalInputInFlight = &item
	a.armTerminalInputTimeout()
	return true
}

func (a *App) nextTerminalInputID() string {
	a.terminalInputSeq++
	return fmt.Sprintf("ti-%d", a.terminalInputSeq)
}

// sendTerminalInput emits bare additive JSON (not an RpcCommand waiter).
// data []byte → base64 via encoding/json.
func (a *App) sendTerminalInput(item pendingTerminalInput) error {
	if a.cli == nil {
		return client.ErrClosed
	}
	body, err := json.Marshal(struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Data []byte `json:"data,omitempty"`
	}{
		Type: protocol.MsgTerminalInput,
		ID:   item.id,
		Data: item.data,
	})
	if err != nil {
		return err
	}
	return a.cli.SendRaw(body)
}

func (a *App) handleTerminalInputFrame(ev client.Event) {
	typ := ev.Envelope.Type
	raw := ev.Raw
	if len(raw) == 0 {
		raw = ev.Envelope.HistoricalPayload()
	}
	switch typ {
	case protocol.MsgTerminalInputSubscription:
		var p protocol.TerminalInputSubscriptionPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			_ = protocol.DecodePayload(ev.Envelope, &p)
		}
		a.applyTerminalInputSubscription(p.Active)

	case protocol.MsgTerminalInputResult:
		var p protocol.TerminalInputResultPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			a.logf("terminal_input_result decode: %v", err)
			a.failOpenInFlightOnly("malformed result")
			return
		}
		a.applyTerminalInputResult(p)

	case protocol.MsgTerminalInput:
		// Host→core only; ignore inbound echo.
	default:
		a.logf("terminal input frame %s", typ)
	}
}

func (a *App) applyTerminalInputSubscription(active bool) {
	was := a.terminalInputActive
	a.terminalInputActive = active
	if was && !active {
		// Off: after any in-flight response, drain queued originals.
		if a.terminalInputInFlight == nil {
			a.drainTerminalInputQueueLocal()
		}
	}
}

func (a *App) applyTerminalInputResult(p protocol.TerminalInputResultPayload) {
	inflight := a.terminalInputInFlight
	if inflight == nil {
		return
	}
	if p.ID == "" || p.ID != inflight.id {
		// Stale/duplicate — ignore; keep waiting.
		return
	}

	orig := *inflight
	a.terminalInputInFlight = nil
	a.disarmTerminalInputTimeout()

	if p.Error != "" {
		// Bun isolates listener exceptions and still runs the remaining chain.
		// The error is diagnostic; consume/data still carry the final outcome.
		a.logf("terminal_input_result %s: %s", p.ID, p.Error)
	}
	if p.Consume {
		a.afterTerminalInputSettled()
		return
	}

	// Omitted data, or data exactly equal to forwarded Raw → original event.
	if !p.HasData || p.Data == string(orig.data) || bytes.Equal([]byte(p.Data), orig.data) {
		a.routeLocalTermEvent(orig.ev)
		a.afterTerminalInputSettled()
		return
	}

	// Changed data: fresh Decoder Write+Flush, route locally, no recursive forward.
	data := []byte(p.Data)
	if len(data) > protocol.MaxTerminalInputBytes {
		data = data[:protocol.MaxTerminalInputBytes]
	}
	a.routeTransformedTerminalBytes(data)
	a.afterTerminalInputSettled()
}

func (a *App) afterTerminalInputSettled() {
	if !a.terminalInputActive {
		a.drainTerminalInputQueueLocal()
		return
	}
	a.pumpTerminalInputQueue()
}

func (a *App) pumpTerminalInputQueue() {
	for a.terminalInputInFlight == nil && len(a.terminalInputQueue) > 0 {
		if !a.terminalInputActive {
			a.drainTerminalInputQueueLocal()
			return
		}
		next := a.terminalInputQueue[0]
		a.terminalInputQueue = a.terminalInputQueue[1:]
		if err := a.sendTerminalInput(next); err != nil {
			a.logf("terminal_input send: %v", err)
			a.routeLocalTermEvent(next.ev)
			a.drainTerminalInputQueueLocal()
			return
		}
		item := next
		a.terminalInputInFlight = &item
		a.armTerminalInputTimeout()
		return
	}
}

func (a *App) drainTerminalInputQueueLocal() {
	if len(a.terminalInputQueue) == 0 {
		return
	}
	q := a.terminalInputQueue
	a.terminalInputQueue = nil
	for i := range q {
		a.routeLocalTermEvent(q[i].ev)
	}
}

func (a *App) failOpenTerminalInput(reason string) {
	a.terminalInputActive = false
	a.disarmTerminalInputTimeout()
	a.logf("terminal_input fail-open: %s", reason)
	var held []pendingTerminalInput
	if a.terminalInputInFlight != nil {
		held = append(held, *a.terminalInputInFlight)
		a.terminalInputInFlight = nil
	}
	if len(a.terminalInputQueue) > 0 {
		held = append(held, a.terminalInputQueue...)
		a.terminalInputQueue = nil
	}
	for i := range held {
		a.routeLocalTermEvent(held[i].ev)
	}
}

func (a *App) failOpenInFlightOnly(reason string) {
	if a.terminalInputInFlight == nil {
		return
	}
	a.logf("terminal_input fail-open in-flight: %s", reason)
	orig := *a.terminalInputInFlight
	a.terminalInputInFlight = nil
	a.disarmTerminalInputTimeout()
	a.routeLocalTermEvent(orig.ev)
	a.afterTerminalInputSettled()
}

func (a *App) armTerminalInputTimeout() {
	if a.terminalInputTimer == nil {
		a.terminalInputTimer = time.NewTimer(terminalInputResponseTimeout)
		return
	}
	if !a.terminalInputTimer.Stop() {
		select {
		case <-a.terminalInputTimer.C:
		default:
		}
	}
	a.terminalInputTimer.Reset(terminalInputResponseTimeout)
}

func (a *App) disarmTerminalInputTimeout() {
	if a.terminalInputTimer == nil {
		return
	}
	if !a.terminalInputTimer.Stop() {
		select {
		case <-a.terminalInputTimer.C:
		default:
		}
	}
}

func (a *App) terminalInputTimeoutC() <-chan time.Time {
	if a.terminalInputTimer == nil {
		return nil
	}
	return a.terminalInputTimer.C
}

// routeTransformedTerminalBytes decodes transformed bytes with a fresh
// input.Decoder Write+Flush and routes each event locally without re-forwarding.
func (a *App) routeTransformedTerminalBytes(data []byte) {
	if len(data) == 0 {
		return
	}
	opts := input.Options{WindowsTerminal: goruntime.GOOS == "windows"}
	if a.term != nil {
		opts.KittyActive = a.term.KittyProtocolActive()
	}
	dec := input.NewDecoder(opts)
	dec.Write(data)
	for _, ev := range dec.Flush() {
		switch ev.Kind {
		case event.KindResize, event.KindFocus, event.KindError:
			continue
		}
		a.routeLocalTermEvent(ev)
	}
}

// routeLocalTermEvent is the local key/overlay/editor path with no
// terminal_input forwarding (after results and fail-open).
func (a *App) routeLocalTermEvent(ev event.Event) {
	switch ev.Kind {
	case event.KindResize:
		a.width = ev.Size.Cols
		a.height = ev.Size.Rows
		if a.width < 1 {
			a.width = 1
		}
		if a.height < 1 {
			a.height = 1
		}
		if a.sched != nil {
			a.sched.MarkResizeEvent()
		}
		a.overlays.ReconcileFocus(a.width, a.height)
		if a.term != nil && a.sched != nil {
			caps := renderer.CapsFromSnapshot(a.term.Capabilities(), termcaps.ProcessEnv())
			a.sched.SetCaps(caps)
		}
		if a.images != nil {
			a.images.Clear()
		}
		// Re-request every live remote so Bun can re-eval visible(w,h) and reflow.
		// Includes height-only resizes (LastWidth path still sends current height).
		for id := range a.remotes {
			a.requestRemoteRender(id)
		}
		a.requestRender(renderer.ReasonResize)
		return
	case event.KindError:
		if ev.Err != nil {
			a.logf("tty event error: %v", ev.Err)
		}
		return
	case event.KindFocus:
		return
	}

	// The focused component owns input, including Ctrl+C/Escape, matching the
	// TypeScript TUI. App-level shortcuts apply only to the built-in editor.
	if a.overlays.HasVisible(a.width, a.height) {
		a.overlays.HandleInput(ev)
		a.requestRender(renderer.ReasonFlush)
		return
	}

	target := component.Component(a.ed)
	if a.root != nil && a.root.FocusTarget() != nil {
		target = a.root.FocusTarget()
	}
	if target != a.ed {
		component.RouteInput(target, ev)
		a.requestRender(renderer.ReasonFlush)
		return
	}

	if ev.Kind == event.KindKey && ev.Key.Action != event.ActionRelease {
		if a.handleGlobalKey(ev) {
			a.requestRender(renderer.ReasonFlush)
			return
		}
	}

	component.RouteInput(target, ev)
	a.requestRender(renderer.ReasonFlush)
}

// handleComponentInputResult dirties/re-renders a remote after Bun input handling.
func (a *App) handleComponentInputResult(raw []byte, env protocol.Envelope) {
	var p protocol.ComponentInputResultPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		_ = protocol.DecodePayload(env, &p)
	}
	if p.ComponentID == "" {
		return
	}
	if p.Error != "" {
		a.logf("component_input_result %s: %s", p.ComponentID, p.Error)
	}
	if p.Dirty {
		if r := a.remotes[p.ComponentID]; r != nil {
			r.Invalidate()
			a.requestRemoteRender(p.ComponentID)
		}
		a.requestRender(renderer.ReasonUpdate)
	}
}

// handleComponentFocusRequest focuses a currently owned Remote only.
// Never replies or echoes.
func (a *App) handleComponentFocusRequest(raw []byte, env protocol.Envelope) {
	var p protocol.ComponentFocusRequestPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		_ = protocol.DecodePayload(env, &p)
	}
	if p.ComponentID == "" {
		return
	}
	r := a.remotes[p.ComponentID]
	if r == nil {
		return
	}
	focused := p.WantFocused()
	r.SetFocused(focused)
	if focused {
		slot := a.remoteSlots[p.ComponentID]
		if slot == protocol.SlotOverlay || slot == protocol.SlotCustom || slot == "" {
			a.overlays.SetFocus(r, a.width, a.height)
		} else if a.root != nil {
			component.ApplyFocus(a.root.FocusTarget(), r, false)
			a.root.SetFocusTarget(r)
		}
	} else if a.root != nil && a.root.FocusTarget() == r {
		component.ApplyFocus(r, a.ed, false)
		a.root.SetFocusTarget(a.ed)
	}
	a.requestRender(renderer.ReasonFlush)
}
