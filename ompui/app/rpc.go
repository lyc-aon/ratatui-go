package app

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/michaelkelly/ratatui-go/ompui/client"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/event"
	"github.com/michaelkelly/ratatui-go/ompui/interact"
	"github.com/michaelkelly/ratatui-go/ompui/protocol"
	"github.com/michaelkelly/ratatui-go/ompui/renderer"
)

func (a *App) handleRPCEvent(ev client.Event) {
	if ev.Err != nil {
		a.logf("rpc event error: %v", ev.Err)
	}
	if ev.Envelope.Type == client.TypeReady {
		return
	}

	switch ev.Kind {
	case protocol.KindRPCResponse:
		if ev.Response != nil {
			a.applyResponse(client.Response{
				ID: ev.Response.ID, Command: ev.Response.Command,
				Success: ev.Response.Success, Data: ev.Response.Data,
				Error: ev.Response.Error, Raw: ev.Response.Raw,
			})
		} else if len(ev.Raw) > 0 {
			if env, err := protocol.WrapHistorical(ev.Raw); err == nil {
				_, _ = a.state.Apply(env)
				a.syncFromSnapshot(false)
			}
		}
		a.requestRender(renderer.ReasonUpdate)
		return

	case protocol.KindExtensionUIRequest:
		a.handleExtensionUI(ev)
		return

	case protocol.KindComponent:
		a.handleComponentFrame(ev)
		return

	case protocol.KindOverlay:
		a.handleOverlayFrame(ev)
		return

	case protocol.KindHostTool, protocol.KindHostURI:
		a.handleHostFrame(ev)
		return

	case protocol.KindEditor:
		a.handleEditorFrame(ev)
		return

	case protocol.KindTheme:
		a.handleThemeFrame(ev)
		return

	case protocol.KindTerminalInput:
		a.handleTerminalInputFrame(ev)
		return

	case protocol.KindShutdown:
		a.quitReason = "peer-shutdown"
		a.cancel()
		return
	}

	typ := ev.Envelope.Type
	if typ == "" && len(ev.Raw) > 0 {
		var err error
		ev.Envelope, err = protocol.WrapHistorical(ev.Raw)
		if err != nil {
			return
		}
		typ = ev.Envelope.Type
	}
	// Late classify for bare frames that arrived as KindUnknown.
	switch protocol.ClassifyType(typ) {
	case protocol.KindTerminalInput:
		a.handleTerminalInputFrame(ev)
		return
	case protocol.KindComponent:
		a.handleComponentFrame(ev)
		return
	}

	if typ != "" {
		if _, err := a.state.Apply(ev.Envelope); err != nil {
			a.logf("model apply %s: %v", typ, err)
		}
		a.syncFromSnapshot(false)
		a.requestRender(renderer.ReasonUpdate)
	}
}

func (a *App) handleEditorFrame(ev client.Event) {
	raw := ev.Raw
	if len(raw) == 0 {
		raw = ev.Envelope.HistoricalPayload()
	}
	switch ev.Envelope.Type {
	case protocol.MsgEditorUpdate:
		var p protocol.EditorUpdatePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			_ = protocol.DecodePayload(ev.Envelope, &p)
		}
		switch p.Op {
		case protocol.EditorOpSetText:
			a.ed.SetText(p.Text)
			if p.Cursor != nil {
				a.ed.SetCursor(editorCursorFromUTF16Offset(p.Text, *p.Cursor))
			}
		case protocol.EditorOpPaste:
			a.ed.PasteText(p.Text)
		case protocol.EditorOpClear:
			a.ed.SetText("")
		case protocol.EditorOpSetCursor:
			if p.Cursor != nil {
				a.ed.SetCursor(editorCursorFromUTF16Offset(a.ed.Text(), *p.Cursor))
			}
		}
		a.editorDirty = true
		a.requestRender(renderer.ReasonFlush)
	case protocol.MsgEditorQuery:
		a.pushEditorState()
	case protocol.MsgEditorState:
		var p protocol.EditorStatePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			_ = protocol.DecodePayload(ev.Envelope, &p)
		}
		if p.Text != a.ed.Text() {
			a.ed.SetText(p.Text)
		}
		cursorRaw := raw
		if ev.Envelope.V != 0 && len(ev.Envelope.Payload) > 0 {
			cursorRaw = ev.Envelope.Payload
		}
		var cursorField struct {
			Cursor *int `json:"cursor"`
		}
		if json.Unmarshal(cursorRaw, &cursorField) == nil && cursorField.Cursor != nil {
			a.ed.SetCursor(editorCursorFromUTF16Offset(p.Text, *cursorField.Cursor))
		}
		a.requestRender(renderer.ReasonFlush)
	}
}

func (a *App) handleThemeFrame(ev client.Event) {
	// theme_query is outbound-only from Go.
	if ev.Envelope.Type == protocol.MsgThemeQuery {
		return
	}
	if ev.Envelope.Type != protocol.MsgThemeSync && ev.Envelope.Type != "" {
		a.logf("theme frame ignored type=%s", ev.Envelope.Type)
		return
	}
	raw := ev.Raw
	if len(raw) == 0 {
		raw = ev.Envelope.HistoricalPayload()
	}
	var p protocol.ThemeSyncPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		if err2 := protocol.DecodePayload(ev.Envelope, &p); err2 != nil {
			a.logf("theme_sync decode: %v", err)
			return
		}
	}
	a.logf("theme_sync name=%q appearance=%q palette=%v", p.Name, p.Appearance, p.Palette != nil)
	a.applyThemeSync(p)
}

// handleComponentFrame routes remote component protocol into component.Remote.
func (a *App) handleComponentFrame(ev client.Event) {
	typ := ev.Envelope.Type
	raw := ev.Raw
	if len(raw) == 0 {
		raw = ev.Envelope.HistoricalPayload()
	}
	switch typ {
	case protocol.MsgComponentOpen:
		var p protocol.ComponentOpenPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			_ = protocol.DecodePayload(ev.Envelope, &p)
		}
		if p.ComponentID == "" {
			return
		}
		a.mountRemote(p.ComponentID, p.Slot, p.WantsKeyRelease)

	case protocol.MsgComponentResult:
		var p protocol.ComponentResultPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			_ = protocol.DecodePayload(ev.Envelope, &p)
		}
		if p.ComponentID == "" {
			return
		}
		// Drop stale generations.
		if p.Generation > 0 {
			if last, ok := a.remoteResultG[p.ComponentID]; ok && p.Generation < last {
				return
			}
			if reqG, ok := a.remoteGen[p.ComponentID]; ok && p.Generation < reqG {
				// Older than latest request — drop.
				return
			}
			a.remoteResultG[p.ComponentID] = p.Generation
		}
		r := a.ensureRemote(p.ComponentID)
		if p.Error != "" {
			r.SetLines([]string{danger(p.Error)})
		} else {
			var cur *component.Cursor
			if p.CursorRow != nil || p.CursorCol != nil {
				cur = &component.Cursor{}
				if p.CursorRow != nil {
					cur.Row = *p.CursorRow
				}
				if p.CursorCol != nil {
					cur.Column = *p.CursorCol
				}
			}
			live, commit, snap := component.BoundaryUnset, component.BoundaryUnset, component.BoundaryUnset
			if p.LiveRegionStart != nil {
				live = *p.LiveRegionStart
			}
			if p.CommitSafeEnd != nil {
				commit = *p.CommitSafeEnd
			}
			if p.SnapshotSafeEnd != nil {
				snap = *p.SnapshotSafeEnd
			}
			if live != component.BoundaryUnset || commit != component.BoundaryUnset || snap != component.BoundaryUnset {
				r.Apply(p.Lines, cur, live, commit, snap)
			} else if cur != nil {
				fr := component.NewFrame(p.Lines, 0).WithCursor(cur)
				r.SetFrame(fr)
			} else {
				r.SetLines(p.Lines)
			}
		}
		a.requestRender(renderer.ReasonUpdate)

	case protocol.MsgComponentInvalidate:
		var p protocol.ComponentInvalidatePayload
		_ = json.Unmarshal(raw, &p)
		if r := a.remotes[p.ComponentID]; r != nil {
			r.Invalidate()
			// Re-request render from Bun.
			a.requestRemoteRender(p.ComponentID)
			a.requestRender(renderer.ReasonUpdate)
		}

	case protocol.MsgComponentDispose:
		var p protocol.ComponentDisposePayload
		_ = json.Unmarshal(raw, &p)
		a.disposeRemote(p.ComponentID, false)

	case protocol.MsgComponentFocus:
		var p protocol.ComponentFocusPayload
		_ = json.Unmarshal(raw, &p)
		if r := a.remotes[p.ComponentID]; r != nil {
			r.SetFocused(p.Focused)
			if p.Focused {
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

	case protocol.MsgComponentInput:
		// Host→core direction normally; ignore inbound.
	case protocol.MsgComponentRender:
		// Host→core direction normally; ignore inbound.
	case protocol.MsgComponentInputResult:
		a.handleComponentInputResult(raw, ev.Envelope)
	case protocol.MsgComponentFocusRequest:
		a.handleComponentFocusRequest(raw, ev.Envelope)
	default:
		a.logf("component frame %s", typ)
	}
}

func (a *App) handleOverlayFrame(ev client.Event) {
	raw := ev.Raw
	if len(raw) == 0 {
		raw = ev.Envelope.HistoricalPayload()
	}
	switch ev.Envelope.Type {
	case protocol.MsgOverlayMount:
		var p protocol.OverlayMountPayload
		_ = json.Unmarshal(raw, &p)
		if p.OverlayID == "" {
			return
		}
		// Prefer backing remote component if given.
		var c component.Component
		if p.ComponentID != "" {
			c = a.ensureRemote(p.ComponentID)
			// component_open slot=overlay already auto-showed this Remote via
			// slotRemote/extDialogs. Tear that handle so protocolOverlays is sole mount.
			if d, ok := a.extDialogs[p.ComponentID]; ok && d != nil && d.method == "component" {
				if d.handle.Component() != nil {
					d.handle.Hide()
				}
				delete(a.extDialogs, p.ComponentID)
			}
		} else {
			c = newStaticText(bold(p.Title))
		}
		opt := overlayOptionsFromMount(p, a.width)
		// Replace existing protocol mount (idempotent remount).
		if h, ok := a.protocolOverlays[p.OverlayID]; ok {
			h.Hide()
		}
		handle := a.overlays.Show(c, opt)
		a.protocolOverlays[p.OverlayID] = handle
		a.overlays.SetFocus(c, a.width, a.height)
		a.requestRender(renderer.ReasonFlush)

	case protocol.MsgOverlayUnmount:
		var p protocol.OverlayUnmountPayload
		_ = json.Unmarshal(raw, &p)
		if h, ok := a.protocolOverlays[p.OverlayID]; ok {
			h.Hide()
			delete(a.protocolOverlays, p.OverlayID)
			a.overlays.SetFocus(a.ed, a.width, a.height)
			a.requestRender(renderer.ReasonFlush)
		}

	case protocol.MsgOverlayUpdate:
		var p protocol.OverlayUpdatePayload
		_ = json.Unmarshal(raw, &p)
		h, ok := a.protocolOverlays[p.OverlayID]
		if !ok || p.OverlayID == "" {
			a.requestRender(renderer.ReasonUpdate)
			return
		}
		// mode=hidden → SetHidden(true); modal/alt_screen/inline → unhide.
		// Nested OMP overlay uses update hidden then later remount.
		if p.Mode != nil {
			switch *p.Mode {
			case protocol.OverlayModeHidden:
				h.SetHidden(true)
			case protocol.OverlayModeModal, protocol.OverlayModeAltScreen, protocol.OverlayModeInline, "":
				h.SetHidden(false)
			default:
				// Unknown mode: still try unhide so chrome is not stuck hidden.
				h.SetHidden(false)
			}
		}
		// Title/zIndex: stack has no chrome setters yet; visibility is the critical path.
		_ = p.Title
		_ = p.ZIndex
		a.requestRender(renderer.ReasonUpdate)
	}
}

func (a *App) ensureRemote(id string) *component.Remote {
	if r := a.remotes[id]; r != nil {
		return r
	}
	return a.mountRemote(id, protocol.SlotOverlay, false)
}

func (a *App) mountRemote(id string, slot protocol.ComponentSlot, wantsRelease bool) *component.Remote {
	if r := a.remotes[id]; r != nil {
		if slot != "" && a.remoteSlots[id] != slot {
			a.unslotRemote(id)
			a.remoteSlots[id] = slot
			a.slotRemote(id, r, slot)
		}
		return r
	}
	r := component.NewRemote(id)
	r.WantsReleases = wantsRelease
	r.OnWidth = func(width int) {
		a.requestRemoteRenderAt(id, width)
	}
	r.OnInput = func(ev event.Event) {
		if a.cli == nil {
			return
		}
		// Send Data as []byte (JSON base64 via encoding/json).
		fields := map[string]any{"componentId": id}
		if len(ev.Raw) > 0 {
			fields["data"] = ev.Raw // []byte → base64 on wire
		}
		if ev.Text != "" {
			fields["text"] = ev.Text
		}
		_ = a.cli.SendCommand(protocol.MsgComponentInput, fields)
	}
	a.remotes[id] = r
	if slot == "" {
		slot = protocol.SlotOverlay
	}
	a.remoteSlots[id] = slot
	a.slotRemote(id, r, slot)
	a.requestRemoteRender(id)
	a.requestRender(renderer.ReasonFlush)
	return r
}

func (a *App) requestRemoteRender(id string) {
	w := a.width
	if w < 1 {
		w = 80
	}
	if r := a.remotes[id]; r != nil {
		if lw, ok := r.LastWidth(); ok && lw > 0 {
			w = lw
		}
	}
	a.requestRemoteRenderAt(id, w)
}

func (a *App) requestRemoteRenderAt(id string, width int) {
	if a.cli == nil {
		return
	}
	if width < 1 {
		width = a.width
	}
	if width < 1 {
		width = 80
	}
	h := a.height
	if h < 1 {
		h = 24
	}
	a.remoteGen[id]++
	gen := a.remoteGen[id]
	_ = a.cli.SendCommand(protocol.MsgComponentRender, map[string]any{
		"componentId":    id,
		"width":          width,
		"height":         h,
		"terminalWidth":  a.width,
		"terminalHeight": h,
		"generation":     gen,
	})
}

func (a *App) slotRemote(id string, r *component.Remote, slot protocol.ComponentSlot) {
	switch slot {
	case protocol.SlotHeader:
		a.headerRemote = r
		a.rebuildRoot()
	case protocol.SlotFooter:
		a.footerRemote = r
		a.rebuildRoot()
	case protocol.SlotWidgetAbove:
		a.widgetAbove = append(a.widgetAbove, r)
		a.rebuildRoot()
	case protocol.SlotWidgetBelow:
		a.widgetBelow = append(a.widgetBelow, r)
		a.rebuildRoot()
	case protocol.SlotEditor:
		// Truly replace local editor in root.
		a.editorRemote = r
		r.SetFocused(true)
		a.ed.SetFocused(false)
		a.rebuildRoot()
	case protocol.SlotOverlay, protocol.SlotCustom, protocol.SlotToolCall, protocol.SlotToolResult, "":
		w := interact.SizeAbs(minInt(a.width-4, 80))
		opt := interact.OverlayOptions{Anchor: interact.AnchorCenter, Width: &w, UniformMargin: 1}
		handle := a.overlays.Show(r, opt)
		a.extDialogs[id] = &extDialog{id: id, method: "component", handle: handle, comp: r}
		a.overlays.SetFocus(r, a.width, a.height)
	default:
		w := interact.SizeAbs(minInt(a.width-4, 80))
		opt := interact.OverlayOptions{Anchor: interact.AnchorCenter, Width: &w, UniformMargin: 1}
		handle := a.overlays.Show(r, opt)
		a.extDialogs[id] = &extDialog{id: id, method: "component", handle: handle, comp: r}
	}
}

func (a *App) unslotRemote(id string) {
	slot := a.remoteSlots[id]
	r := a.remotes[id]
	switch slot {
	case protocol.SlotHeader:
		if a.headerRemote == r {
			a.headerRemote = nil
			a.rebuildRoot()
		}
	case protocol.SlotFooter:
		if a.footerRemote == r {
			a.footerRemote = nil
			a.rebuildRoot()
		}
	case protocol.SlotEditor:
		if a.editorRemote == r {
			a.editorRemote = nil
			a.ed.SetFocused(true)
			a.rebuildRoot()
		}
	case protocol.SlotWidgetAbove, protocol.SlotWidgetBelow:
		a.widgetAbove = filterComp(a.widgetAbove, r)
		a.widgetBelow = filterComp(a.widgetBelow, r)
		a.rebuildRoot()
	default:
		if d, ok := a.extDialogs[id]; ok {
			d.handle.Hide()
			delete(a.extDialogs, id)
		}
	}
}

// disposeRemote tears down a remote. notifyPeer sends component_dispose + focus out.
func (a *App) disposeRemote(id string, notifyPeer bool) {
	if id == "" {
		return
	}
	if notifyPeer && a.cli != nil {
		_ = a.cli.SendCommand(protocol.MsgComponentFocus, map[string]any{
			"componentId": id, "focused": false,
		})
		_ = a.cli.SendCommand(protocol.MsgComponentDispose, map[string]any{
			"componentId": id,
		})
	}
	a.unslotRemote(id)
	if r := a.remotes[id]; r != nil {
		r.Dispose()
		delete(a.remotes, id)
	}
	delete(a.remoteSlots, id)
	delete(a.remoteGen, id)
	delete(a.remoteResultG, id)
	if a.editorRemote == nil {
		a.overlays.SetFocus(a.ed, a.width, a.height)
		a.ed.SetFocused(true)
	}
	a.requestRender(renderer.ReasonUpdate)
}

func filterComp(in []component.Component, drop component.Component) []component.Component {
	if len(in) == 0 {
		return in
	}
	out := in[:0]
	for _, c := range in {
		if c != drop {
			out = append(out, c)
		}
	}
	for i := len(out); i < len(in); i++ {
		in[i] = nil
	}
	return out
}

func (a *App) handleHostFrame(ev client.Event) {
	switch ev.Envelope.Type {
	case protocol.MsgHostToolCall:
		var call protocol.HostToolCall
		raw := ev.Raw
		if len(raw) == 0 {
			raw = ev.Envelope.HistoricalPayload()
		}
		_ = json.Unmarshal(raw, &call)
		if call.ID != "" {
			errBody, _ := json.Marshal(map[string]string{"error": "omp-tui does not expose host tools"})
			_ = a.cli.SendHostToolResult(protocol.HostToolResult{
				Type: protocol.MsgHostToolResult, ID: call.ID,
				Result: errBody, IsError: true,
			})
		}
	case protocol.MsgHostURIRequest:
		var req protocol.HostURIRequest
		raw := ev.Raw
		if len(raw) == 0 {
			raw = ev.Envelope.HistoricalPayload()
		}
		_ = json.Unmarshal(raw, &req)
		if req.ID != "" {
			_ = a.cli.SendHostURIResult(protocol.HostURIResult{
				Type: protocol.MsgHostURIResult, ID: req.ID,
				IsError: true, Error: "omp-tui does not expose host URI schemes",
			})
		}
	}
}

// overlayOptionsFromMount maps OverlayMountPayload → interact.OverlayOptions.
// Defaults match prior host behavior when fields are omitted/malformed:
// center anchor, width min(term-4,80), uniform margin 1.
// fullscreen or mode=alt_screen borrows alt screen. Bounds clamp stays in interact.
func overlayOptionsFromMount(p protocol.OverlayMountPayload, termWidth int) interact.OverlayOptions {
	if termWidth < 1 {
		termWidth = 80
	}
	defW := interact.SizeAbs(minInt(termWidth-4, 80))
	opt := interact.OverlayOptions{
		Anchor:        interact.AnchorCenter,
		Width:         &defW,
		UniformMargin: 1,
	}

	if sv, ok := parseOverlaySizeRaw(p.Width); ok {
		opt.Width = &sv
	}
	if p.MinWidth != nil {
		opt.MinWidth = *p.MinWidth
	}
	if sv, ok := parseOverlaySizeRaw(p.MaxHeight); ok {
		opt.MaxHeight = &sv
	}
	if a, ok := parseOverlayAnchor(p.Anchor); ok {
		opt.Anchor = a
	}
	if p.OffsetX != nil {
		opt.OffsetX = *p.OffsetX
	}
	if p.OffsetY != nil {
		opt.OffsetY = *p.OffsetY
	}
	if sv, ok := parseOverlaySizeRaw(p.Row); ok {
		opt.Row = &sv
	}
	if sv, ok := parseOverlaySizeRaw(p.Col); ok {
		opt.Col = &sv
	}
	if m, uni, ok := parseOverlayMarginRaw(p.Margin); ok {
		if m != nil {
			opt.Margin = m
			opt.UniformMargin = -1
		} else {
			opt.UniformMargin = uni
			opt.Margin = nil
		}
	}

	fs := p.Mode == protocol.OverlayModeAltScreen
	if p.Fullscreen != nil {
		fs = *p.Fullscreen || fs
	}
	opt.Fullscreen = fs
	return opt
}

func parseOverlaySizeRaw(raw json.RawMessage) (interact.SizeValue, bool) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return interact.SizeValue{}, false
	}
	// JSON number
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return interact.SizeAbs(int(n)), true
	}
	// JSON string: "50%" or "40"
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return interact.SizeValue{}, false
	}
	return interact.ParseSizeValue(s)
}

func parseOverlayAnchor(s string) (interact.OverlayAnchor, bool) {
	switch interact.OverlayAnchor(strings.TrimSpace(s)) {
	case interact.AnchorCenter,
		interact.AnchorTopLeft,
		interact.AnchorTopRight,
		interact.AnchorBottomLeft,
		interact.AnchorBottomRight,
		interact.AnchorTopCenter,
		interact.AnchorBottomCenter,
		interact.AnchorLeftCenter,
		interact.AnchorRightCenter:
		return interact.OverlayAnchor(strings.TrimSpace(s)), true
	default:
		return "", false
	}
}

// parseOverlayMarginRaw accepts a uniform number or {top,right,bottom,left}.
// ok false → caller keeps default uniform margin 1.
func parseOverlayMarginRaw(raw json.RawMessage) (m *interact.OverlayMargin, uniform int, ok bool) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, 0, false
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return nil, int(n), true
	}
	// Numeric string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if v, err2 := strconv.Atoi(strings.TrimSpace(s)); err2 == nil {
			return nil, v, true
		}
		return nil, 0, false
	}
	var obj struct {
		Top    *int `json:"top"`
		Right  *int `json:"right"`
		Bottom *int `json:"bottom"`
		Left   *int `json:"left"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, 0, false
	}
	if obj.Top == nil && obj.Right == nil && obj.Bottom == nil && obj.Left == nil {
		return nil, 0, false
	}
	mm := interact.OverlayMargin{}
	if obj.Top != nil {
		mm.Top = *obj.Top
	}
	if obj.Right != nil {
		mm.Right = *obj.Right
	}
	if obj.Bottom != nil {
		mm.Bottom = *obj.Bottom
	}
	if obj.Left != nil {
		mm.Left = *obj.Left
	}
	return &mm, 0, true
}

// Silence unused import if event only used in OnInput signature via other file.
var _ = event.KindNone
