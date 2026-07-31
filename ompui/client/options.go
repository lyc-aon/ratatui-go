package client

import (
	"io"
	"os"
	"time"

	"github.com/michaelkelly/ratatui-go/ompui/protocol"
)

// Default queue / timeout knobs.
const (
	// DefaultEventBuffer is the per-subscriber channel capacity.
	DefaultEventBuffer = 256
	// DefaultDispatchBuffer is the internal reader→dispatcher queue capacity.
	// Bounded; never grows. Full queue applies backpressure on the reader
	// (critical frames) rather than dropping session/tool/UI frames.
	DefaultDispatchBuffer = 1024
	// DefaultReadyTimeout is how long Start waits for ready/hello.
	DefaultReadyTimeout = 30 * time.Second
	// DefaultShutdownTimeout is how long Shutdown waits for a clean child exit
	// before Kill.
	DefaultShutdownTimeout = 5 * time.Second
)

// Options configures [Start].
type Options struct {
	// Command is the Bun core argv. Required unless ProcessFactory returns a
	// live Process without using it.
	Command Command

	// Framing selects the wire codec. Default [protocol.FramingJSONL] for
	// current Bun rpc-ui. Length-prefix is reserved for v1 peers that advertise
	// CapLengthPrefix after hello.
	Framing protocol.Framing

	// Stderr is attached to the child stderr. nil → os.Stderr.
	// The client never reads stderr; inject a multi-writer for capture.
	Stderr io.Writer

	// ProcessFactory overrides process creation (tests). nil → DefaultProcessFactory.
	ProcessFactory ProcessFactory

	// IDs generates request correlation ids. nil → protocol.NewIDGenerator("fe").
	IDs *protocol.IDGenerator

	// ReadyTimeout bounds the wait for {"type":"ready"} or v1 hello.
	// Zero selects [DefaultReadyTimeout]. Negative means wait forever (still
	// cancelled by the Start context).
	ReadyTimeout time.Duration

	// ShutdownTimeout bounds graceful Shutdown before Kill.
	// Zero selects [DefaultShutdownTimeout]. Negative means wait forever.
	ShutdownTimeout time.Duration

	// EventBuffer is the per-subscriber channel capacity.
	// Zero selects [DefaultEventBuffer].
	EventBuffer int

	// SubscribeBeforeReady creates one EventBuffer-sized subscription before
	// reader goroutines start. Use [Client.InitialSubscription] to consume it.
	// This prevents startup frames emitted immediately after "ready" from racing
	// a caller's first Subscribe call.
	SubscribeBeforeReady bool

	// DispatchBuffer is the internal ordered dispatch queue capacity.
	// Zero selects [DefaultDispatchBuffer].
	DispatchBuffer int

	// Hello is the local hello payload sent when speaking v1.
	// Zero value → protocol.NewHello(protocol.RoleFrontend) with default caps.
	// In historical Bun compatibility mode the client does NOT send hello first;
	// it waits for ready. If the peer later sends hello, local hello is replied.
	Hello protocol.HelloPayload

	// SendHelloOnStart, when true, sends local hello immediately after spawn
	// (v1 peers). Default false: historical Bun rpc-ui has no hello and would
	// treat a hello frame as an unknown command.
	SendHelloOnStart bool

	// OnRawWrite, if set, is invoked with every outbound JSON body (without the
	// trailing newline / length header) immediately before the write. Intended
	// for tests and diagnostics; must not block.
	OnRawWrite func([]byte)

	// OnRawRead, if set, is invoked with every inbound JSON body (line or frame
	// payload) immediately after decode, before routing. Must not block.
	OnRawRead func([]byte)

	// Environ is used only by helpers that need a parent env snapshot; the
	// default process factory uses os.Environ when Command.Env is nil.
	// Exposed so tests can avoid reading the real environment.
	Environ func() []string
}

func (o *Options) withDefaults() Options {
	out := *o
	if out.Stderr == nil {
		out.Stderr = os.Stderr
	}
	if out.ProcessFactory == nil {
		out.ProcessFactory = DefaultProcessFactory
	}
	if out.IDs == nil {
		out.IDs = protocol.NewIDGenerator("fe")
	}
	if out.ReadyTimeout == 0 {
		out.ReadyTimeout = DefaultReadyTimeout
	}
	if out.ShutdownTimeout == 0 {
		out.ShutdownTimeout = DefaultShutdownTimeout
	}
	if out.EventBuffer <= 0 {
		out.EventBuffer = DefaultEventBuffer
	}
	if out.DispatchBuffer <= 0 {
		out.DispatchBuffer = DefaultDispatchBuffer
	}
	if out.Hello.Protocol == "" && out.Hello.Role == "" && out.Hello.Major == 0 {
		out.Hello = protocol.NewHello(protocol.RoleFrontend)
	}
	if out.Environ == nil {
		out.Environ = os.Environ
	}
	// Framing zero value is FramingLengthPrefix in protocol; force JSONL as the
	// Bun-compatible default when the caller left it unset AND did not opt into
	// length-prefix explicitly. We treat the zero value as "use JSONL default"
	// because that matches current Bun rpc-ui. Callers that want length-prefix
	// must set Framing: protocol.FramingLengthPrefix AND speak v1 hello.
	//
	// Detection: Options is a value type; we cannot tell "unset" from
	// FramingLengthPrefix==0. Document that JSONL is the Start default via
	// explicit assignment below when neither flag is set. Callers wanting
	// length-prefix set Framing and SendHelloOnStart.
	if !o.framingExplicit() {
		out.Framing = protocol.FramingJSONL
	}
	return out
}

// framingExplicit reports whether the caller set Framing non-zero OR opted into
// length-prefix via SendHelloOnStart with FramingLengthPrefix. Because the zero
// value of Framing is LengthPrefix, the public default path always assigns JSONL
// unless the caller sets Framing after constructing Options with a non-default
// intent. We use a side channel: if Framing is LengthPrefix (0) AND
// SendHelloOnStart is true, treat it as explicit length-prefix. Otherwise default JSONL.
func (o *Options) framingExplicit() bool {
	if o.Framing == protocol.FramingJSONL {
		return true
	}
	// LengthPrefix (0) is explicit only when the caller is speaking v1 up front.
	if o.Framing == protocol.FramingLengthPrefix && o.SendHelloOnStart {
		return true
	}
	return false
}
