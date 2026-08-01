package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
	"strings"
	"time"
)

func (a *App) handleTermEvent(ev event.Event) {
	// Resize / focus / error always stay local (never forwarded).
	switch ev.Kind {
	case event.KindResize, event.KindError, event.KindFocus:
		a.routeLocalTermEvent(ev)
		return
	}

	// While subscription active, forward key/paste/mouse raw input to Bun first.
	// One in-flight; FIFO queue; local route only after result / fail-open.
	if a.maybeForwardTerminalInput(ev) {
		return
	}
	a.routeLocalTermEvent(ev)
}

func (a *App) handleGlobalKey(ev event.Event) bool {
	k := ev.Key

	if event.MatchesKey(k, "ctrl+c") {
		a.handleCtrlC()
		return true
	}
	if event.MatchesKey(k, "ctrl+d") {
		if strings.TrimSpace(a.ed.Text()) == "" && !a.overlays.HasVisible(a.width, a.height) && a.editorRemote == nil {
			a.handleCtrlD()
			return true
		}
		return false
	}
	if event.MatchesKey(k, "ctrl+l") {
		a.resetDisplay()
		return true
	}
	if event.MatchesKey(k, "escape") {
		return a.handleEscape()
	}
	if event.MatchesKey(k, "ctrl+o") {
		a.toggleToolsExpanded()
		return true
	}
	if event.MatchesKey(k, "shift+tab") {
		a.cycleThinking()
		return true
	}
	if event.MatchesKey(k, "ctrl+p") {
		if a.ed.IsShowingAutocomplete() {
			return false
		}
		a.cycleModel()
		return true
	}
	if event.MatchesAnyKey(k, "ctrl+enter", "ctrl+q") {
		text := strings.TrimSpace(a.ed.ExpandedText())
		if text != "" {
			a.followUpPrefer = true
			a.submitEditor(text)
			a.followUpPrefer = false
			return true
		}
	}
	return false
}

func (a *App) handleCtrlC() {
	if a.shuttingDown.Load() {
		a.cancel()
		return
	}
	now := time.Now()
	if a.overlays.HasVisible(a.width, a.height) {
		a.cancelTopOverlay(false)
		a.lastSigint = now
		a.requestRender(renderer.ReasonFlush)
		return
	}
	text := strings.TrimSpace(a.ed.Text())
	streaming := a.isStreaming()

	// Streaming + empty editor: abort on first ctrl+c.
	if streaming && text == "" {
		a.abortAgent()
		a.lastSigint = now
		a.requestRender(renderer.ReasonFlush)
		return
	}

	if !streaming && text == "" && !a.lastSigint.IsZero() && now.Sub(a.lastSigint) < 500*time.Millisecond {
		a.quitReason = "ctrl-c"
		a.exitCode = ExitOK
		a.cancel()
		return
	}
	if text != "" || a.ed.IsShowingAutocomplete() {
		a.ed.CancelAutocomplete()
		a.ed.SetText("")
		a.lastSigint = now
		a.requestRender(renderer.ReasonFlush)
		return
	}
	// Idle empty: arm double-tap quit.
	a.lastSigint = now
	a.setLocalNotice("press ctrl+c again to quit")
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) handleCtrlD() {
	if a.shuttingDown.Load() {
		return
	}
	a.quitReason = "ctrl-d"
	a.cancel()
}

func (a *App) handleEscape() bool {
	if a.overlays.HasVisible(a.width, a.height) {
		a.cancelTopOverlay(false)
		return true
	}
	if a.ed.IsShowingAutocomplete() {
		a.ed.CancelAutocomplete()
		return true
	}
	if a.isStreaming() {
		a.abortAgent()
		return true
	}
	if strings.TrimSpace(a.ed.Text()) != "" {
		a.ed.SetText("")
		return true
	}
	return true
}

func (a *App) resetDisplay() {
	if a.sched != nil {
		a.sched.ForceNextWindowRewrite()
	}
	if a.transcript != nil {
		a.transcript.Invalidate()
	}
	if a.status != nil {
		a.status.Invalidate()
	}
	if a.footer != nil {
		a.footer.Invalidate()
	}
	if a.overlays != nil {
		a.overlays.InvalidateAll()
	}
	a.requestRender(renderer.ReasonReset)
}

func (a *App) toggleToolsExpanded() {
	// Effective shown state: forced local value or snapshot.
	snap := a.state.Snapshot()
	shown := snap.Status.ToolsExpanded
	if a.toolsForced {
		shown = a.toolsExpand
	}
	a.toolsExpand = !shown
	a.toolsForced = true

	payload, _ := json.Marshal(map[string]any{
		"type": protocol.MsgToolsExpanded, "expanded": a.toolsExpand,
	})
	if env, err := protocol.WrapHistorical(payload); err == nil {
		_, _ = a.state.Apply(env)
	}
	// SetSnapshot adopts ToolsExpanded; then override with forced value.
	a.syncFromSnapshot(false)
	opts := a.viewOpts
	opts.ToolsExpanded = a.toolsExpand
	a.transcript.SetOptions(opts)
	a.requestRender(renderer.ReasonForce)
}

func (a *App) cycleThinking() {
	a.bgCall("cycle_thinking_level", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdCycleThinkingLevel, "", nil))
	}, "")
}

func (a *App) cycleModel() {
	a.bgCall("cycle_model", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdCycleModel, "", nil))
	}, "")
}

// submitEditor sends editor text RAW — Bun core resolves inline @mentions.
func (a *App) submitEditor(text string) {
	text = strings.TrimRight(text, "\n")
	expanded := a.ed.ExpandedText()
	if expanded != "" {
		text = strings.TrimRight(expanded, "\n")
	}
	if strings.TrimSpace(text) == "" {
		return
	}

	a.ed.AddToHistory(text)
	a.ed.SetText("")
	a.ed.CancelAutocomplete()
	a.editorDirty = true

	streaming := a.isStreaming()
	restore := text
	send := func(ctx context.Context) (client.Response, error) {
		switch {
		case streaming && a.followUpPrefer:
			return a.cli.FollowUp(ctx, text)
		case streaming:
			return a.cli.Prompt(ctx, text, client.WithStreamingBehavior("steer"))
		default:
			return a.cli.Prompt(ctx, text)
		}
	}
	a.bgCall("prompt", send, restore)
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) abortAgent() {
	a.bgCall("abort", func(ctx context.Context) (client.Response, error) {
		return a.cli.Abort(ctx)
	}, "")
}

func (a *App) handleRPCDone(d rpcDone) {
	if d.err != nil {
		if d.restore != "" {
			a.ed.SetText(d.restore)
		}
		if a.shuttingDown.Load() || errors.Is(d.err, context.Canceled) {
			return
		}
		a.setLocalError(d.op + ": " + d.err.Error())
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	a.applyResponse(d.resp)
	if !d.resp.Success && d.resp.Error != "" {
		if d.restore != "" {
			a.ed.SetText(d.restore)
		}
		a.setLocalError(d.resp.Error)
	}
	switch d.op {
	case "cycle_model", "cycle_thinking_level", "prompt", "steer", "follow_up", "abort":
		a.bgCall("get_state", a.cli.GetState, "")
	}
	a.requestRender(renderer.ReasonUpdate)
}
