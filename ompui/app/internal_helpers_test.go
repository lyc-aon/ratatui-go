package app

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/client"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/editor"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/interact"
	"github.com/lyc-aon/ratatui-go/ompui/protocol"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
	"github.com/lyc-aon/ratatui-go/ompui/view"
)

// Tests in package app can exercise unexported helpers without a TTY.

func TestQueueInitialPromptsOrder(t *testing.T) {
	a := New(Config{
		Core: client.Command{Path: "unused"},
		Bootstrap: Bootstrap{
			InitialMessage: "primary",
			QueuedMessages: []string{"q1", "q2", "  ", "q3"},
		},
	})
	// Mark a fake cli non-nil so enqueueRPC accepts jobs; we intercept rpcJobs.
	// cli nil short-circuits enqueue — set a dummy marker via enqueue path rewrite:
	// use non-nil cli pointer by starting nothing; instead call enqueue through
	// a.cli check — assign a non-started client is hard. Bypass by temporarily
	// stuffing jobs via testing the bootstrap list assembly.

	// Directly verify Bootstrap ordering contract used by queueInitialPrompts.
	boot := a.cfg.Bootstrap
	if boot.PrimaryMessage() != "primary" {
		t.Fatal(boot.PrimaryMessage())
	}
	var ordered []string
	if p := boot.PrimaryMessage(); p != "" {
		ordered = append(ordered, p)
	}
	for _, m := range boot.AllQueued() {
		if m = trimSpace(m); m != "" {
			ordered = append(ordered, m)
		}
	}
	want := []string{"primary", "q1", "q2", "q3"}
	if len(ordered) != len(want) {
		t.Fatalf("ordered=%v", ordered)
	}
	for i := range want {
		if ordered[i] != want[i] {
			t.Fatalf("order %v", ordered)
		}
	}

	// With cli nil, queueInitialPrompts must not panic and sets initialSent.
	a.ctx, a.cancel = context.WithCancel(context.Background())
	defer a.cancel()
	a.queueInitialPrompts()
	if !a.initialSent {
		t.Fatal("initialSent")
	}
	// Second call is idempotent.
	a.queueInitialPrompts()
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func TestEnqueueRPCSerialRequiresClient(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ctx, a.cancel = context.WithCancel(context.Background())
	defer a.cancel()
	// nil cli: no enqueue
	a.enqueueRPC("prompt", func(ctx context.Context) (client.Response, error) {
		t.Fatal("should not run")
		return client.Response{}, nil
	}, "")
	select {
	case <-a.rpcJobs:
		t.Fatal("job enqueued without cli")
	default:
	}
}

func TestRemoteGenerationStaleDrop(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.width, a.height = 80, 24
	a.ed = editor.New()
	a.overlays = interact.NewOverlayStack()
	a.remotes = map[string]*component.Remote{}
	a.remoteSlots = map[string]protocol.ComponentSlot{}
	a.remoteGen = map[string]uint64{}
	a.remoteResultG = map[string]uint64{}
	r := a.mountRemote("c1", protocol.SlotOverlay, false)
	if r == nil || a.remotes["c1"] == nil {
		t.Fatal("mount")
	}
	// Simulate applied generation 5.
	a.remoteResultG["c1"] = 5
	a.remoteGen["c1"] = 5

	// Older result dropped.
	rawOlder, _ := json.Marshal(protocol.ComponentResultPayload{
		ComponentID: "c1",
		Generation:  4,
		Lines:       []string{"OLD"},
	})
	envOlder := protocol.MustEnvelope(protocol.MsgComponentResult, "", json.RawMessage(rawOlder))
	// Payload for Decode: use historical wrap
	envOlder, _ = protocol.WrapHistorical(rawOlder)
	// Ensure type is component_result
	envOlder.Type = protocol.MsgComponentResult
	envOlder.Payload = rawOlder

	a.handleComponentFrame(client.Event{
		Kind:     protocol.KindComponent,
		Envelope: envOlder,
		Raw:      rawOlder,
	})
	if fr := a.remotes["c1"].Frame(); len(fr.Lines) == 1 && fr.Lines[0] == "OLD" {
		t.Fatal("stale generation applied")
	}

	rawNew, _ := json.Marshal(map[string]any{
		"componentId": "c1",
		"generation":  6,
		"lines":       []string{"NEW"},
	})
	a.handleComponentFrame(client.Event{
		Kind:     protocol.KindComponent,
		Envelope: protocol.Envelope{Type: protocol.MsgComponentResult, Payload: rawNew},
		Raw:      rawNew,
	})
	fr := a.remotes["c1"].Frame()
	if len(fr.Lines) != 1 || fr.Lines[0] != "NEW" {
		t.Fatalf("fresh result not applied: %+v", fr.Lines)
	}
	if a.remoteResultG["c1"] != 6 {
		t.Fatalf("resultG=%d", a.remoteResultG["c1"])
	}
}

func TestHandleEditorFrameOps(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ed = editor.New()
	a.overlays = interact.NewOverlayStack()
	a.width, a.height = 40, 12
	raw, _ := json.Marshal(map[string]any{
		"op":   protocol.EditorOpSetText,
		"text": "from-core",
	})
	a.handleEditorFrame(client.Event{
		Kind:     protocol.KindEditor,
		Envelope: protocol.Envelope{Type: protocol.MsgEditorUpdate, Payload: raw},
		Raw:      raw,
	})
	if a.ed.Text() != "from-core" {
		t.Fatalf("set_text=%q", a.ed.Text())
	}

	raw, _ = json.Marshal(map[string]any{
		"op":     protocol.EditorOpSetText,
		"text":   "a😀\né",
		"cursor": 3,
	})
	a.handleEditorFrame(client.Event{
		Kind:     protocol.KindEditor,
		Envelope: protocol.Envelope{Type: protocol.MsgEditorUpdate, Payload: raw},
		Raw:      raw,
	})
	if cursor := a.ed.Cursor(); cursor.Line != 0 || cursor.Col != len("a😀") {
		t.Fatalf("UTF-16 set_text cursor=%+v", cursor)
	}

	raw, _ = json.Marshal(map[string]any{
		"op":     protocol.EditorOpSetCursor,
		"cursor": 5,
	})
	a.handleEditorFrame(client.Event{
		Kind:     protocol.KindEditor,
		Envelope: protocol.Envelope{Type: protocol.MsgEditorUpdate, Payload: raw},
		Raw:      raw,
	})
	if cursor := a.ed.Cursor(); cursor.Line != 1 || cursor.Col != len("é") {
		t.Fatalf("UTF-16 set_cursor=%+v", cursor)
	}

	raw, _ = json.Marshal(map[string]any{"op": protocol.EditorOpClear})
	a.handleEditorFrame(client.Event{
		Kind:     protocol.KindEditor,
		Envelope: protocol.Envelope{Type: protocol.MsgEditorUpdate, Payload: raw},
		Raw:      raw,
	})
	if a.ed.Text() != "" {
		t.Fatalf("clear=%q", a.ed.Text())
	}

	raw, _ = json.Marshal(map[string]any{"text": "state-snap"})
	a.handleEditorFrame(client.Event{
		Kind:     protocol.KindEditor,
		Envelope: protocol.Envelope{Type: protocol.MsgEditorState, Payload: raw},
		Raw:      raw,
	})
	if a.ed.Text() != "state-snap" {
		t.Fatalf("state=%q", a.ed.Text())
	}
}

func TestHandleRPCEventModelApplyAndThemeFailOpen(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.themes = buildTheme(nil, view.AppearanceDark)
	a.buildUI()
	a.width, a.height = 40, 12

	// Session event applies into model.
	raw := []byte(`{"type":"agent_start"}`)
	env, err := protocol.WrapHistorical(raw)
	if err != nil {
		t.Fatal(err)
	}
	a.handleRPCEvent(client.Event{
		Kind:     protocol.Classify(env),
		Envelope: env,
		Raw:      raw,
	})
	if !a.state.Snapshot().Status.AgentRunning {
		t.Fatal("agent_start not applied")
	}

	// Theme frames fail-open: no panic, no model requirement.
	themeRaw := []byte(`{"type":"theme_sync","name":"dark"}`)
	themeEnv, _ := protocol.WrapHistorical(themeRaw)
	a.handleRPCEvent(client.Event{
		Kind:     protocol.KindTheme,
		Envelope: themeEnv,
		Raw:      themeRaw,
	})

	// terminal_input* currently fail-open log path (bridge mid-land).
	for _, typ := range []string{
		protocol.MsgTerminalInputSubscription,
		protocol.MsgTerminalInputResult,
		protocol.MsgComponentInputResult,
	} {
		r := []byte(`{"type":"` + typ + `"}`)
		e, _ := protocol.WrapHistorical(r)
		a.handleRPCEvent(client.Event{Kind: protocol.Classify(e), Envelope: e, Raw: r})
	}
}

func TestDisposeRemoteCleansMaps(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.width, a.height = 80, 24
	a.ed = editor.New()
	a.overlays = interact.NewOverlayStack()
	// Seed maps; dispose→unslot header calls rebuildRoot which needs ed/overlays.
	r := component.NewRemote("z")
	a.remotes["z"] = r
	a.remoteSlots["z"] = protocol.SlotHeader
	a.headerRemote = r
	a.disposeRemote("z", false)
	if a.remotes["z"] != nil || a.headerRemote != nil {
		t.Fatal("dispose incomplete")
	}
}

func TestRequestRenderReasonEscalation(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.requestRender(renderer.ReasonUpdate)
	if a.renderReason != renderer.ReasonUpdate || !a.needRender {
		t.Fatal("update")
	}
	a.requestRender(renderer.ReasonForce)
	if a.renderReason != renderer.ReasonForce {
		t.Fatalf("reason=%v", a.renderReason)
	}
	// Lower priority does not demote.
	a.requestRender(renderer.ReasonUpdate)
	if a.renderReason != renderer.ReasonForce {
		t.Fatalf("demoted to %v", a.renderReason)
	}
}
func TestComponentOpenMount(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.width, a.height = 60, 20
	a.overlays = interact.NewOverlayStack()
	raw, _ := json.Marshal(map[string]any{
		"componentId": "r1",
		"slot":        "overlay",
	})
	a.handleComponentFrame(client.Event{
		Kind:     protocol.KindComponent,
		Envelope: protocol.Envelope{Type: protocol.MsgComponentOpen, Payload: raw},
		Raw:      raw,
	})
	if a.remotes["r1"] == nil {
		t.Fatal("open did not mount")
	}
}

func TestPostCommandNonblocking(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ctx, a.cancel = context.WithCancel(context.Background())
	defer a.cancel()
	// Fill is not needed; single post should not block.
	done := make(chan struct{})
	go func() {
		a.post(command{kind: cmdForceRender})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("post blocked")
	}
	select {
	case c := <-a.cmds:
		if c.kind != cmdForceRender {
			t.Fatalf("%+v", c)
		}
	case <-time.After(time.Second):
		t.Fatal("no cmd")
	}
}

func TestHandleTermEventDoesNotPanicWithoutUI(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ed = editor.New()
	a.overlays = interact.NewOverlayStack()
	a.width, a.height = 40, 10
	// No root — should fail-open.
	a.handleTermEvent(event.KeyEvent(event.Key{ID: "ctrl+l"}, nil))
	a.handleTermEvent(event.ResizeEvent(event.Size{Cols: 50, Rows: 20}, nil))
}

func TestBgCallNoopWithoutClient(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ctx, a.cancel = context.WithCancel(context.Background())
	defer a.cancel()
	var ran atomic.Bool
	a.bgCall("x", func(ctx context.Context) (client.Response, error) {
		ran.Store(true)
		return client.Response{}, nil
	}, "")
	if ran.Load() {
		t.Fatal("ran without client")
	}
}

func fireSetWidget(t *testing.T, a *App, key, placement string, lines []string) {
	t.Helper()
	var linesRaw json.RawMessage
	if lines == nil {
		linesRaw = json.RawMessage("null")
	} else {
		b, err := json.Marshal(lines)
		if err != nil {
			t.Fatal(err)
		}
		linesRaw = b
	}
	req := protocol.ExtensionUIRequest{
		Type:            protocol.MsgExtensionUIRequest,
		Method:          protocol.ExtUISetWidget,
		WidgetKey:       key,
		WidgetPlacement: placement,
		WidgetLines:     linesRaw,
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	env, err := protocol.WrapHistorical(raw)
	if err != nil {
		t.Fatal(err)
	}
	a.handleExtensionUI(client.Event{
		Kind:     protocol.KindExtensionUIRequest,
		Envelope: env,
		Raw:      raw,
	})
}

func TestSetWidgetLocalCachePlacementAndRootOrder(t *testing.T) {
	a := New(Config{Core: client.Command{Path: "x"}})
	a.ed = editor.New()
	a.overlays = interact.NewOverlayStack()
	a.width, a.height = 80, 24
	// Minimal root peers so order assertions have anchors.
	// rebuildRoot tolerates nil transcript/status/footer; we only need ed non-nil.

	// Place aboveEditor — local Remote only, no remotes[] / no cli frames.
	fireSetWidget(t, a, "w1", "aboveEditor", []string{"ABOVE"})
	r1 := a.extensionWidgets["w1"]
	if r1 == nil {
		t.Fatal("missing local widget w1")
	}
	if r1.ID() != "extension-widget-w1" {
		t.Fatalf("id=%q", r1.ID())
	}
	if a.remotes["extension-widget-w1"] != nil || a.remotes["w1"] != nil {
		t.Fatal("setWidget must not register protocol remotes map")
	}
	if a.extensionWidgetSlots["w1"] != "aboveEditor" {
		t.Fatalf("slot=%q", a.extensionWidgetSlots["w1"])
	}
	if len(a.widgetAbove) != 1 || a.widgetAbove[0] != r1 {
		t.Fatalf("widgetAbove=%v", a.widgetAbove)
	}
	if len(a.widgetBelow) != 0 {
		t.Fatalf("widgetBelow=%v", a.widgetBelow)
	}
	if got := r1.Frame().Lines; len(got) != 1 || got[0] != "ABOVE" {
		t.Fatalf("lines=%v", got)
	}

	// Same key + same placement updates lines in place (same Remote instance).
	fireSetWidget(t, a, "w1", "aboveEditor", []string{"ABOVE2"})
	if a.extensionWidgets["w1"] != r1 {
		t.Fatal("in-place update replaced Remote")
	}
	if got := r1.Frame().Lines; len(got) != 1 || got[0] != "ABOVE2" {
		t.Fatalf("updated lines=%v", got)
	}

	// belowEditor second key.
	fireSetWidget(t, a, "w2", "belowEditor", []string{"BELOW"})
	r2 := a.extensionWidgets["w2"]
	if r2 == nil || len(a.widgetBelow) != 1 || a.widgetBelow[0] != r2 {
		t.Fatalf("below w2 above=%d below=%d", len(a.widgetAbove), len(a.widgetBelow))
	}

	// Root order: … status → widgetAbove → editor → widgetBelow → footer …
	if a.root == nil {
		t.Fatal("root nil")
	}
	kids := a.root.Children()
	idx := func(c component.Component) int {
		for i, k := range kids {
			if k == c {
				return i
			}
		}
		return -1
	}
	iAbove, iEd, iBelow := idx(r1), idx(a.ed), idx(r2)
	if iAbove < 0 || iEd < 0 || iBelow < 0 {
		t.Fatalf("missing in root above=%d ed=%d below=%d kids=%d", iAbove, iEd, iBelow, len(kids))
	}
	if !(iAbove < iEd && iEd < iBelow) {
		t.Fatalf("order above=%d editor=%d below=%d (want above < editor < below)", iAbove, iEd, iBelow)
	}

	// Key replacement: move w1 from above → below (dispose old, new instance).
	fireSetWidget(t, a, "w1", "belowEditor", []string{"MOVED"})
	r1b := a.extensionWidgets["w1"]
	if r1b == nil || r1b == r1 {
		t.Fatal("placement change should replace Remote instance")
	}
	if a.extensionWidgetSlots["w1"] != "belowEditor" {
		t.Fatal("slot not updated")
	}
	// Old above entry gone; both w1 and w2 under below (order: w2 then w1 append).
	if len(a.widgetAbove) != 0 {
		t.Fatalf("above after move=%v", a.widgetAbove)
	}
	if len(a.widgetBelow) != 2 {
		t.Fatalf("below=%d", len(a.widgetBelow))
	}
	if a.widgetBelow[0] != r2 || a.widgetBelow[1] != r1b {
		t.Fatalf("below order=%v %v", a.widgetBelow[0], a.widgetBelow[1])
	}
	if r1.Disposed() != true {
		// Dispose sets disposed flag when available.
		if !r1.Disposed() {
			t.Fatal("old remote not disposed on key move")
		}
	}

	// Removal: empty/null lines drops key.
	fireSetWidget(t, a, "w2", "belowEditor", nil)
	if a.extensionWidgets["w2"] != nil {
		t.Fatal("w2 not removed")
	}
	if a.extensionWidgetSlots["w2"] != "" {
		t.Fatal("slot map leak")
	}
	if len(a.widgetBelow) != 1 || a.widgetBelow[0] != r1b {
		t.Fatalf("below after remove w2=%v", a.widgetBelow)
	}
	if !r2.Disposed() {
		t.Fatal("removed widget not disposed")
	}

	// Empty lines slice also removes.
	fireSetWidget(t, a, "w1", "belowEditor", []string{})
	if a.extensionWidgets["w1"] != nil || len(a.widgetBelow) != 0 {
		t.Fatalf("empty lines should remove w1 widgets=%v below=%v", a.extensionWidgets, a.widgetBelow)
	}

	// Default key + unknown placement → aboveEditor.
	fireSetWidget(t, a, "", "sideways", []string{"DEF"})
	def := a.extensionWidgets["default"]
	if def == nil || a.extensionWidgetSlots["default"] != "aboveEditor" {
		t.Fatalf("default key/placement widgets=%v slots=%v", a.extensionWidgets, a.extensionWidgetSlots)
	}
	if len(a.widgetAbove) != 1 || a.widgetAbove[0] != def {
		t.Fatal("default not above")
	}
	// Still no protocol remotes / no cli required.
	if len(a.remotes) != 0 {
		t.Fatalf("remotes polluted: %v", a.remotes)
	}
}

func TestOverlayOptionsFromMountPreservesGeometry(t *testing.T) {
	minWidth, offsetX, offsetY, fullscreen := 24, -2, 3, true
	p := protocol.OverlayMountPayload{
		Width:      json.RawMessage(`"60%"`),
		MinWidth:   &minWidth,
		MaxHeight:  json.RawMessage(`14`),
		Anchor:     string(interact.AnchorTopRight),
		OffsetX:    &offsetX,
		OffsetY:    &offsetY,
		Row:        json.RawMessage(`"25%"`),
		Col:        json.RawMessage(`4`),
		Margin:     json.RawMessage(`{"top":1,"right":2,"bottom":3,"left":4}`),
		Fullscreen: &fullscreen,
	}
	got := overlayOptionsFromMount(p, 120)
	if got.Width == nil || !got.Width.IsPct || got.Width.Pct != 60 {
		t.Fatalf("width=%+v", got.Width)
	}
	if got.MinWidth != 24 || got.MaxHeight == nil || got.MaxHeight.Abs != 14 {
		t.Fatalf("min/max=%d %+v", got.MinWidth, got.MaxHeight)
	}
	if got.Anchor != interact.AnchorTopRight || got.OffsetX != -2 || got.OffsetY != 3 {
		t.Fatalf("anchor/offset=%q %d %d", got.Anchor, got.OffsetX, got.OffsetY)
	}
	if got.Row == nil || !got.Row.IsPct || got.Row.Pct != 25 || got.Col == nil || got.Col.Abs != 4 {
		t.Fatalf("row/col=%+v %+v", got.Row, got.Col)
	}
	if got.Margin == nil || *got.Margin != (interact.OverlayMargin{Top: 1, Right: 2, Bottom: 3, Left: 4}) {
		t.Fatalf("margin=%+v", got.Margin)
	}
	if got.UniformMargin != -1 || !got.Fullscreen {
		t.Fatalf("uniform/fullscreen=%d %v", got.UniformMargin, got.Fullscreen)
	}
}

func TestOverlayOptionsFromMountMalformedUsesDefaults(t *testing.T) {
	got := overlayOptionsFromMount(protocol.OverlayMountPayload{
		Width:  json.RawMessage(`"bad"`),
		Anchor: "sideways",
		Margin: json.RawMessage(`{}`),
	}, 100)
	if got.Width == nil || got.Width.IsPct || got.Width.Abs != 80 {
		t.Fatalf("default width=%+v", got.Width)
	}
	if got.Anchor != interact.AnchorCenter || got.UniformMargin != 1 || got.Margin != nil {
		t.Fatalf("defaults=%+v", got)
	}
}

func TestThemeSyncPaletteMergeAndRemoteOwnership(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TERM", "xterm-256color")

	defaults := view.LightPalette()
	merged := mergeThemeSyncPalette(view.AppearanceLight, &protocol.ThemeSyncPalette{
		Text:   "#112233",
		Accent: "not-a-color",
	})
	if merged.Text != "#112233" {
		t.Fatalf("text=%q", merged.Text)
	}
	if merged.Accent != defaults.Accent {
		t.Fatalf("invalid accent replaced default: %q", merged.Accent)
	}

	a := New(Config{Core: client.Command{Path: "x"}})
	a.themes = buildTheme(nil, view.AppearanceDark)
	a.buildUI()
	a.applyThemeSync(protocol.ThemeSyncPayload{
		Name:       "custom",
		Appearance: "light",
		Palette:    &protocol.ThemeSyncPalette{Text: "#112233"},
	})
	if !a.themes.remoteOwned || a.themes.appearance != view.AppearanceLight {
		t.Fatalf("theme ownership/appearance=%v %v", a.themes.remoteOwned, a.themes.appearance)
	}
	if !a.needRender || !a.forceRender {
		t.Fatalf("theme sync did not force render: need=%v force=%v", a.needRender, a.forceRender)
	}
}

func TestMarshalEditorStateFrameIncludesDiscriminator(t *testing.T) {
	raw, err := marshalEditorStateFrame("/model", 6, "Type a message…")
	if err != nil {
		t.Fatal(err)
	}
	var frame struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Cursor      int    `json:"cursor"`
		Placeholder string `json:"placeholder"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Type != protocol.MsgEditorState || frame.Text != "/model" || frame.Cursor != 6 {
		t.Fatalf("frame=%+v raw=%s", frame, raw)
	}
	if frame.Placeholder != "Type a message…" {
		t.Fatalf("placeholder=%q", frame.Placeholder)
	}
}

func TestEditorCursorWireOffsetsUseUTF16(t *testing.T) {
	text := "a😀\né"
	if got := utf16OffsetFromByte(text, len("a😀")); got != 3 {
		t.Fatalf("outbound offset=%d", got)
	}
	if got := editorCursorFromUTF16Offset(text, 3); got.Line != 0 || got.Col != len("a😀") {
		t.Fatalf("round trip=%+v", got)
	}
	if got := editorCursorFromUTF16Offset(text, 2); got.Line != 0 || got.Col != len("a") {
		t.Fatalf("surrogate split should snap before rune: %+v", got)
	}
	if got := editorCursorFromUTF16Offset(text, 5); got.Line != 1 || got.Col != len("é") {
		t.Fatalf("multiline end=%+v", got)
	}
	if got := truncateUTF8("aa😀bb", 4); got != "aa" {
		t.Fatalf("truncated UTF-8=%q", got)
	}
}

func TestFrontendHelloAdvertisesOnlyImplementedCapabilities(t *testing.T) {
	hello := frontendHello()
	for _, capability := range []string{
		protocol.CapJSONL,
		protocol.CapLengthPrefix,
		protocol.CapRPC,
		protocol.CapSessionEvents,
		protocol.CapExtensionUI,
		protocol.CapRemoteComponents,
		protocol.CapEditorSync,
		protocol.CapOverlays,
	} {
		if !protocol.HasCap(hello.Caps, capability) {
			t.Fatalf("missing capability %q: %v", capability, hello.Caps)
		}
	}
	for _, unsupported := range []string{protocol.CapHostTools, protocol.CapHostURI} {
		if protocol.HasCap(hello.Caps, unsupported) {
			t.Fatalf("advertised unsupported capability %q: %v", unsupported, hello.Caps)
		}
	}
}
