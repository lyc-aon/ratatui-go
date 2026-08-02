package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/interact"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
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
	if k.Action == event.ActionRelease {
		return false
	}

	if a.keys.Matches(k, "app.clear") {
		a.handleCtrlC()
		return true
	}
	if a.keys.Matches(k, "app.exit") {
		if strings.TrimSpace(a.ed.Text()) == "" && !a.overlays.HasVisible(a.width, a.height) && a.editorRemote == nil {
			a.handleCtrlD()
			return true
		}
		return false
	}
	if a.keys.Matches(k, "app.display.reset") {
		a.resetDisplay()
		return true
	}
	if a.keys.Matches(k, "app.interrupt") {
		return a.handleEscape()
	}
	if a.keys.Matches(k, "app.suspend") {
		a.handleSuspend()
		return true
	}
	if a.keys.Matches(k, "app.tools.expand") {
		a.toggleToolsExpanded()
		return true
	}
	if a.keys.Matches(k, "app.thinking.cycle") {
		a.cycleThinking()
		return true
	}
	if a.keys.Matches(k, "app.thinking.toggle") {
		a.toggleThinking()
		return true
	}
	if a.keys.Matches(k, "app.model.cycleForward") {
		if a.ed.IsShowingAutocomplete() {
			return false
		}
		a.cycleModelForward()
		return true
	}
	if a.keys.Matches(k, "app.model.cycleBackward") {
		if a.ed.IsShowingAutocomplete() {
			return false
		}
		a.cycleModelBackward()
		return true
	}
	if a.keys.Matches(k, "app.model.select") {
		a.showModelSelector()
		return true
	}
	if a.keys.Matches(k, "app.model.selectTemporary") {
		a.showModelSelector()
		return true
	}
	if a.keys.Matches(k, "app.history.search") {
		a.showHistorySearch()
		return true
	}
	if a.keys.Matches(k, "app.retry") {
		a.retryLast()
		return true
	}
	if a.keys.Matches(k, "app.message.dequeue") {
		a.dequeueMessage()
		return true
	}
	if a.keys.Matches(k, "app.editor.external") {
		a.openExternalEditor()
		return true
	}
	if a.keys.Matches(k, "app.agents.hub") {
		a.showAgentsHub()
		return true
	}
	if a.keys.Matches(k, "app.session.observe") {
		a.showAgentsHub()
		return true
	}
	if a.keys.Matches(k, "app.plan.toggle") {
		a.togglePlanMode()
		return true
	}
	if a.keys.Matches(k, "app.clipboard.pasteImage") {
		a.handlePasteImage()
		return true
	}
	if a.keys.Matches(k, "app.clipboard.pasteTextRaw") {
		a.handlePasteTextRaw()
		return true
	}
	if a.keys.Matches(k, "app.clipboard.copyLine") {
		a.handleCopyLine()
		return true
	}
	if a.keys.Matches(k, "app.clipboard.copyPrompt") {
		a.handleCopyPrompt()
		return true
	}
	if event.CanonicalKeyID(k.ID) == "left" &&
		strings.TrimSpace(a.ed.Text()) == "" &&
		!a.ed.IsShowingAutocomplete() &&
		!a.overlays.HasVisible(a.width, a.height) {
		if a.detectLeftDoubleTap(time.Now()) {
			a.showAgentsHub(true)
		}
		return true
	}

	if a.keys.Matches(k, "app.message.followUp") {
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

func (a *App) detectLeftDoubleTap(now time.Time) bool {
	const (
		minGap = 40 * time.Millisecond
		maxGap = 500 * time.Millisecond
	)
	sinceLast := now.Sub(a.lastLeftTap)
	a.lastLeftTap = now
	if sinceLast >= maxGap || sinceLast < 0 {
		a.leftTapCount = 1
		return false
	}
	a.leftTapCount++
	if a.leftTapCount == 2 && sinceLast >= minGap {
		a.leftTapCount = 0
		a.lastLeftTap = time.Time{}
		return true
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
	a.viewOpts.ToolsExpanded = a.toolsExpand
	a.transcript.SetOptions(a.viewOpts)
	a.requestRender(renderer.ReasonForce)
}

func (a *App) cycleThinking() {
	a.bgCall("cycle_thinking_level", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdCycleThinkingLevel, "", nil))
	}, "")
}

func (a *App) cycleModel() {
	a.cycleModelForward()
}

func (a *App) cycleModelForward() {
	a.bgCall("cycle_model", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdCycleModel, "", nil))
	}, "")
}

func (a *App) cycleModelBackward() {
	if a.cli == nil {
		a.setLocalError("cycle model backward: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	if !a.bgCallWithCompletion("get_available_models", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetAvailableModels, "", nil))
	}, "", func(a *App, d rpcDone) {
		if d.err != nil || !d.resp.Success {
			return
		}
		models, err := decodeAvailableModelItems(d.resp)
		if err != nil {
			a.setLocalError("cycle model backward: " + err.Error())
			return
		}
		current, err := currentAvailableModel(a.state.Snapshot().Session.Model)
		if err != nil {
			a.setLocalError("cycle model backward: " + err.Error())
			return
		}
		previous, err := previousAvailableModel(models, current)
		if err != nil {
			a.setLocalError("cycle model backward: " + err.Error())
			return
		}
		a.bgCall("set_model", func(ctx context.Context) (client.Response, error) {
			return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdSetModel, "", map[string]any{
				"provider": previous.Provider,
				"modelId":  previous.ID,
			}))
		}, "")
	}) {
		a.setLocalError("cycle model backward: RPC worker unavailable")
		a.requestRender(renderer.ReasonUpdate)
	}
}

func currentAvailableModel(raw json.RawMessage) (availableModelItem, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return availableModelItem{}, fmt.Errorf("current model is unavailable")
	}
	var model availableModelItem
	if err := json.Unmarshal(raw, &model); err != nil {
		return availableModelItem{}, fmt.Errorf("decode current model: %w", err)
	}
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.ID) == "" {
		return availableModelItem{}, fmt.Errorf("current model is missing provider or id")
	}
	return model, nil
}

func previousAvailableModel(models []availableModelItem, current availableModelItem) (availableModelItem, error) {
	if len(models) < 2 {
		return availableModelItem{}, fmt.Errorf("only one model is available")
	}
	for i, model := range models {
		if model.Provider == current.Provider && model.ID == current.ID {
			if i == 0 {
				return models[len(models)-1], nil
			}
			return models[i-1], nil
		}
	}
	return availableModelItem{}, fmt.Errorf("current model is not available")
}

func (a *App) toggleThinking() {
	if !a.toolsForced {
		a.viewOpts.ToolsExpanded = a.state.Snapshot().Status.ToolsExpanded
	}
	a.viewOpts.HideThinking = !a.viewOpts.HideThinking
	if a.transcript != nil {
		a.transcript.SetOptions(a.viewOpts)
		a.transcript.Invalidate()
	}
	visibility := "visible"
	if a.viewOpts.HideThinking {
		visibility = "hidden"
	}
	a.setLocalNotice("thinking blocks: " + visibility)
	a.requestRender(renderer.ReasonForce)
}

// submitEditor sends editor text RAW — Bun core resolves inline @mentions.
func (a *App) submitEditor(text string) {
	text = strings.TrimRight(text, "\n")
	expanded := a.ed.ExpandedText()
	if expanded != "" {
		text = strings.TrimRight(expanded, "\n")
	}
	if strings.TrimSpace(text) == "" && len(a.pendingPromptImages) == 0 {
		return
	}
	if a.cli == nil {
		a.setLocalError("prompt: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}

	if strings.TrimSpace(text) != "" {
		a.ed.AddToHistory(text)
	}
	a.ed.SetText("")
	a.ed.CancelAutocomplete()
	a.editorDirty = true

	streaming := a.isStreaming()
	followUp := a.followUpPrefer
	restore := text
	images := a.takePendingPromptImages()
	send := func(ctx context.Context) (client.Response, error) {
		var opts []client.PromptOption
		if len(images) > 0 {
			opts = append(opts, client.WithPromptImages(images...))
		}
		if streaming {
			behavior := "steer"
			if followUp {
				behavior = "followUp"
			}
			opts = append(opts, client.WithStreamingBehavior(behavior))
		}
		return a.cli.Prompt(ctx, text, opts...)
	}
	if !a.bgCallWithCompletion("prompt", send, restore, func(a *App, d rpcDone) {
		if d.err != nil || !d.resp.Success {
			a.restorePendingPromptImages(images)
		}
	}) {
		a.restorePendingPromptImages(images)
		a.ed.SetText(restore)
		a.setLocalError("prompt: RPC worker unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) takePendingPromptImages() []any {
	images := a.pendingPromptImages
	a.pendingPromptImages = nil
	return images
}

func (a *App) restorePendingPromptImages(images []any) {
	if len(images) == 0 {
		return
	}
	a.pendingPromptImages = append(images, a.pendingPromptImages...)
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
		if d.complete != nil {
			d.complete(a, d)
		}
		if a.shuttingDown.Load() || errors.Is(d.err, context.Canceled) {
			return
		}
		a.setLocalError(d.op + ": " + d.err.Error())
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	a.applyResponse(d.resp)
	if !d.resp.Success {
		if d.restore != "" {
			a.ed.SetText(d.restore)
		}
		if d.complete != nil {
			d.complete(a, d)
		}
		message := d.resp.Error
		if message == "" {
			message = d.op + ": command failed"
		}
		a.setLocalError(message)
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	if d.complete != nil {
		d.complete(a, d)
	}
	switch d.op {
	case "cycle_model", "set_model", "cycle_thinking_level", "set_thinking_level", "prompt", "steer", "follow_up", "abort":
		a.bgCall("get_state", a.cli.GetState, "")
	}
	a.requestRender(renderer.ReasonUpdate)
}

type availableModelItem struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Name     string `json:"name"`
}

type availableModelsResponse struct {
	Models []availableModelItem `json:"models"`
}

func (a *App) showModelSelector() {
	if a.cli == nil {
		a.setLocalError("get_available_models: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	if !a.bgCallWithCompletion("get_available_models", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetAvailableModels, "", nil))
	}, "", func(a *App, d rpcDone) {
		if d.err != nil || !d.resp.Success {
			return
		}
		models, err := decodeAvailableModelItems(d.resp)
		if err != nil {
			a.setLocalError("get_available_models: " + err.Error())
			return
		}
		a.openModelSelector(models)
	}) {
		a.setLocalError("get_available_models: RPC worker unavailable")
		a.requestRender(renderer.ReasonUpdate)
	}
}

func decodeAvailableModelItems(resp client.Response) ([]availableModelItem, error) {
	if resp.Command != "" && resp.Command != protocol.CmdGetAvailableModels {
		return nil, fmt.Errorf("unexpected response command %q", resp.Command)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("missing response data")
	}
	var payload availableModelsResponse
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(payload.Models) == 0 {
		return nil, fmt.Errorf("no models available")
	}
	for i, model := range payload.Models {
		if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.ID) == "" {
			return nil, fmt.Errorf("model %d is missing provider or id", i+1)
		}
	}
	return payload.Models, nil
}

func (a *App) openModelSelector(models []availableModelItem) {
	th := interact.SelectListTheme{Cursor: a.themes.theme.Accent("❯")}
	picker := interact.NewSelectList(models, 10, th, func(m availableModelItem) interact.SelectItem {
		title := m.Name
		if title == "" {
			title = m.ID
		}
		description := m.Provider
		if m.Name != "" && m.Name != m.ID {
			description += " · " + m.ID
		}
		return interact.SelectItem{
			Label:       title,
			Description: description,
		}
	}, interact.SelectListLayoutOptions{})

	picker.SetKeyMatcher(a.keys)
	var handle interact.OverlayHandle
	picker.OnSelect = func(m availableModelItem) {
		handle.Hide()
		a.bgCall("set_model", func(ctx context.Context) (client.Response, error) {
			return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdSetModel, "", map[string]any{
				"provider": m.Provider,
				"modelId":  m.ID,
			}))
		}, "")
		a.requestRender(renderer.ReasonFlush)
	}
	picker.OnCancel = func() {
		handle.Hide()
		a.requestRender(renderer.ReasonFlush)
	}

	handle = a.showPickerOverlay("Select Model", 70, picker)
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) showPickerOverlay(title string, width float64, picker component.Component) interact.OverlayHandle {
	box := interact.NewBox(1, 0, newStaticText(bold(title)), picker)
	border := interact.DefaultBoxBorder()
	border.Color = interact.Style(a.themes.theme.Border)
	box.SetBorder(&border)
	box.SetFocusTarget(picker)

	w := interact.SizePct(width)
	maxHeight := interact.SizeAbs(14)
	handle := a.overlays.Show(box, interact.OverlayOptions{
		Anchor:        interact.AnchorCenter,
		Width:         &w,
		MaxHeight:     &maxHeight,
		UniformMargin: 1,
	})
	a.overlays.SetFocus(picker, a.width, a.height)
	return handle
}

func (a *App) showHistorySearch() {
	entries := a.ed.History()
	if len(entries) == 0 {
		a.setLocalNotice("prompt history is empty")
		return
	}
	th := interact.SelectListTheme{Cursor: a.themes.theme.Accent("❯")}
	type histItem struct {
		text string
	}
	items := make([]histItem, len(entries))
	for i, e := range entries {
		items[i] = histItem{text: e}
	}
	picker := interact.NewSelectList(items, 10, th, func(it histItem) interact.SelectItem {
		return interact.SelectItem{Label: it.text}
	}, interact.SelectListLayoutOptions{})

	picker.SetKeyMatcher(a.keys)
	var handle interact.OverlayHandle
	picker.OnSelect = func(it histItem) {
		handle.Hide()
		a.ed.SetText(it.text)
		a.requestRender(renderer.ReasonFlush)
	}
	picker.OnCancel = func() {
		handle.Hide()
		a.requestRender(renderer.ReasonFlush)
	}

	handle = a.showPickerOverlay("Search History", 80, picker)
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) retryLast() {
	if a.cli == nil {
		a.setLocalError("retry: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	a.bgCall("prompt", func(ctx context.Context) (client.Response, error) {
		return a.cli.Prompt(ctx, "/retry")
	}, "")
}

type dequeuedMessage struct {
	Text   string               `json:"text"`
	Images []pendingPromptImage `json:"images,omitempty"`
}

func decodeDequeuedMessages(resp client.Response) ([]dequeuedMessage, error) {
	if resp.Command != "" && resp.Command != protocol.CmdDequeueMessages {
		return nil, fmt.Errorf("unexpected response command %q", resp.Command)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("missing response data")
	}
	var payload struct {
		Messages []dequeuedMessage `json:"messages"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return payload.Messages, nil
}
func (a *App) restoreDequeuedMessages(messages []dequeuedMessage) {
	if len(messages) == 0 {
		a.setLocalNotice("No queued messages to restore")
		return
	}

	parts := make([]string, 0, len(messages)+1)
	for _, message := range messages {
		if message.Text != "" {
			parts = append(parts, message.Text)
		}
		for _, image := range message.Images {
			a.pendingPromptImages = append(a.pendingPromptImages, image)
		}

	}
	if draft := a.ed.Text(); draft != "" {
		parts = append(parts, draft)
	}
	a.ed.SetText(strings.Join(parts, "\n"))
	a.setLocalNotice(fmt.Sprintf("Restored %d queued message(s) to editor", len(messages)))
}

func (a *App) dequeueMessage() {
	if a.cli == nil {
		a.setLocalError("dequeue: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	if !a.bgCallWithCompletion("dequeue_messages", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdDequeueMessages, "", nil))
	}, "", func(a *App, d rpcDone) {
		if d.err != nil || !d.resp.Success {
			return
		}
		messages, err := decodeDequeuedMessages(d.resp)
		if err != nil {
			a.setLocalError("dequeue: " + err.Error())
			return
		}
		a.restoreDequeuedMessages(messages)
	}) {
		a.setLocalError("dequeue: RPC worker unavailable")
		a.requestRender(renderer.ReasonUpdate)
	}
}

func (a *App) togglePlanMode() {
	if a.cli == nil {
		a.setLocalError("plan: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	a.bgCall("prompt", func(ctx context.Context) (client.Response, error) {
		return a.cli.Prompt(ctx, "/plan")
	}, "")
}

type subagentViewItem struct {
	id          string
	agent       string
	status      string
	task        string
	sessionFile string
}

func (a *App) showAgentsHub(requireContent ...bool) {

	if a.cli == nil {
		a.setLocalError("get_subagents: core connection unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	if !a.bgCallWithCompletion("get_subagents", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetSubagents, "", nil))
	}, "", func(a *App, d rpcDone) {
		if d.err != nil || !d.resp.Success {
			return
		}
		items, err := decodeSubagentViewItems(d.resp)
		if err != nil {
			a.setLocalError("get_subagents: " + err.Error())
			return
		}
		if len(requireContent) > 0 && requireContent[0] && len(items) == 0 {
			return
		}

		a.openAgentsHub(items)
	}) {
		a.setLocalError("get_subagents: RPC worker unavailable")
		a.requestRender(renderer.ReasonUpdate)
	}
}

func decodeSubagentViewItems(resp client.Response) ([]subagentViewItem, error) {
	if resp.Command != "" && resp.Command != protocol.CmdGetSubagents {
		return nil, fmt.Errorf("unexpected response command %q", resp.Command)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("missing response data")
	}
	var payload struct {
		Subagents []protocol.SubagentSnapshot `json:"subagents"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	items := make([]subagentViewItem, len(payload.Subagents))
	for i, subagent := range payload.Subagents {
		if strings.TrimSpace(subagent.ID) == "" {
			return nil, fmt.Errorf("subagent %d is missing id", i+1)
		}
		status, err := decodeSubagentStatus(subagent.Status)
		if err != nil {
			return nil, fmt.Errorf("subagent %d: %w", i+1, err)
		}
		task := subagent.Task
		if task == "" {
			task = subagent.Assignment
		}
		if task == "" {
			task = subagent.Description
		}
		items[i] = subagentViewItem{
			id:          subagent.ID,
			agent:       subagent.Agent,
			status:      status,
			task:        task,
			sessionFile: subagent.SessionFile,
		}

	}
	return items, nil
}

func decodeSubagentStatus(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return "", nil
	}
	var status string
	if err := json.Unmarshal(raw, &status); err != nil {
		return "", fmt.Errorf("decode status: %w", err)
	}
	return status, nil
}

func (a *App) openAgentsHub(items []subagentViewItem) {
	th := interact.SelectListTheme{Cursor: a.themes.theme.Accent("❯")}
	picker := interact.NewSelectList(items, 10, th, func(it subagentViewItem) interact.SelectItem {
		label := it.id
		if it.agent != "" {
			label = it.agent + " · " + it.id
		}
		if it.status != "" {
			label += " (" + it.status + ")"
		}
		return interact.SelectItem{
			Label:       label,
			Description: it.task,
		}
	}, interact.SelectListLayoutOptions{})

	picker.SetKeyMatcher(a.keys)
	var handle interact.OverlayHandle
	picker.OnSelect = func(item subagentViewItem) {
		handle.Hide()
		a.focusAgentSession(item)
	}

	picker.OnCancel = func() {
		handle.Hide()
		a.requestRender(renderer.ReasonFlush)
	}

	handle = a.showPickerOverlay("Agent Hub", 70, picker)
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) focusAgentSession(item subagentViewItem) {
	target := strings.TrimSpace(item.sessionFile)
	if target == "" {
		target = item.id
	}
	if target == "" {
		a.setLocalError("focus agent: session target is unavailable")
		a.requestRender(renderer.ReasonUpdate)
		return
	}
	if !a.bgCallWithCompletion("switch_session", func(ctx context.Context) (client.Response, error) {
		return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdSwitchSession, "", map[string]any{
			"sessionPath": target,
		}))
	}, "", func(a *App, d rpcDone) {
		if d.err != nil || !d.resp.Success {
			return
		}
		a.bgCall("get_state", a.cli.GetState, "")
		a.bgCall("get_messages", func(ctx context.Context) (client.Response, error) {
			return a.cli.Call(ctx, protocol.BuildRPCCommand(protocol.CmdGetMessages, "", nil))
		}, "")
		a.setLocalNotice("focused agent session " + item.id)
	}) {
		a.setLocalError("focus agent: RPC worker unavailable")
		a.requestRender(renderer.ReasonUpdate)
	}
}

func (a *App) openExternalEditor() {
	editorBin := os.Getenv("VISUAL")
	if editorBin == "" {
		editorBin = os.Getenv("EDITOR")
	}
	if editorBin == "" {
		editorBin = "vim"
	}

	tmpFile, err := os.CreateTemp("", "omp_edit_*.txt")
	if err != nil {
		a.setLocalError(fmt.Sprintf("external editor temp file error: %v", err))
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	initialText := a.ed.Text()
	if _, err := tmpFile.WriteString(initialText); err != nil {
		tmpFile.Close()
		a.setLocalError(fmt.Sprintf("external editor write error: %v", err))
		return
	}
	tmpFile.Close()

	if a.term != nil {
		_ = a.term.Stop()
	}

	cmd := exec.Command(editorBin, tmpPath)
	if a.tty != nil && a.tty.In != nil && a.tty.Out != nil {
		cmd.Stdin = a.tty.In
		cmd.Stdout = a.tty.Out
		cmd.Stderr = a.tty.Out
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	runErr := cmd.Run()

	if a.term != nil {
		termCtx := a.ctx
		if termCtx == nil {
			termCtx = context.Background()
		}
		_ = a.term.Start(termCtx)
	}
	a.resetDisplay()

	if runErr != nil {
		a.setLocalError(fmt.Sprintf("external editor exit error: %v", runErr))
		return
	}

	editedBytes, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		a.setLocalError(fmt.Sprintf("external editor read error: %v", readErr))
		return
	}

	a.ed.SetText(string(editedBytes))
	a.setLocalNotice("updated text from external editor")
	a.requestRender(renderer.ReasonFlush)
}

type pendingPromptImage struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MIMEType string `json:"mimeType"`
}

func (a *App) handlePasteImage() {
	base64Data, mimeType, err := a.clip.ReadImage()
	if err == nil && base64Data != "" {
		if strings.TrimSpace(mimeType) == "" {
			a.setLocalError("clipboard image is missing a MIME type")
			a.requestRender(renderer.ReasonUpdate)
			return
		}
		a.pendingPromptImages = append(a.pendingPromptImages, pendingPromptImage{
			Type:     "image",
			Data:     base64Data,
			MIMEType: mimeType,
		})
		a.setLocalNotice("attached image from clipboard")
		a.requestRender(renderer.ReasonFlush)
		return
	}
	text, textErr := a.clip.ReadText()
	if textErr == nil && text != "" {
		a.ed.PasteText(text)
		a.setLocalNotice("pasted text from clipboard")
		a.requestRender(renderer.ReasonFlush)
		return
	}
	a.setLocalNotice("clipboard is empty or unavailable")
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) handlePasteTextRaw() {
	text, err := a.clip.ReadText()
	if err != nil || text == "" {
		a.setLocalNotice("clipboard text is empty or unavailable")
		a.requestRender(renderer.ReasonFlush)
		return
	}
	a.ed.PasteText(text)
	a.setLocalNotice("pasted raw text from clipboard")
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) handleCopyLine() {
	line := a.ed.CurrentLine()
	if strings.TrimSpace(line) == "" {
		a.setLocalNotice("current line is empty")
		return
	}
	err := a.clip.WriteText(line, a.writeTermCopy)
	if err != nil {
		a.setLocalError(fmt.Sprintf("copy failed: %v", err))
	} else {
		a.setLocalNotice("copied line to clipboard")
	}
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) handleCopyPrompt() {
	text := a.ed.Text()
	if strings.TrimSpace(text) == "" {
		a.setLocalNotice("prompt is empty")
		return
	}
	err := a.clip.WriteText(text, a.writeTermCopy)
	if err != nil {
		a.setLocalError(fmt.Sprintf("copy failed: %v", err))
	} else {
		a.setLocalNotice("copied prompt to clipboard")
	}
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) writeTermCopy(osc52 string) {
	if a.tty != nil && a.tty.Out != nil {
		_, _ = a.tty.Out.WriteString(osc52)
	}
}
