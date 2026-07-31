package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// Encoder writes envelopes to an io.Writer with serialized access.
//
// Use [Encoder.Encode] for length-prefixed frames or [Encoder.EncodeJSONL]
// for Bun-compatible JSON lines. Mixing both on one stream is supported at the
// API level but peers must agree on a single framing mode per connection.
type Encoder struct {
	w   io.Writer
	mu  sync.Mutex
	buf []byte // reusable length header scratch
}

// NewEncoder returns an Encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:   w,
		buf: make([]byte, lengthHeaderSize),
	}
}

// Encode writes one length-prefixed JSON envelope.
// Format: 4-byte big-endian uint32 length + UTF-8 JSON body.
// Rejects bodies larger than [MaxFrameSize].
func (e *Encoder) Encode(env Envelope) error {
	if e == nil || e.w == nil {
		return ErrClosed
	}
	body, err := env.Bytes()
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return ErrZeroLength
	}
	if len(body) > MaxFrameSize {
		return fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(body), MaxFrameSize)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	binary.BigEndian.PutUint32(e.buf, uint32(len(body)))
	if _, err := e.w.Write(e.buf); err != nil {
		return err
	}
	_, err = e.w.Write(body)
	return err
}

// EncodeJSONL writes one JSON envelope followed by a single '\n'.
// Rejects bodies larger than [MaxJSONLLineSize].
func (e *Encoder) EncodeJSONL(env Envelope) error {
	if e == nil || e.w == nil {
		return ErrClosed
	}
	body, err := env.Bytes()
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return ErrZeroLength
	}
	if len(body) > MaxJSONLLineSize {
		return fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(body), MaxJSONLLineSize)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, err := e.w.Write(body); err != nil {
		return err
	}
	_, err = e.w.Write([]byte{'\n'})
	return err
}

// EncodeRaw writes a pre-marshaled JSON body as a length-prefixed frame.
// The body must be a complete JSON value and must not exceed [MaxFrameSize].
func (e *Encoder) EncodeRaw(body []byte) error {
	if e == nil || e.w == nil {
		return ErrClosed
	}
	if len(body) == 0 {
		return ErrZeroLength
	}
	if len(body) > MaxFrameSize {
		return fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(body), MaxFrameSize)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	binary.BigEndian.PutUint32(e.buf, uint32(len(body)))
	if _, err := e.w.Write(e.buf); err != nil {
		return err
	}
	_, err := e.w.Write(body)
	return err
}

// EncodeRawJSONL writes a pre-marshaled JSON body followed by '\n'.
func (e *Encoder) EncodeRawJSONL(body []byte) error {
	if e == nil || e.w == nil {
		return ErrClosed
	}
	if len(body) == 0 {
		return ErrZeroLength
	}
	if len(body) > MaxJSONLLineSize {
		return fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(body), MaxJSONLLineSize)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, err := e.w.Write(body); err != nil {
		return err
	}
	_, err := e.w.Write([]byte{'\n'})
	return err
}

// WriteMessage marshals payload into a new envelope and length-prefix encodes it.
func (e *Encoder) WriteMessage(typ, id string, payload any) error {
	env, err := NewEnvelope(typ, id, payload)
	if err != nil {
		return err
	}
	return e.Encode(env)
}

// WriteMessageJSONL marshals payload into a new envelope and JSONL-encodes it.
func (e *Encoder) WriteMessageJSONL(typ, id string, payload any) error {
	env, err := NewEnvelope(typ, id, payload)
	if err != nil {
		return err
	}
	return e.EncodeJSONL(env)
}

// Decoder reads envelopes from an io.Reader.
// Not safe for concurrent use on the same instance.
type Decoder struct {
	r       io.Reader
	br      *bufio.Reader
	maxSize int
	closed  bool
	hdr     [lengthHeaderSize]byte
}

// NewDecoder returns a length-prefixed Decoder reading from r.
// Frame payloads larger than [MaxFrameSize] are rejected.
func NewDecoder(r io.Reader) *Decoder {
	return newDecoder(r, MaxFrameSize)
}

// NewDecoderSize returns a Decoder with a custom max frame size.
// size <= 0 selects [MaxFrameSize].
func NewDecoderSize(r io.Reader, size int) *Decoder {
	if size <= 0 {
		size = MaxFrameSize
	}
	return newDecoder(r, size)
}

func newDecoder(r io.Reader, maxSize int) *Decoder {
	br, ok := r.(*bufio.Reader)
	if !ok {
		// Buffer enough for the header and typical small frames; large frames
		// still stream through via io.ReadFull on the underlying reader.
		br = bufio.NewReaderSize(r, 64*1024)
	}
	return &Decoder{r: r, br: br, maxSize: maxSize}
}

// Decode reads one length-prefixed envelope.
// Returns [io.EOF] only when no bytes of the next frame were consumed.
// A truncated frame after a valid header returns [ErrMalformedLength] or
// a wrapped io error — never a silent partial envelope.
func (d *Decoder) Decode() (Envelope, error) {
	if d == nil || d.closed {
		return Envelope{}, ErrClosed
	}
	if _, err := io.ReadFull(d.br, d.hdr[:]); err != nil {
		if err == io.EOF {
			return Envelope{}, io.EOF
		}
		if err == io.ErrUnexpectedEOF {
			return Envelope{}, fmt.Errorf("%w: truncated length header", ErrMalformedLength)
		}
		return Envelope{}, err
	}
	n := binary.BigEndian.Uint32(d.hdr[:])
	if n == 0 {
		return Envelope{}, ErrZeroLength
	}
	if int(n) > d.maxSize {
		// Drain is not attempted; peer is out of contract. Surface size error.
		return Envelope{}, fmt.Errorf("%w: declared %d > %d", ErrFrameTooLarge, n, d.maxSize)
	}
	body := make([]byte, int(n))
	if _, err := io.ReadFull(d.br, body); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return Envelope{}, fmt.Errorf("%w: truncated body (want %d)", ErrMalformedLength, n)
		}
		return Envelope{}, err
	}
	env, err := ParseEnvelope(body)
	if err != nil {
		return Envelope{}, err
	}
	if err := env.CheckMajor(); err != nil {
		return env, err
	}
	return env, nil
}

// DecodeRaw reads one length-prefixed frame and returns the raw JSON body
// without parsing it into an Envelope.
func (d *Decoder) DecodeRaw() ([]byte, error) {
	if d == nil || d.closed {
		return nil, ErrClosed
	}
	if _, err := io.ReadFull(d.br, d.hdr[:]); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		if err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("%w: truncated length header", ErrMalformedLength)
		}
		return nil, err
	}
	n := binary.BigEndian.Uint32(d.hdr[:])
	if n == 0 {
		return nil, ErrZeroLength
	}
	if int(n) > d.maxSize {
		return nil, fmt.Errorf("%w: declared %d > %d", ErrFrameTooLarge, n, d.maxSize)
	}
	body := make([]byte, int(n))
	if _, err := io.ReadFull(d.br, body); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("%w: truncated body (want %d)", ErrMalformedLength, n)
		}
		return nil, err
	}
	return body, nil
}

// JSONLDecoder reads newline-delimited JSON envelopes.
// This is the Bun rpc-ui compatibility transport.
type JSONLDecoder struct {
	sc      *bufio.Scanner
	maxSize int
	closed  bool
}

// NewJSONLDecoder returns a JSONL decoder reading from r.
// Lines larger than [MaxJSONLLineSize] are rejected.
func NewJSONLDecoder(r io.Reader) *JSONLDecoder {
	return NewJSONLDecoderSize(r, MaxJSONLLineSize)
}

// NewJSONLDecoderSize returns a JSONL decoder with a custom max line size.
func NewJSONLDecoderSize(r io.Reader, size int) *JSONLDecoder {
	if size <= 0 {
		size = MaxJSONLLineSize
	}
	sc := bufio.NewScanner(r)
	// Start with a modest buffer; allow growth up to size.
	sc.Buffer(make([]byte, 0, 64*1024), size)
	sc.Split(bufio.ScanLines)
	return &JSONLDecoder{sc: sc, maxSize: size}
}

// Decode reads the next non-empty line as an envelope.
// Blank lines are skipped. Returns [io.EOF] at end of stream.
//
// Accepts both v1 envelopes ({"v":1,"type":...}) and bare historical Bun RPC
// frames ({"type":"prompt",...} / {"type":"response",...}). Bare frames are
// normalized via [ParseEnvelope] (V=0, fields preserved in Extra / type/id).
func (d *JSONLDecoder) Decode() (Envelope, error) {
	if d == nil || d.closed {
		return Envelope{}, ErrClosed
	}
	for {
		if !d.sc.Scan() {
			if err := d.sc.Err(); err != nil {
				// bufio.Scanner returns ErrTooLong when the line exceeds the buffer.
				if err == bufio.ErrTooLong {
					return Envelope{}, fmt.Errorf("%w: JSONL line", ErrFrameTooLarge)
				}
				return Envelope{}, err
			}
			return Envelope{}, io.EOF
		}
		line := d.sc.Bytes()
		// Scanner reuses the buffer; copy before parse.
		if len(bytesTrimSpace(line)) == 0 {
			continue
		}
		if len(line) > d.maxSize {
			return Envelope{}, fmt.Errorf("%w: JSONL line %d > %d", ErrFrameTooLarge, len(line), d.maxSize)
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		env, err := ParseEnvelope(cp)
		if err != nil {
			return Envelope{}, err
		}
		if err := env.CheckMajor(); err != nil {
			return env, err
		}
		return env, nil
	}
}

// DecodeRaw reads the next non-empty JSONL line and returns a copy of its bytes.
func (d *JSONLDecoder) DecodeRaw() ([]byte, error) {
	if d == nil || d.closed {
		return nil, ErrClosed
	}
	for {
		if !d.sc.Scan() {
			if err := d.sc.Err(); err != nil {
				if err == bufio.ErrTooLong {
					return nil, fmt.Errorf("%w: JSONL line", ErrFrameTooLarge)
				}
				return nil, err
			}
			return nil, io.EOF
		}
		line := d.sc.Bytes()
		if len(bytesTrimSpace(line)) == 0 {
			continue
		}
		if len(line) > d.maxSize {
			return nil, fmt.Errorf("%w: JSONL line %d > %d", ErrFrameTooLarge, len(line), d.maxSize)
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		return cp, nil
	}
}

// Conn pairs an Encoder with either framing mode for a bidirectional stream.
// Readers still use [Decoder] or [JSONLDecoder] separately on the read half.
type Conn struct {
	Enc *Encoder
	// Mode selects the default write framing for helper methods.
	Mode Framing
}

// Framing selects the wire frame format.
type Framing int

const (
	// FramingLengthPrefix is 4-byte BE length + JSON body.
	FramingLengthPrefix Framing = iota
	// FramingJSONL is JSON body + '\n' (Bun rpc-ui default).
	FramingJSONL
)

// NewConn returns a Conn writing to w with the given framing mode.
func NewConn(w io.Writer, mode Framing) *Conn {
	return &Conn{Enc: NewEncoder(w), Mode: mode}
}

// Send encodes env using the connection's framing mode.
func (c *Conn) Send(env Envelope) error {
	if c == nil || c.Enc == nil {
		return ErrClosed
	}
	switch c.Mode {
	case FramingJSONL:
		return c.Enc.EncodeJSONL(env)
	default:
		return c.Enc.Encode(env)
	}
}

// SendMessage builds and sends an envelope with the connection's framing.
func (c *Conn) SendMessage(typ, id string, payload any) error {
	env, err := NewEnvelope(typ, id, payload)
	if err != nil {
		return err
	}
	return c.Send(env)
}

// MarshalEnvelope is a package-level helper that returns length-prefixed bytes
// (header+body) for env. Useful for tests and one-shot buffers.
func MarshalEnvelope(env Envelope) ([]byte, error) {
	body, err := env.Bytes()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, ErrZeroLength
	}
	if len(body) > MaxFrameSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(body), MaxFrameSize)
	}
	out := make([]byte, lengthHeaderSize+len(body))
	binary.BigEndian.PutUint32(out, uint32(len(body)))
	copy(out[lengthHeaderSize:], body)
	return out, nil
}

// MarshalEnvelopeJSONL returns JSON body + newline.
func MarshalEnvelopeJSONL(env Envelope) ([]byte, error) {
	body, err := env.Bytes()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, ErrZeroLength
	}
	if len(body) > MaxJSONLLineSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(body), MaxJSONLLineSize)
	}
	out := make([]byte, len(body)+1)
	copy(out, body)
	out[len(body)] = '\n'
	return out, nil
}

// UnmarshalFrame parses a length-prefixed buffer (header+body) into an Envelope.
func UnmarshalFrame(data []byte) (Envelope, error) {
	if len(data) < lengthHeaderSize {
		return Envelope{}, fmt.Errorf("%w: short buffer", ErrMalformedLength)
	}
	n := binary.BigEndian.Uint32(data[:lengthHeaderSize])
	if n == 0 {
		return Envelope{}, ErrZeroLength
	}
	if int(n) > MaxFrameSize {
		return Envelope{}, fmt.Errorf("%w: declared %d > %d", ErrFrameTooLarge, n, MaxFrameSize)
	}
	if len(data) < lengthHeaderSize+int(n) {
		return Envelope{}, fmt.Errorf("%w: truncated body", ErrMalformedLength)
	}
	if len(data) > lengthHeaderSize+int(n) {
		// Allow exact frames only for this helper; trailing bytes are malformed.
		return Envelope{}, fmt.Errorf("%w: trailing bytes after frame", ErrMalformedLength)
	}
	return ParseEnvelope(data[lengthHeaderSize : lengthHeaderSize+int(n)])
}

// DecodeFlexible parses either a v1 envelope or a bare historical JSON object
// into an Envelope without reading from a stream.
func DecodeFlexible(data []byte) (Envelope, error) {
	return ParseEnvelope(data)
}

// IsJSONObject reports whether data looks like a JSON object (leading '{').
func IsJSONObject(data []byte) bool {
	data = bytesTrimSpace(data)
	return len(data) > 0 && data[0] == '{'
}

func bytesTrimSpace(b []byte) []byte {
	// Inline trim to avoid an extra strings/bytes import cycle in hot paths;
	// bytes.TrimSpace is fine and clearer.
	return jsonTrimSpace(b)
}

func jsonTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) {
		c := b[start]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		start++
	}
	end := len(b)
	for end > start {
		c := b[end-1]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		end--
	}
	return b[start:end]
}

