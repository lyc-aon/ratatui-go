package input

// seqStatus is the completeness of a candidate escape sequence.
type seqStatus int

const (
	seqComplete seqStatus = iota
	seqIncomplete
	seqNotEscape
)

// isCompleteSequence reports whether data is a complete escape sequence,
// needs more bytes, or is not an escape at all.
//
// Port of stdin-buffer.ts isCompleteSequence.
func isCompleteSequence(data []byte) seqStatus {
	if len(data) == 0 || data[0] != esc {
		return seqNotEscape
	}
	if len(data) == 1 {
		return seqIncomplete
	}
	after := data[1:]

	// CSI: ESC [
	if after[0] == '[' {
		// Old-style X10 mouse: ESC [ M + 3 bytes = 6 total
		if len(after) >= 2 && after[1] == 'M' {
			if len(data) >= 6 {
				return seqComplete
			}
			return seqIncomplete
		}
		return isCompleteCSI(data)
	}

	// OSC: ESC ]
	if after[0] == ']' {
		return isCompleteOSC(data)
	}

	// DCS: ESC P
	if after[0] == 'P' {
		return isCompleteDCS(data)
	}

	// APC: ESC _
	if after[0] == '_' {
		return isCompleteAPC(data)
	}

	// SS3: ESC O + one character
	if after[0] == 'O' {
		if len(after) >= 2 {
			return seqComplete
		}
		return seqIncomplete
	}

	// ESC-prefixed (metaSendsEscape): only when inner ESC starts CSI/SS3.
	// Bare double-ESC remains complete to avoid flush-timeout lag.
	if after[0] == esc {
		inner := data[1:]
		if len(inner) >= 2 {
			third := inner[1]
			if third == '[' || third == 'O' {
				return isCompleteSequence(inner)
			}
		}
		return seqComplete
	}

	// Meta key: ESC + single character
	if len(after) == 1 {
		return seqComplete
	}

	// Unknown escape — treat complete
	return seqComplete
}

// isCompleteCSI: CSI sequences end with final byte 0x40-0x7E.
// SGR mouse requires <digits;digits;digits[Mm].
func isCompleteCSI(data []byte) seqStatus {
	if len(data) < 2 || data[0] != esc || data[1] != '[' {
		return seqComplete
	}
	if len(data) < 3 {
		return seqIncomplete
	}
	payload := data[2:]
	last := payload[len(payload)-1]
	if last < 0x40 || last > 0x7e {
		return seqIncomplete
	}

	// SGR mouse: ESC [ < B ; X ; Y M/m
	if payload[0] == '<' {
		if isStrictSGRMousePayload(payload) {
			return seqComplete
		}
		// Ends with M/m but structure wrong → still incomplete
		if last == 'M' || last == 'm' {
			if isStrictSGRMousePayload(payload) {
				return seqComplete
			}
		}
		return seqIncomplete
	}
	return seqComplete
}

// isStrictSGRMousePayload matches /^<\d+;\d+;\d+[Mm]$/.
func isStrictSGRMousePayload(payload []byte) bool {
	n := len(payload)
	if n < 6 { // <0;0;0M minimum
		return false
	}
	if payload[0] != '<' {
		return false
	}
	term := payload[n-1]
	if term != 'M' && term != 'm' {
		return false
	}
	// Split body on ';' — need exactly 3 numeric fields.
	body := payload[1 : n-1]
	fields := 0
	i := 0
	for fields < 3 {
		if i >= len(body) {
			return false
		}
		start := i
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
		fields++
		if fields == 3 {
			return i == len(body)
		}
		if i >= len(body) || body[i] != ';' {
			return false
		}
		i++ // skip ';'
	}
	return false
}

func isCompleteOSC(data []byte) seqStatus {
	if len(data) < 2 || data[0] != esc || data[1] != ']' {
		return seqComplete
	}
	// ST (ESC \) or BEL
	if len(data) >= 2 && data[len(data)-1] == bel {
		return seqComplete
	}
	if len(data) >= 3 && data[len(data)-2] == esc && data[len(data)-1] == '\\' {
		return seqComplete
	}
	return seqIncomplete
}

func isCompleteDCS(data []byte) seqStatus {
	if len(data) < 2 || data[0] != esc || data[1] != 'P' {
		return seqComplete
	}
	if len(data) >= 3 && data[len(data)-2] == esc && data[len(data)-1] == '\\' {
		return seqComplete
	}
	return seqIncomplete
}

func isCompleteAPC(data []byte) seqStatus {
	if len(data) < 2 || data[0] != esc || data[1] != '_' {
		return seqComplete
	}
	if len(data) >= 3 && data[len(data)-2] == esc && data[len(data)-1] == '\\' {
		return seqComplete
	}
	return seqIncomplete
}

// isSGRMousePartial reports whether buf matches /^\x1b\[<[\d;]*$/ —
// an unambiguous partial SGR mouse report prefix.
func isSGRMousePartial(buf []byte) bool {
	if len(buf) < 3 {
		return false
	}
	if buf[0] != esc || buf[1] != '[' || buf[2] != '<' {
		return false
	}
	for i := 3; i < len(buf); i++ {
		c := buf[i]
		if (c >= '0' && c <= '9') || c == ';' {
			continue
		}
		return false
	}
	return true
}

// utf8ScalarLen returns the byte length of the Unicode scalar at buf[pos],
// or 1 for invalid/incomplete lead (consume one byte to make progress).
func utf8ScalarLen(buf []byte, pos int) int {
	if pos >= len(buf) {
		return 0
	}
	c := buf[pos]
	if c < 0x80 {
		return 1
	}
	// Multi-byte: determine expected length from lead.
	var need int
	switch {
	case c&0xe0 == 0xc0:
		need = 2
	case c&0xf0 == 0xe0:
		need = 3
	case c&0xf8 == 0xf0:
		need = 4
	default:
		return 1 // invalid lead
	}
	if pos+need > len(buf) {
		// Incomplete multi-byte sequence — hold for more data by returning 0
		// only when caller wants incomplete; extractCompleteSequences treats
		// non-ESC as one scalar. Incomplete UTF-8 mid-stream is rare on TTY;
		// hold remainder by returning -1 sentinel.
		return -1
	}
	// Validate continuations
	for i := 1; i < need; i++ {
		if buf[pos+i]&0xc0 != 0x80 {
			return 1
		}
	}
	return need
}

// extractCompleteSequences splits buf into complete sequences and a remainder.
// Index-based scan — O(n) for plain-text bursts (no per-byte alloc of the whole buffer).
//
// Port of stdin-buffer.ts extractCompleteSequences.
func extractCompleteSequences(buf []byte) (sequences [][]byte, remainder []byte) {
	length := len(buf)
	pos := 0
	for pos < length {
		if buf[pos] == esc {
			end := pos + 1
			consumed := false
			for end <= length {
				candidate := buf[pos:end]
				status := isCompleteSequence(candidate)
				if status == seqIncomplete {
					end++
					continue
				}
				// "\x1b\x1b" alone parses complete (legacy alt+esc), but when the
				// next byte opens CSI/SS3 this is ESC prefixing another sequence.
				// Keep growing; if buffer ends here, hold the partial.
				if len(candidate) == 2 && candidate[0] == esc && candidate[1] == esc {
					if end >= length {
						return sequences, buf[pos:]
					}
					next := buf[end]
					if next == '[' || next == 'O' {
						end++
						continue
					}
				}
				// ESC + SGR mouse is never a meta chord: deliver bare ESC and
				// the report separately.
				if len(candidate) >= 4 &&
					candidate[0] == esc && candidate[1] == esc &&
					candidate[2] == '[' && candidate[3] == '<' {
					sequences = append(sequences, []byte{esc}, dupBytes(candidate[1:]))
					pos = end
					consumed = true
					break
				}
				sequences = append(sequences, dupBytes(candidate))
				pos = end
				consumed = true
				break
			}
			if !consumed {
				return sequences, buf[pos:]
			}
		} else {
			n := utf8ScalarLen(buf, pos)
			if n < 0 {
				// Incomplete UTF-8 — hold remainder
				return sequences, buf[pos:]
			}
			if n == 0 {
				return sequences, buf[pos:]
			}
			sequences = append(sequences, dupBytes(buf[pos:pos+n]))
			pos += n
		}
	}
	return sequences, nil
}

// dupBytes copies b so callers can retain frames while the parent buffer grows.
// Small frames dominate; copy keeps ownership simple and avoids aliasing.
func dupBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// indexOfBytes is bytes.Index without importing for a few call sites that
// want the paste-marker search with an explicit needle.
func indexOfBytes(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	if len(needle) > len(haystack) {
		return -1
	}
	// Fast single-byte
	if len(needle) == 1 {
		c := needle[0]
		for i, b := range haystack {
			if b == c {
				return i
			}
		}
		return -1
	}
	// Simple search — paste markers are short (6 bytes)
outer:
	for i := 0; i+len(needle) <= len(haystack); i++ {
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// hasPrefix reports whether b begins with prefix.
func hasPrefix(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	for i := range prefix {
		if b[i] != prefix[i] {
			return false
		}
	}
	return true
}

// hasSuffix reports whether b ends with suffix.
func hasSuffix(b, suffix []byte) bool {
	if len(b) < len(suffix) {
		return false
	}
	off := len(b) - len(suffix)
	for i := range suffix {
		if b[off+i] != suffix[i] {
			return false
		}
	}
	return true
}
