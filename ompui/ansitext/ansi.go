package ansitext

// ansiSeqLen returns the byte length of a complete ANSI/control string sequence
// starting at s[pos], or 0 if none. Mirrors pi-natives ansi_seq_len_u16:
//
//	CSI  ESC [ ... final(0x40-0x7e)
//	OSC  ESC ] ... BEL | ST(ESC \)
//	DCS/SOS/PM/APC  ESC P|X|^|_ ... BEL | ST
//	ESC + intermediates(0x20-0x2f) + final(0x30-0x7e)
//	two-byte ESC final(0x40-0x7e)
func ansiSeqLen(s string, pos int) int {
	n := len(s)
	if pos >= n || s[pos] != esc {
		return 0
	}
	if pos+1 >= n {
		return 0
	}
	switch s[pos+1] {
	case '[': // CSI
		for i := pos + 2; i < n; i++ {
			b := s[i]
			if b >= 0x40 && b <= 0x7e {
				return i - pos + 1
			}
		}
		return 0
	case ']': // OSC
		for i := pos + 2; i < n; i++ {
			b := s[i]
			if b == bel {
				return i - pos + 1
			}
			if b == esc && i+1 < n && s[i+1] == '\\' {
				return i - pos + 2
			}
		}
		return 0
	case 'P', 'X', '^', '_': // DCS, SOS, PM, APC (cursor marker is BEL-APC)
		for i := pos + 2; i < n; i++ {
			b := s[i]
			if b == bel {
				return i - pos + 1
			}
			if b == esc && i+1 < n && s[i+1] == '\\' {
				return i - pos + 2
			}
		}
		return 0
	case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
		0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f:
		// ESC + intermediates + final
		for i := pos + 2; i < n; i++ {
			b := s[i]
			if b >= 0x30 && b <= 0x7e {
				return i - pos + 1
			}
		}
		return 0
	default:
		if s[pos+1] >= 0x40 && s[pos+1] <= 0x7e {
			return 2
		}
		return 0
	}
}

func isSGR(seq string) bool {
	return len(seq) >= 3 && seq[1] == '[' && seq[len(seq)-1] == 'm'
}

// Segment is a parsed slice of an ANSI-bearing string. Exported so higher
// ompui packages can walk SGR/OSC8 boundaries without reimplementing the
// scanner. Text is a substring of the original input (no copy).
type Segment struct {
	// Kind is "text", "sgr", "osc8", "osc66", or "other".
	Kind string
	// Text is the raw bytes of this segment.
	Text string
	// Width is the visible cell width (0 for pure control segments; OSC 66
	// spans report their scaled payload width).
	Width int
}

// ParseSegments splits s into text and control segments, preserving order.
// Incomplete escape prefixes become single-byte "other" units so scanners never
// hang. Grapheme clusters inside text runs are not further split here; callers
// that need grapheme boundaries should measure text segments with VisibleWidth
// or walk them via the text package.
func ParseSegments(s string) []Segment {
	if s == "" {
		return nil
	}
	out := make([]Segment, 0, 8)
	i := 0
	n := len(s)
	for i < n {
		if s[i] == esc {
			if seqLen := ansiSeqLen(s, i); seqLen > 0 {
				seq := s[i : i+seqLen]
				kind, w := classifySeq(seq)
				out = append(out, Segment{Kind: kind, Text: seq, Width: w})
				i += seqLen
				continue
			}
			// Lone/malformed ESC: emit as zero-width other, advance one byte.
			out = append(out, Segment{Kind: "other", Text: s[i : i+1], Width: 0})
			i++
			continue
		}
		// Gather a run of non-escape bytes as one text segment.
		start := i
		for i < n && s[i] != esc {
			i++
		}
		run := s[start:i]
		out = append(out, Segment{Kind: "text", Text: run, Width: plainVisibleWidth(run)})
	}
	return out
}

func classifySeq(seq string) (kind string, width int) {
	if isSGR(seq) {
		return "sgr", 0
	}
	if isOSC8(seq) {
		return "osc8", 0
	}
	if info, ok := osc66Info(seq); ok {
		return "osc66", info.width
	}
	return "other", 0
}

func isOSC8(seq string) bool {
	// ESC ] 8 ; ... BEL|ST
	if len(seq) < 5 || seq[0] != esc || seq[1] != ']' || seq[2] != '8' || seq[3] != ';' {
		return false
	}
	return true
}

type osc66 struct {
	payload string
	scale   int
	width   int
}

func osc66Info(seq string) (osc66, bool) {
	meta, payload, ok := osc66MetaPayload(seq)
	if !ok {
		return osc66{}, false
	}
	scale, explicit := parseOSC66Meta(meta)
	base := explicit
	if base < 0 {
		base = plainVisibleWidth(payload)
	}
	return osc66{payload: payload, scale: scale, width: scale * base}, true
}

func osc66MetaPayload(seq string) (meta, payload string, ok bool) {
	// ESC ] 6 6 ; <meta> ; <payload> BEL|ST
	if len(seq) < 7 || seq[0] != esc || seq[1] != ']' || seq[2] != '6' || seq[3] != '6' || seq[4] != ';' {
		return "", "", false
	}
	payloadEnd := 0
	switch {
	case seq[len(seq)-1] == bel:
		payloadEnd = len(seq) - 1
	case len(seq) >= 8 && seq[len(seq)-2] == esc && seq[len(seq)-1] == '\\':
		payloadEnd = len(seq) - 2
	default:
		return "", "", false
	}
	for sep := 5; sep < payloadEnd; sep++ {
		if seq[sep] == ';' {
			return seq[5:sep], seq[sep+1 : payloadEnd], true
		}
	}
	return "", "", false
}

func parseOSC66Meta(meta string) (scale int, explicitWidth int) {
	scale = 1
	explicitWidth = -1 // sentinel: absent
	partStart := 0
	for i := 0; i <= len(meta); i++ {
		if i != len(meta) && meta[i] != ':' {
			continue
		}
		part := meta[partStart:i]
		if eq := indexByte(part, '='); eq == 1 {
			key := part[0]
			val := parseASCIIInt(part[2:])
			switch key {
			case 's':
				if val >= 1 && val <= 7 {
					scale = val
				}
			case 'w':
				if val > 0 {
					explicitWidth = val
				}
			}
		}
		partStart = i + 1
	}
	return scale, explicitWidth
}

func indexByte(s string, c byte) int {
	for i := range s {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func parseASCIIInt(s string) int {
	if s == "" {
		return -1
	}
	v := 0
	for i := range s {
		b := s[i]
		if b < '0' || b > '9' {
			return -1
		}
		v = v*10 + int(b-'0')
	}
	return v
}

func isSGRParamByte(c byte) bool {
	return (c >= '0' && c <= '9') || c == ';' || c == ':'
}
