package app

import (
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/editor"
	"github.com/lyc-aon/ratatui-go/ompui/interact"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
	ompruntime "github.com/lyc-aon/ratatui-go/ompui/runtime"
)

func (a *App) handleExtensionUI(ev client.Event) {
	var req protocol.ExtensionUIRequest
	raw := ev.Raw
	if len(raw) == 0 {
		raw = ev.Envelope.HistoricalPayload()
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		a.logf("extension_ui decode: %v", err)
		return
	}

	switch req.Method {
	case protocol.ExtUISelect:
		a.openSelectDialog(req)
	case protocol.ExtUIConfirm:
		a.openConfirmDialog(req)
	case protocol.ExtUIInput:
		a.openInputDialog(req)
	case protocol.ExtUIEditor:
		a.openEditorDialog(req)
	case protocol.ExtUICancel:
		a.cancelExtension(req.TargetID, false)
	case protocol.ExtUINotify:
		msg := req.Message
		if msg == "" {
			msg = req.Title
		}
		if req.NotifyType == "error" {
			a.setLocalError(msg)
		} else {
			a.setLocalNotice(msg)
			if a.term != nil {
				_ = a.term.Notify(ompruntime.Notification{Title: "omp", Body: msg})
			}
		}
		a.requestRender(renderer.ReasonUpdate)
	case protocol.ExtUISetStatus:
		entries := map[string]string{}
		var clearKeys []string
		if req.StatusKey != "" {
			if req.StatusText == nil || *req.StatusText == "" {
				clearKeys = append(clearKeys, req.StatusKey)
			} else {
				entries[req.StatusKey] = *req.StatusText
			}
		}
		frame, _ := json.Marshal(map[string]any{
			"type": protocol.MsgStatusSync, "entries": entries, "clearKeys": clearKeys,
		})
		if env, err := protocol.WrapHistorical(frame); err == nil {
			_, _ = a.state.Apply(env)
		}
		a.syncFromSnapshot(false)
		a.requestRender(renderer.ReasonUpdate)
	case protocol.ExtUISetWidget:
		var lines []string
		if len(req.WidgetLines) > 0 && string(req.WidgetLines) != "null" {
			_ = json.Unmarshal(req.WidgetLines, &lines)
		}
		key := req.WidgetKey
		if key == "" {
			key = "default"
		}
		placement := req.WidgetPlacement
		if placement != "belowEditor" {
			placement = "aboveEditor"
		}
		if existing := a.extensionWidgets[key]; existing != nil {
			if len(lines) > 0 && a.extensionWidgetSlots[key] == placement {
				existing.SetLines(lines)
				a.requestRender(renderer.ReasonUpdate)
				break
			}
			a.widgetAbove = filterComp(a.widgetAbove, existing)
			a.widgetBelow = filterComp(a.widgetBelow, existing)
			existing.Dispose()
			delete(a.extensionWidgets, key)
			delete(a.extensionWidgetSlots, key)
		}
		if len(lines) > 0 {
			local := component.NewRemote("extension-widget-" + key)
			local.SetLines(lines)
			a.extensionWidgets[key] = local
			a.extensionWidgetSlots[key] = placement
			if placement == "belowEditor" {
				a.widgetBelow = append(a.widgetBelow, local)
			} else {
				a.widgetAbove = append(a.widgetAbove, local)
			}
		}
		a.rebuildRoot()
		a.requestRender(renderer.ReasonUpdate)
	case protocol.ExtUISetTitle:
		a.title = req.Title
		if a.term != nil {
			_ = a.term.SetTitle(req.Title)
		}
	case protocol.ExtUISetEditorText:
		a.ed.SetText(req.Text)
		a.requestRender(renderer.ReasonFlush)
	case protocol.ExtUIOpenURL:
		url := req.URL
		go func(u string) {
			err := openURL(u)
			a.post(command{kind: cmdOpenURLDone, openURL: u, openURLErr: err})
		}(url)
		if req.Instructions != "" {
			a.setLocalNotice(req.Instructions)
			a.requestRender(renderer.ReasonUpdate)
		}
	default:
		// Unknown method that looks like a custom component → Remote host.
		a.logf("extension_ui unknown method %q id=%s — routing to Remote", req.Method, req.ID)
		if req.ID != "" {
			r := a.mountRemote(req.ID, protocol.SlotOverlay, false)
			if len(req.WidgetLines) > 0 {
				var lines []string
				if err := json.Unmarshal(req.WidgetLines, &lines); err == nil {
					r.SetLines(lines)
				}
			}
		}
	}
}

func (a *App) openSelectDialog(req protocol.ExtensionUIRequest) {
	opts := decodeStringOptions(req.Options)
	items := make([]interact.SelectItem, 0, len(opts))
	for _, o := range opts {
		items = append(items, interact.SelectItem{Value: o, Label: o})
	}
	if len(items) == 0 {
		_ = a.cli.ExtensionUIResponseCancelled(req.ID, false)
		return
	}
	list := interact.NewSelectItemList(items, 12, interact.SelectListTheme{Cursor: a.themes.theme.Accent("❯")}, interact.SelectListLayoutOptions{})
	id := req.ID
	list.OnSelect = func(item interact.SelectItem) {
		_ = a.cli.ExtensionUIResponseValue(id, item.Value)
		a.closeExtDialog(id)
		a.requestRender(renderer.ReasonFlush)
	}
	list.OnCancel = func() {
		_ = a.cli.ExtensionUIResponseCancelled(id, false)
		a.closeExtDialog(id)
		a.requestRender(renderer.ReasonFlush)
	}
	title := req.Title
	if title == "" {
		title = "Select"
	}
	box := interact.NewBox(1, 0, newStaticText(bold(title)), list)
	b := interact.DefaultBoxBorder()
	b.Color = interact.Style(a.themes.theme.Border)
	box.SetBorder(&b)
	box.SetFocusTarget(list)
	a.showExtDialog(req, box, list)
}

func (a *App) openConfirmDialog(req protocol.ExtensionUIRequest) {
	id := req.ID
	items := []interact.SelectItem{
		{Value: "yes", Label: "Yes"},
		{Value: "no", Label: "No"},
	}
	list := interact.NewSelectItemList(items, 4, interact.SelectListTheme{Cursor: a.themes.theme.Accent("❯")}, interact.SelectListLayoutOptions{})
	list.OnSelect = func(item interact.SelectItem) {
		_ = a.cli.ExtensionUIResponseConfirmed(id, item.Value == "yes")
		a.closeExtDialog(id)
		a.requestRender(renderer.ReasonFlush)
	}
	list.OnCancel = func() {
		_ = a.cli.ExtensionUIResponseCancelled(id, false)
		a.closeExtDialog(id)
		a.requestRender(renderer.ReasonFlush)
	}
	title := req.Title
	if title == "" {
		title = "Confirm"
	}
	children := []component.Component{newStaticText(bold(title))}
	if req.Message != "" {
		children = append(children, newStaticText(req.Message))
	}
	children = append(children, list)
	box := interact.NewBox(1, 0, children...)
	b := interact.DefaultBoxBorder()
	b.Color = interact.Style(a.themes.theme.Border)
	box.SetBorder(&b)
	box.SetFocusTarget(list)
	a.showExtDialog(req, box, list)
}

func (a *App) openInputDialog(req protocol.ExtensionUIRequest) {
	id := req.ID
	ed := editor.New(
		editor.WithPlaceholder(req.Placeholder),
		editor.WithBorder(true),
		editor.WithBorderColor(a.themes.theme.Border),
		editor.WithPromptPrefix(""),
		editor.WithMaxHeight(4),
		editor.WithOnSubmit(func(text string) {
			_ = a.cli.ExtensionUIResponseValue(id, text)
			a.closeExtDialog(id)
			a.requestRender(renderer.ReasonFlush)
		}),
		editor.WithOnInterrupt(func() {
			_ = a.cli.ExtensionUIResponseCancelled(id, false)
			a.closeExtDialog(id)
			a.requestRender(renderer.ReasonFlush)
		}),
	)
	if req.Prefill != "" {
		ed.SetText(req.Prefill)
	}
	ed.SetFocused(true)
	title := req.Title
	if title == "" {
		title = "Input"
	}
	box := interact.NewBox(1, 0, newStaticText(bold(title)), ed)
	b := interact.DefaultBoxBorder()
	b.Color = interact.Style(a.themes.theme.Border)
	box.SetBorder(&b)
	box.SetFocusTarget(ed)
	a.showExtDialog(req, box, ed)
}

func (a *App) openEditorDialog(req protocol.ExtensionUIRequest) {
	promptStyle := req.PromptStyle != nil && *req.PromptStyle
	submitMode := editor.SubmitOnCtrlEnter
	promptPrefix := ""
	if promptStyle {
		submitMode = editor.SubmitOnEnter
		promptPrefix = "> "
	}
	id := req.ID
	ed := editor.New(
		editor.WithPlaceholder(req.Placeholder),
		editor.WithBorderColor(a.themes.theme.Border),
		editor.WithBorder(!promptStyle),
		editor.WithPromptPrefix(promptPrefix),
		editor.WithSubmitMode(submitMode),
		editor.WithMaxHeight(16),
		editor.WithOnSubmit(func(text string) {
			_ = a.cli.ExtensionUIResponseValue(id, text)
			a.closeExtDialog(id)
			a.requestRender(renderer.ReasonFlush)
		}),
		editor.WithOnInterrupt(func() {
			_ = a.cli.ExtensionUIResponseCancelled(id, false)
			a.closeExtDialog(id)
			a.requestRender(renderer.ReasonFlush)
		}),
	)
	if req.Prefill != "" {
		ed.SetText(req.Prefill)
	} else if req.Text != "" {
		ed.SetText(req.Text)
	}
	ed.SetFocused(true)
	title := req.Title
	if title == "" {
		title = "Editor"
	}
	hint := "ctrl+q/ctrl+enter submit · esc cancel"
	if promptStyle {
		hint = "enter submit · shift+enter newline · esc cancel"
	}
	box := interact.NewBox(1, 0, newStaticText(bold(title)), ed, newStaticText(a.themes.theme.Dim(hint)))
	b := interact.DefaultBoxBorder()
	b.Color = interact.Style(a.themes.theme.Border)
	box.SetBorder(&b)
	box.SetFocusTarget(ed)
	a.showExtDialog(req, box, ed)
}

func (a *App) showExtDialog(req protocol.ExtensionUIRequest, root, focus component.Component) {
	a.closeExtDialog(req.ID)
	w := interact.SizeAbs(minInt(a.width-4, 72))
	mh := interact.SizeAbs(minInt(a.height-4, 24))
	opt := interact.OverlayOptions{
		Anchor: interact.AnchorCenter, Width: &w, MaxHeight: &mh, UniformMargin: 1,
	}
	handle := a.overlays.Show(root, opt)
	a.overlays.SetFocus(focus, a.width, a.height)
	a.extDialogs[req.ID] = &extDialog{
		id: req.ID, method: req.Method, req: req, handle: handle, comp: root,
	}
	a.requestRender(renderer.ReasonFlush)
}

func (a *App) closeExtDialog(id string) {
	d, ok := a.extDialogs[id]
	if !ok {
		return
	}
	d.handle.Hide()
	component.DisposeOne(d.comp)
	delete(a.extDialogs, id)
	a.overlays.SetFocus(a.ed, a.width, a.height)
	a.ed.SetFocused(true)
}

func (a *App) cancelTopOverlay(timedOut bool) {
	var last string
	for id := range a.extDialogs {
		last = id
	}
	if last == "" {
		a.overlays.HideTop(a.width, a.height)
		a.overlays.SetFocus(a.ed, a.width, a.height)
		return
	}
	a.cancelExtension(last, timedOut)
}

func (a *App) cancelExtension(id string, timedOut bool) {
	if id == "" {
		a.cancelTopOverlay(timedOut)
		return
	}
	if _, ok := a.extDialogs[id]; ok {
		_ = a.cli.ExtensionUIResponseCancelled(id, timedOut)
		a.closeExtDialog(id)
	}
}

func decodeStringOptions(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err == nil {
		return ss
	}
	var objs []struct {
		Label string `json:"label"`
		Value string `json:"value"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &objs); err == nil {
		out := make([]string, 0, len(objs))
		for _, o := range objs {
			v := o.Value
			if v == "" {
				v = o.Label
			}
			if v == "" {
				v = o.Name
			}
			if v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	return nil
}

func openURL(u string) error {
	u = strings.TrimSpace(u)
	if u == "" {
		return errEmptyURL
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}

type emptyURLError struct{}

func (emptyURLError) Error() string { return "empty url" }

var errEmptyURL error = emptyURLError{}

// silence unused import if client only used via a.cli
var _ = client.TypeReady
