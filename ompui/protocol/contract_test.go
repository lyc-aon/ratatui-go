package protocol_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

func TestV1EnvelopeRoundTripPreservesPayloadAndExtra(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"message": "hello",
		"nested":  map[string]any{"n": 1, "flag": true},
		"list":    []any{"a", 2},
	}
	env, err := protocol.NewEnvelope(protocol.MsgHello, "id-1", payload)
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	env.Extra = map[string]protocol.RawPayload{
		"x_forward": protocol.RawPayload(`{"keep":true}`),
		"trace":     protocol.RawPayload(`"abc"`),
	}

	raw, err := env.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	got, err := protocol.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if got.V != protocol.Major {
		t.Fatalf("V=%d want %d", got.V, protocol.Major)
	}
	if got.Type != protocol.MsgHello || got.ID != "id-1" {
		t.Fatalf("type/id = %q/%q", got.Type, got.ID)
	}
	if string(got.Extra["x_forward"]) != `{"keep":true}` {
		t.Fatalf("extra x_forward lost: %s", got.Extra["x_forward"])
	}
	if string(got.Extra["trace"]) != `"abc"` {
		t.Fatalf("extra trace lost: %s", got.Extra["trace"])
	}

	// Re-marshal must still carry unknown top-level keys.
	raw2, err := got.Bytes()
	if err != nil {
		t.Fatalf("re-Bytes: %v", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw2, &obj); err != nil {
		t.Fatalf("unmarshal reencoded: %v", err)
	}
	if _, ok := obj["x_forward"]; !ok {
		t.Fatalf("reencoded missing x_forward: %s", raw2)
	}
	if _, ok := obj["payload"]; !ok {
		t.Fatalf("reencoded missing payload: %s", raw2)
	}
}

func TestHistoricalBareFrameRoundTripLossless(t *testing.T) {
	t.Parallel()

	frame := []byte(`{"type":"prompt","id":"p1","message":"hi","images":[{"type":"image","data":"abc"}],"extraField":{"k":1}}`)
	env, err := protocol.WrapHistorical(frame)
	if err != nil {
		t.Fatalf("WrapHistorical: %v", err)
	}
	if env.Type != "prompt" || env.ID != "p1" || env.V != protocol.Major {
		t.Fatalf("env = %+v", env)
	}
	if !bytes.Equal(env.HistoricalPayload(), frame) {
		t.Fatalf("HistoricalPayload mutated\n got %s\nwant %s", env.HistoricalPayload(), frame)
	}

	// JSONL bare decode path (no v) keeps fields in Extra and rebuilds payload.
	dec := protocol.NewJSONLDecoder(bytes.NewReader(append(append([]byte{}, frame...), '\n')))
	got, err := dec.Decode()
	if err != nil {
		t.Fatalf("JSONL Decode: %v", err)
	}
	if got.V != 0 {
		t.Fatalf("bare frame V=%d want 0", got.V)
	}
	if got.Type != "prompt" || got.ID != "p1" {
		t.Fatalf("bare type/id = %q/%q", got.Type, got.ID)
	}
	hist := got.HistoricalPayload()
	var want, have map[string]json.RawMessage
	if err := json.Unmarshal(frame, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(hist, &have); err != nil {
		t.Fatalf("hist json: %v", err)
	}
	for k, v := range want {
		if !bytes.Equal(have[k], v) {
			t.Fatalf("historical field %q lost: got %s want %s", k, have[k], v)
		}
	}
}

func TestUnknownTopLevelAndPayloadPreservation(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"v":1,"type":"future_feature","id":"x","payload":{"a":1,"b":{"c":[1,2]}},"peerTag":"keep-me"}`)
	env, err := protocol.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if protocol.Classify(env) != protocol.KindUnknown {
		t.Fatalf("Classify=%v want unknown", protocol.Classify(env))
	}
	if string(env.Extra["peerTag"]) != `"keep-me"` {
		t.Fatalf("peerTag not preserved: %v", env.Extra)
	}
	out, err := env.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if string(obj["peerTag"]) != `"keep-me"` {
		t.Fatalf("marshal dropped peerTag: %s", out)
	}
	if !bytes.Contains(obj["payload"], []byte(`"b"`)) {
		t.Fatalf("payload nested lost: %s", obj["payload"])
	}
}

func TestClassifyCoversAllMessageKinds(t *testing.T) {
	t.Parallel()

	want := map[string]protocol.Kind{
		protocol.MsgHello:                     protocol.KindHello,
		protocol.MsgShutdown:                  protocol.KindShutdown,
		protocol.MsgError:                     protocol.KindError,
		protocol.MsgRPCCommand:                protocol.KindRPCCommand,
		protocol.MsgRPCResponse:               protocol.KindRPCResponse,
		protocol.MsgSessionEvent:              protocol.KindSessionEvent,
		protocol.MsgAvailableCommandsUpdate:   protocol.KindSessionEvent,
		protocol.MsgPromptResult:              protocol.KindSessionEvent,
		protocol.MsgCommandOutput:             protocol.KindSessionEvent,
		protocol.MsgSubagentLifecycle:         protocol.KindSessionEvent,
		protocol.MsgSubagentProgress:          protocol.KindSessionEvent,
		protocol.MsgSubagentEvent:             protocol.KindSessionEvent,
		protocol.MsgExtensionUIRequest:        protocol.KindExtensionUIRequest,
		protocol.MsgExtensionUIResponse:       protocol.KindExtensionUIResponse,
		protocol.MsgHostToolCall:              protocol.KindHostTool,
		protocol.MsgHostToolCancel:            protocol.KindHostTool,
		protocol.MsgHostToolUpdate:            protocol.KindHostTool,
		protocol.MsgHostToolResult:            protocol.KindHostTool,
		protocol.MsgHostURIRequest:            protocol.KindHostURI,
		protocol.MsgHostURICancel:             protocol.KindHostURI,
		protocol.MsgHostURIResult:             protocol.KindHostURI,
		protocol.MsgEditorState:               protocol.KindEditor,
		protocol.MsgEditorUpdate:              protocol.KindEditor,
		protocol.MsgEditorQuery:               protocol.KindEditor,
		protocol.MsgStatusSync:                protocol.KindStatus,
		protocol.MsgWorkingMessage:            protocol.KindStatus,
		protocol.MsgToolsExpanded:             protocol.KindStatus,
		protocol.MsgThemeSync:                 protocol.KindTheme,
		protocol.MsgThemeQuery:                protocol.KindTheme,
		protocol.MsgComponentOpen:             protocol.KindComponent,
		protocol.MsgComponentRender:           protocol.KindComponent,
		protocol.MsgComponentResult:           protocol.KindComponent,
		protocol.MsgComponentInput:            protocol.KindComponent,
		protocol.MsgComponentInputResult:      protocol.KindComponent,
		protocol.MsgComponentInvalidate:       protocol.KindComponent,
		protocol.MsgComponentDispose:          protocol.KindComponent,
		protocol.MsgComponentFocus:            protocol.KindComponent,
		protocol.MsgComponentFocusRequest:     protocol.KindComponent,
		protocol.MsgTerminalInputSubscription: protocol.KindTerminalInput,
		protocol.MsgTerminalInput:             protocol.KindTerminalInput,
		protocol.MsgTerminalInputResult:       protocol.KindTerminalInput,
		protocol.MsgOverlayMount:              protocol.KindOverlay,
		protocol.MsgOverlayUnmount:            protocol.KindOverlay,
		protocol.MsgOverlayUpdate:             protocol.KindOverlay,
	}

	kinds := protocol.MessageKinds()
	if len(kinds) == 0 {
		t.Fatal("MessageKinds empty")
	}
	seen := make(map[string]bool, len(kinds))
	for _, typ := range kinds {
		seen[typ] = true
		k, ok := want[typ]
		if !ok {
			t.Fatalf("MessageKinds has %q with no expected Kind", typ)
		}
		if got := protocol.ClassifyType(typ); got != k {
			t.Fatalf("ClassifyType(%q)=%v want %v", typ, got, k)
		}
		if got := protocol.Classify(protocol.Envelope{Type: typ}); got != k {
			t.Fatalf("Classify(%q)=%v want %v", typ, got, k)
		}
		if k.String() == "unknown" {
			t.Fatalf("Kind.String for %q is unknown", typ)
		}
	}
	for typ := range want {
		if !seen[typ] {
			t.Fatalf("MessageKinds missing %q", typ)
		}
	}

	// Bare historical RpcCommand types classify as RPC commands.
	for _, cmd := range protocol.AllRPCCommands() {
		if !protocol.IsKnownRPCCommand(cmd) {
			t.Fatalf("IsKnownRPCCommand(%q)=false", cmd)
		}
		if got := protocol.ClassifyType(cmd); got != protocol.KindRPCCommand {
			t.Fatalf("ClassifyType(%q)=%v want rpc_command", cmd, got)
		}
	}
	if protocol.ClassifyType("totally_new_thing") != protocol.KindUnknown {
		t.Fatal("unknown type must stay KindUnknown")
	}
	if protocol.Kind(999).String() != "unknown" {
		t.Fatal("invalid Kind.String")
	}
}

func TestLengthPrefixedAndJSONLCodecRoundTrip(t *testing.T) {
	t.Parallel()

	env := protocol.MustEnvelope(protocol.MsgStatusSync, "s1", protocol.StatusSyncPayload{
		Entries: map[string]string{"model": "x"},
	})

	var buf bytes.Buffer
	enc := protocol.NewEncoder(&buf)
	if err := enc.Encode(env); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	framed := buf.Bytes()
	back, err := protocol.UnmarshalFrame(framed)
	if err != nil {
		t.Fatalf("UnmarshalFrame: %v", err)
	}
	if back.Type != env.Type || back.ID != env.ID {
		t.Fatalf("framed round-trip mismatch: %+v", back)
	}

	dec := protocol.NewDecoder(bytes.NewReader(framed))
	got, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Type != env.Type {
		t.Fatalf("decoder type=%q", got.Type)
	}
	if _, err := dec.Decode(); !errors.Is(err, io.EOF) {
		t.Fatalf("second decode err=%v want EOF", err)
	}

	line, err := protocol.MarshalEnvelopeJSONL(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(line, []byte{'\n'}) {
		t.Fatalf("JSONL missing newline: %q", line)
	}
	jdec := protocol.NewJSONLDecoder(bytes.NewReader(line))
	got2, err := jdec.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if got2.Type != env.Type || got2.ID != env.ID {
		t.Fatalf("jsonl round-trip: %+v", got2)
	}

	// Blank lines skipped.
	jdec = protocol.NewJSONLDecoder(strings.NewReader("\n\n" + string(line)))
	if _, err := jdec.Decode(); err != nil {
		t.Fatalf("blank-skip decode: %v", err)
	}
}

func TestCodecRejectsOversizeAndZeroLength(t *testing.T) {
	t.Parallel()

	// Zero-length frame header.
	var z bytes.Buffer
	z.Write([]byte{0, 0, 0, 0})
	if _, err := protocol.NewDecoder(&z).Decode(); !errors.Is(err, protocol.ErrZeroLength) {
		t.Fatalf("zero length err=%v", err)
	}

	// Oversize declared length.
	var big bytes.Buffer
	// length = MaxFrameSize+1
	n := protocol.MaxFrameSize + 1
	big.Write([]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	if _, err := protocol.NewDecoder(&big).Decode(); !errors.Is(err, protocol.ErrFrameTooLarge) {
		t.Fatalf("oversize err=%v", err)
	}
}

func TestHelloAcceptAndHandshakeJSONL(t *testing.T) {
	t.Parallel()

	local := protocol.NewHello(protocol.RoleFrontend, protocol.CapJSONL, protocol.CapRPC)
	if local.Major != protocol.Major || local.Protocol != protocol.ProtocolName {
		t.Fatalf("NewHello defaults: %+v", local)
	}
	if !protocol.HasCap(local.Caps, protocol.CapJSONL) {
		t.Fatal("missing CapJSONL")
	}
	if err := protocol.AcceptHello(local); err != nil {
		t.Fatalf("AcceptHello same major: %v", err)
	}
	bad := local
	bad.Major = protocol.Major + 1
	if err := protocol.AcceptHello(bad); !errors.Is(err, protocol.ErrMajorMismatch) {
		t.Fatalf("AcceptHello mismatch err=%v", err)
	}

	// Peer hello with unknown extra field round-trips.
	peer := protocol.NewHello(protocol.RoleCore, protocol.CapJSONL)
	peer.Extra = map[string]protocol.RawPayload{"build": protocol.RawPayload(`"nightly"`)}
	peerEnv := protocol.MustEnvelope(protocol.MsgHello, "", peer)
	var peerBuf bytes.Buffer
	if err := protocol.NewEncoder(&peerBuf).EncodeJSONL(peerEnv); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	enc := protocol.NewEncoder(&out)
	rdr := protocol.NewJSONLDecoder(bytes.NewReader(peerBuf.Bytes()))
	got, err := protocol.Handshake(enc, protocol.FramingJSONL, rdr, local)
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if got.Role != protocol.RoleCore {
		t.Fatalf("peer role=%q", got.Role)
	}
	if string(got.Extra["build"]) != `"nightly"` {
		t.Fatalf("hello extra lost: %v", got.Extra)
	}
	// Local hello was written first.
	sent, err := protocol.NewJSONLDecoder(bytes.NewReader(out.Bytes())).Decode()
	if err != nil {
		t.Fatal(err)
	}
	if sent.Type != protocol.MsgHello {
		t.Fatalf("first write type=%q", sent.Type)
	}
}

func TestWrapRPCCommandAndCheckMajor(t *testing.T) {
	t.Parallel()

	cmd := protocol.BuildRPCCommand(protocol.CmdPrompt, "c1", map[string]any{"message": "x"})
	env, err := protocol.EnvelopeFromRPCCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if env.Type != protocol.MsgRPCCommand || env.ID != "c1" {
		t.Fatalf("env=%+v", env)
	}
	var inner map[string]any
	if err := json.Unmarshal(env.Payload, &inner); err != nil {
		t.Fatal(err)
	}
	if inner["type"] != protocol.CmdPrompt || inner["message"] != "x" {
		t.Fatalf("payload=%v", inner)
	}

	if err := (protocol.Envelope{V: 0, Type: "x"}).CheckMajor(); err != nil {
		t.Fatalf("V=0 allowed: %v", err)
	}
	if err := (protocol.Envelope{V: protocol.Major, Type: "x"}).CheckMajor(); err != nil {
		t.Fatalf("V=major ok: %v", err)
	}
	if err := (protocol.Envelope{V: 99, Type: "x"}).CheckMajor(); !errors.Is(err, protocol.ErrMajorMismatch) {
		t.Fatalf("mismatch err=%v", err)
	}
	if err := (protocol.Envelope{}).ValidateBasic(); !errors.Is(err, protocol.ErrInvalidEnvelope) {
		t.Fatalf("ValidateBasic empty type: %v", err)
	}
}

func TestIDGeneratorUniqueAndPrefixed(t *testing.T) {
	t.Parallel()

	g := protocol.NewIDGenerator("fe")
	a, b := g.Next(), g.Next()
	if a == b {
		t.Fatalf("ids collided: %q", a)
	}
	if !strings.HasPrefix(a, "fe-") || !strings.HasPrefix(b, "fe-") {
		t.Fatalf("prefix missing: %q %q", a, b)
	}
	if protocol.NewID() == "" || protocol.NextID() == "" {
		t.Fatal("empty id")
	}
}

func TestSessionEventTypesClassify(t *testing.T) {
	t.Parallel()

	events := []string{
		protocol.EventAgentStart, protocol.EventAgentEnd,
		protocol.EventTurnStart, protocol.EventTurnEnd,
		protocol.EventMessageStart, protocol.EventMessageUpdate, protocol.EventMessageEnd,
		protocol.EventToolExecutionStart, protocol.EventToolExecutionUpdate, protocol.EventToolExecutionEnd,
		protocol.EventAutoCompactionStart, protocol.EventAutoCompactionEnd,
		protocol.EventAutoRetryStart, protocol.EventAutoRetryEnd,
		protocol.EventNotice, protocol.EventThinkingLevelChanged, protocol.EventGoalUpdated,
	}
	for _, ev := range events {
		if !protocol.IsSessionEventType(ev) {
			t.Fatalf("IsSessionEventType(%q)=false", ev)
		}
	}
}

func TestComponentRenderTerminalGeometryRoundTrip(t *testing.T) {
	t.Parallel()
	in := protocol.ComponentRenderPayload{
		ComponentID:    "component-1",
		Width:          42,
		Height:         7,
		TerminalWidth:  132,
		TerminalHeight: 48,
		Generation:     9,
		Extra: map[string]protocol.RawPayload{
			"future": protocol.RawPayload(`{"kept":true}`),
		},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out protocol.ComponentRenderPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.ComponentID != in.ComponentID || out.Width != 42 || out.Height != 7 ||
		out.TerminalWidth != 132 || out.TerminalHeight != 48 || out.Generation != 9 {
		t.Fatalf("round trip=%+v", out)
	}
	if string(out.Extra["future"]) != `{"kept":true}` {
		t.Fatalf("extra=%s", out.Extra["future"])
	}
}
