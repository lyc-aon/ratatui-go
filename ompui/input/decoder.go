package input

import (
	"time"
	"unicode/utf8"

	"github.com/michaelkelly/ratatui-go/ompui/event"
)

// Options configures Decoder timing and platform heuristics.
type Options struct {
	// FlushTimeout is how long to wait for an incomplete escape (default 50ms).
	FlushTimeout time.Duration
	// PartialHoldTimeout is extra hold time for unambiguous partials (default 150ms).
	PartialHoldTimeout time.Duration
	// PasteTimeout is paste-mode inactivity recovery (default 1s).
	PasteTimeout time.Duration
	// PasteByteLimit caps paste assembly (default 64 MiB).
	PasteByteLimit int
	// KittyActive enables kitty-aware partial holds and release/repeat detection.
	// Updated at runtime via SetKittyActive.
	KittyActive bool
	// WindowsTerminal enables raw 0x08 → ctrl+backspace.
	WindowsTerminal bool
	// Now, if non-nil, supplies the current time (tests); default time.Now.
	Now func() time.Time
}

func (o *Options) withDefaults() Options {
	out := *o
	if out.FlushTimeout <= 0 {
		out.FlushTimeout = defaultFlushTimeout
	}
	if out.PartialHoldTimeout <= 0 {
		out.PartialHoldTimeout = defaultPartialHoldMax
	}
	if out.PasteTimeout <= 0 {
		out.PasteTimeout = defaultPasteInactivity
	}
	if out.PasteByteLimit <= 0 {
		out.PasteByteLimit = defaultPasteMaxBytes
	}
	if out.Now == nil {
		out.Now = time.Now
	}
	return out
}

// Decoder is an incremental byte-stream input framer and semantic decoder.
//
// Feed bytes with Write. Drain decoded events with Next / Events.
// Timers are cooperative: call PopDue(now) from the owner's select loop
// (or use Deadline to arm). No goroutines, no file ownership, no probes.
type Decoder struct {
	opts Options

	buf []byte // incomplete escape remainder

	// paste assembly
	pasteMode    bool
	pasteChunks  [][]byte
	pasteOverlap []byte
	pasteBytes   int
	pasteArmAt   time.Time // zero = no watchdog; else last chunk time

	// flush / partial hold
	flushArmAt       time.Time // when flush timer was armed (zero = none)
	partialHoldStart time.Time
	flushDeferred    bool // true after flush timeout fired; next fresh ESC flushes first

	// kitty printable dedup
	pendingKittyCP  int32
	pendingKittyAt  time.Time
	hasPendingKitty bool

	// pending decoded events
	out []event.Event
}

// NewDecoder constructs a Decoder with opts (defaults applied).
func NewDecoder(opts Options) *Decoder {
	o := opts.withDefaults()
	return &Decoder{opts: o}
}

// SetKittyActive updates the runtime kitty-protocol flag (partial hold + release).
func (d *Decoder) SetKittyActive(active bool) {
	d.opts.KittyActive = active
}

// KittyActive reports the current kitty flag.
func (d *Decoder) KittyActive() bool { return d.opts.KittyActive }

// SetWindowsTerminal updates the raw-backspace heuristic flag.
func (d *Decoder) SetWindowsTerminal(v bool) {
	d.opts.WindowsTerminal = v
}

// Write feeds a chunk of raw terminal bytes. Decoded events are queued.
//
// A lone high byte is held until more bytes or the flush deadline. This avoids
// misclassifying the first byte of split UTF-8 as a legacy Meta key.
func (d *Decoder) Write(p []byte) {
	if len(p) == 0 {
		if len(d.buf) == 0 {
			d.emitDataSequence(nil)
		}
		return
	}

	str := p

	if d.flushDeferred && d.isFreshEscapeAfterDeferredFlush(str) {
		d.flushExpired(d.opts.Now())
	} else {
		d.clearFlushTimer()
	}

	// Append without quadratic reallocation habit: grow buf capacity geometrically.
	d.buf = append(d.buf, str...)

	if d.pasteMode {
		chunk := d.buf
		d.buf = nil
		d.consumePasteChunk(chunk)
		return
	}

	// Bracketed paste start
	if start := indexOfBytes(d.buf, pasteStart); start != -1 {
		if start > 0 {
			before := d.buf[:start]
			seqs, rem := extractCompleteSequences(before)
			for _, s := range seqs {
				d.emitDataSequence(s)
			}
			// rem before paste start should be empty (paste start is complete-ish);
			// if not, drop into paste after emitting nothing more.
			_ = rem
		}
		d.hasPendingKitty = false
		d.buf = d.buf[start+len(pasteStart):]
		first := d.buf
		d.buf = nil
		d.pasteMode = true
		d.pasteChunks = d.pasteChunks[:0]
		d.pasteOverlap = d.pasteOverlap[:0]
		d.pasteBytes = 0
		d.consumePasteChunk(first)
		return
	}

	seqs, rem := extractCompleteSequences(d.buf)
	d.buf = append(d.buf[:0], rem...)
	for _, s := range seqs {
		d.emitDataSequence(s)
	}
	if len(d.buf) > 0 {
		d.armFlushTimer(d.opts.Now())
	} else {
		d.partialHoldStart = time.Time{}
	}
}

// Push is an alias for Write for byte-stream callers.
func (d *Decoder) Push(p []byte) { d.Write(p) }

// Next returns the next queued event, or false if none.
func (d *Decoder) Next() (event.Event, bool) {
	if len(d.out) == 0 {
		return event.Event{}, false
	}
	ev := d.out[0]
	copy(d.out, d.out[1:])
	d.out = d.out[:len(d.out)-1]
	return ev, true
}

// Events drains all queued events.
func (d *Decoder) Events() []event.Event {
	if len(d.out) == 0 {
		return nil
	}
	out := d.out
	d.out = nil
	return out
}

// BufferedLen returns bytes held waiting for sequence completion.
func (d *Decoder) BufferedLen() int { return len(d.buf) }

// InPaste reports whether bracketed-paste assembly is active.
func (d *Decoder) InPaste() bool { return d.pasteMode }

// Deadline returns the next timer fire time and true when a timer is armed.
// Owners select on time.Until(deadline). Zero time means no timer.
func (d *Decoder) Deadline() (time.Time, bool) {
	var earliest time.Time
	has := false
	if !d.flushArmAt.IsZero() {
		fire := d.flushArmAt.Add(d.opts.FlushTimeout)
		earliest = fire
		has = true
	}
	if d.pasteMode && !d.pasteArmAt.IsZero() {
		fire := d.pasteArmAt.Add(d.opts.PasteTimeout)
		if !has || fire.Before(earliest) {
			earliest = fire
			has = true
		}
	}
	return earliest, has
}

// PopDue fires any expired timers (flush / paste watchdog) using now.
// Call after Deadline elapses or periodically from the owner loop.
func (d *Decoder) PopDue(now time.Time) {
	if now.IsZero() {
		now = d.opts.Now()
	}
	// Paste watchdog first
	if d.pasteMode && !d.pasteArmAt.IsZero() {
		if now.Sub(d.pasteArmAt) >= d.opts.PasteTimeout {
			d.abortPaste()
		}
	}
	// Flush
	if !d.flushArmAt.IsZero() && now.Sub(d.flushArmAt) >= d.opts.FlushTimeout {
		// Mark deferred and flush. In Go the owner loop is select-based so the
		// TS setTimeout(0) tear-proofing is approximated by: caller should
		// non-blocking-read once more before PopDue; we still set flushDeferred
		// so a racing Write of a fresh ESC splits correctly.
		d.flushDeferred = true
		d.flushArmAt = time.Time{}
		d.flushExpired(now)
	}
}

// Flush forces delivery of any buffered partial as a raw/decoded sequence.
func (d *Decoder) Flush() []event.Event {
	d.clearFlushTimer()
	if len(d.buf) == 0 {
		return d.Events()
	}
	seq := dupBytes(d.buf)
	seq = legacyMetaOnTimeout(seq)
	d.buf = d.buf[:0]
	d.hasPendingKitty = false
	d.emitDataSequence(seq)
	return d.Events()
}

// Clear drops all buffered state and queued events.
func (d *Decoder) Clear() {
	d.clearFlushTimer()
	d.pasteArmAt = time.Time{}
	d.buf = d.buf[:0]
	d.pasteMode = false
	d.pasteChunks = d.pasteChunks[:0]
	d.pasteOverlap = d.pasteOverlap[:0]
	d.pasteBytes = 0
	d.hasPendingKitty = false
	d.partialHoldStart = time.Time{}
	d.flushDeferred = false
	d.out = d.out[:0]
}

// consumePasteChunk assembles paste with O(total) chunk list + overlap tail.
func (d *Decoder) consumePasteChunk(chunk []byte) {
	probe := append(append([]byte{}, d.pasteOverlap...), chunk...)
	if indexOfBytes(probe, pasteEnd) < 0 {
		// retain chunk
		d.pasteChunks = append(d.pasteChunks, dupBytes(chunk))
		d.pasteBytes += len(chunk)
		keep := len(pasteEnd) - 1
		if len(probe) > keep {
			d.pasteOverlap = append(d.pasteOverlap[:0], probe[len(probe)-keep:]...)
		} else {
			d.pasteOverlap = append(d.pasteOverlap[:0], probe...)
		}
		if d.pasteBytes > d.opts.PasteByteLimit {
			d.abortPaste()
			return
		}
		d.pasteArmAt = d.opts.Now()
		return
	}

	// End marker present: join once
	var flat []byte
	if len(d.pasteChunks) > 0 {
		total := len(chunk)
		for _, c := range d.pasteChunks {
			total += len(c)
		}
		flat = make([]byte, 0, total)
		for _, c := range d.pasteChunks {
			flat = append(flat, c...)
		}
		flat = append(flat, chunk...)
	} else {
		flat = chunk
	}
	endIndex := indexOfBytes(flat, pasteEnd)
	content := flat[:endIndex]
	remaining := flat[endIndex+len(pasteEnd):]

	d.pasteArmAt = time.Time{}
	d.pasteMode = false
	d.pasteChunks = d.pasteChunks[:0]
	d.pasteOverlap = d.pasteOverlap[:0]
	d.pasteBytes = 0
	d.hasPendingKitty = false

	decoded := decodeReencodedPasteControls(content)
	raw := make([]byte, 0, len(pasteStart)+len(content)+len(pasteEnd))
	raw = append(raw, pasteStart...)
	raw = append(raw, content...)
	raw = append(raw, pasteEnd...)
	d.out = append(d.out, event.PasteEvent(string(decoded), raw))

	if len(remaining) > 0 {
		d.Write(remaining)
	}
}

func (d *Decoder) abortPaste() {
	d.pasteArmAt = time.Time{}
	var content []byte
	if len(d.pasteChunks) > 0 {
		total := 0
		for _, c := range d.pasteChunks {
			total += len(c)
		}
		content = make([]byte, 0, total)
		for _, c := range d.pasteChunks {
			content = append(content, c...)
		}
	}
	d.pasteMode = false
	d.pasteChunks = d.pasteChunks[:0]
	d.pasteOverlap = d.pasteOverlap[:0]
	d.pasteBytes = 0
	decoded := decodeReencodedPasteControls(content)
	raw := make([]byte, 0, len(pasteStart)+len(content))
	raw = append(raw, pasteStart...)
	raw = append(raw, content...)
	d.out = append(d.out, event.PasteEvent(string(decoded), raw))
}

func (d *Decoder) emitDataSequence(seq []byte) {
	// Kitty printable dedup
	if len(seq) > 0 {
		r, n := utf8.DecodeRune(seq)
		if n == len(seq) && r != utf8.RuneError {
			if d.hasPendingKitty && int32(r) == d.pendingKittyCP {
				if d.opts.Now().Sub(d.pendingKittyAt) <= kittyPrintableDedupWindow {
					d.hasPendingKitty = false
					return
				}
			}
		}
	}
	cp := parseUnmodifiedKittyPrintableCodepoint(seq)
	if cp >= 0 {
		d.pendingKittyCP = cp
		d.pendingKittyAt = d.opts.Now()
		d.hasPendingKitty = true
	} else {
		d.hasPendingKitty = false
	}

	d.out = append(d.out, d.decodeSequence(seq))
}

func (d *Decoder) decodeSequence(seq []byte) event.Event {
	if len(seq) == 0 {
		return event.RawEvent(nil)
	}

	// Focus events: CSI I / CSI O
	if len(seq) == 3 && seq[0] == esc && seq[1] == '[' {
		if seq[2] == 'I' {
			return event.FocusEvent(true, dupBytes(seq))
		}
		if seq[2] == 'O' {
			return event.FocusEvent(false, dupBytes(seq))
		}
	}

	// SGR mouse
	if isSGRMousePrefix(seq) {
		if m, ok := parseSGRMouse(seq); ok {
			return event.MouseEvent(m, dupBytes(seq))
		}
	}
	// X10 mouse
	if isX10MousePrefix(seq) {
		if m, ok := parseX10Mouse(seq); ok {
			return event.MouseEvent(m, dupBytes(seq))
		}
	}

	opts := KeyParseOptions{
		KittyActive:     d.opts.KittyActive,
		WindowsTerminal: d.opts.WindowsTerminal,
	}

	// Keypad text fast path (numpad digits/ops as text-producing keys).
	// Mirrors keys.ts decodeKittyKeypadText before native parseKey.
	if text, ok := decodeKittyKeypadText(seq); ok {
		_, key, keyOK := parseKeyFull(seq, opts)
		if !keyOK {
			key = event.Key{Action: event.ActionPress}
		}
		key.ID = text
		key.Code = event.Code(text)
		key.Text = text
		if key.Action == 0 {
			key.Action = event.ActionPress
		}
		return event.KeyEvent(key, dupBytes(seq))
	}

	id, key, keyOK := parseKeyFull(seq, opts)
	if keyOK {
		// Attach printable text when appropriate
		if text, ok := decodePrintableKey(seq); ok {
			key.Text = text
		} else if key.Mods == 0 && len(id) == 1 {
			key.Text = id
		} else if key.Mods == 0 && (id == "space") {
			key.Text = " "
		}
		// Release events: keep ActionRelease; ID may still be set
		if key.Action == event.ActionRelease && !d.opts.KittyActive {
			// ignore release classification when kitty off — treat as press
			key.Action = event.ActionPress
		}
		if key.ID == "" && id != "" {
			key.ID = id
		}
		return event.KeyEvent(key, dupBytes(seq))
	}

	// Plain printable UTF-8 scalar(s)
	if text, ok := extractPrintableText(seq); ok {
		return event.TextEvent(text, dupBytes(seq))
	}

	// Unrecognized complete sequence
	return event.RawEvent(dupBytes(seq))
}

func (d *Decoder) armFlushTimer(now time.Time) {
	d.flushArmAt = now
	d.flushDeferred = false
}

func (d *Decoder) clearFlushTimer() {
	d.flushArmAt = time.Time{}
	d.flushDeferred = false
}

func (d *Decoder) isFreshEscapeAfterDeferredFlush(str []byte) bool {
	if len(str) == 0 || str[0] != esc || len(d.buf) == 0 {
		return false
	}
	// ESC \ completing OSC/DCS/APC is a continuation, not fresh
	if len(str) >= 2 && str[0] == esc && str[1] == '\\' {
		if hasPrefix(d.buf, []byte{esc, ']'}) ||
			hasPrefix(d.buf, []byte{esc, 'P'}) ||
			hasPrefix(d.buf, []byte{esc, '_'}) {
			return false
		}
	}
	return true
}

func (d *Decoder) shouldHoldPartial() bool {
	return isSGRMousePartial(d.buf) || d.opts.KittyActive
}

func (d *Decoder) flushExpired(now time.Time) {
	if len(d.buf) == 0 {
		d.partialHoldStart = time.Time{}
		return
	}
	if d.shouldHoldPartial() {
		if d.partialHoldStart.IsZero() {
			d.partialHoldStart = now
		}
		if now.Sub(d.partialHoldStart) < d.opts.PartialHoldTimeout {
			// re-arm flush for another cycle
			d.armFlushTimer(now)
			return
		}
	}
	d.partialHoldStart = time.Time{}
	seq := dupBytes(d.buf)
	seq = legacyMetaOnTimeout(seq)
	d.buf = d.buf[:0]
	d.hasPendingKitty = false
	d.flushDeferred = false
	d.emitDataSequence(seq)
}

func legacyMetaOnTimeout(seq []byte) []byte {
	if len(seq) == 1 && seq[0] > 127 {
		return []byte{esc, seq[0] - 128}
	}
	return seq
}

// DecodeFrame decodes a single already-framed sequence into an event without
// buffer state. Useful for tests and probe-router pass-through.
func DecodeFrame(seq []byte, opts Options) event.Event {
	d := NewDecoder(opts)
	return d.decodeSequence(legacyMetaOnTimeout(dupBytes(seq)))
}

// ParseKeyID is a convenience wrapper around ParseKey with options.
func ParseKeyID(data []byte, kittyActive, windowsTerminal bool) string {
	return ParseKey(data, KeyParseOptions{KittyActive: kittyActive, WindowsTerminal: windowsTerminal})
}
