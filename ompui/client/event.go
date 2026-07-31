package client

import (
	"sync"
	"sync/atomic"

	"github.com/lyc-aon/ratatui-go/ompui/protocol"
)

// Wire type string emitted by current Bun rpc-ui on startup.
const TypeReady = "ready"

// Event is one ordered frame delivered to subscribers.
//
// Envelope always holds the normalized view (bare Bun frames become V=0
// envelopes with fields in Extra; HistoricalPayload reconstructs the wire
// object). Raw is the exact inbound JSON bytes when available — prefer Raw
// when forwarding without re-encode.
type Event struct {
	// Seq is a monotonic per-client sequence number (starts at 1).
	Seq uint64

	// Kind is the coarse routing class from [protocol.Classify].
	Kind protocol.Kind

	// Envelope is the normalized frame. Always populated for wire frames.
	Envelope protocol.Envelope

	// Raw is the exact JSON body read from the peer (no trailing newline).
	// Nil only for synthetic client-local events (e.g. backpressure notice).
	Raw protocol.RawPayload

	// Response is set when Kind is KindRPCResponse.
	Response *protocol.RPCResponse

	// Err is set for synthetic error events or when a fatal protocol fault
	// is surfaced on the event stream.
	Err error
}

// IsCritical reports whether this event must never be silently dropped.
// Session, tool, extension-UI, host-tool/uri, RPC responses, control frames,
// and unknown frames are all critical so forward-compat traffic is safe.
func (e Event) IsCritical() bool {
	// Every classified kind is critical. The method exists so a future
	// diagnostic-only kind can return false without rewriting call sites.
	return true
}

// Subscription is a single ordered event consumer.
// Events are delivered in the order the reader observed them.
// C is closed when the subscription is unsubscribed or the client shuts down.
type Subscription struct {
	// C receives events. Never close this from outside; use Unsubscribe.
	C <-chan Event

	id     uint64
	client *Client
	ch     chan Event

	// quit is closed to unblock a deliverOne parked on a full channel.
	quit     chan struct{}
	quitOnce sync.Once

	// inflight counts trySend calls currently inside the send path so closeCh
	// can wait for them before close(ch), avoiding send-on-closed-channel.
	inflight sync.WaitGroup

	// chClosed guards close(ch); dead is a fast-path skip for deliverOne.
	chClosed atomic.Bool
	dead     atomic.Bool
}

// Unsubscribe removes this subscription and closes C.
// Safe to call multiple times and from any goroutine.
func (s *Subscription) Unsubscribe() {
	if s == nil || s.client == nil {
		return
	}
	s.client.unsubscribe(s)
}

func (s *Subscription) signalQuit() {
	s.quitOnce.Do(func() {
		s.dead.Store(true)
		if s.quit != nil {
			close(s.quit)
		}
	})
}

func (s *Subscription) closeCh() {
	s.signalQuit()
	// Wait for any in-flight trySend to observe quit and return before close.
	s.inflight.Wait()
	if s.chClosed.CompareAndSwap(false, true) {
		close(s.ch)
	}
}

// trySend attempts to deliver ev. Returns false if the subscription is dead
// or the client is shutting down (waitDone closed) while blocked.
// Critical frames block until buffer space, quit, or waitDone.
func (s *Subscription) trySend(ev Event, waitDone <-chan struct{}) bool {
	if s == nil || s.dead.Load() {
		return false
	}
	select {
	case <-s.quit:
		return false
	default:
	}

	s.inflight.Add(1)
	defer s.inflight.Done()

	// Re-check after joining inflight so closeCh's Wait sees us.
	if s.dead.Load() || s.chClosed.Load() {
		return false
	}

	// Fast non-blocking path.
	select {
	case s.ch <- ev:
		return true
	case <-s.quit:
		return false
	default:
	}

	if !ev.IsCritical() {
		return false
	}

	// Critical backpressure: wait for space, quit, or client death.
	select {
	case s.ch <- ev:
		return true
	case <-s.quit:
		return false
	case <-waitDone:
		return false
	}
}
