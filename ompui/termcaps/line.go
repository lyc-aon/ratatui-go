package termcaps

import "strings"

// hasNeedleBefore reports whether needle appears in line with its end at or
// before limit (index+len(needle) <= limit).
func hasNeedleBefore(line, needle string, limit int) bool {
	if needle == "" {
		return false
	}
	idx := strings.Index(line, needle)
	if idx < 0 {
		return false
	}
	return idx+len(needle) <= limit
}

// hasSixelDCSStart reports whether line begins a Sixel DCS within the first 128 bytes:
// ESC P [0-9;]* q
func hasSixelDCSStart(line string) bool {
	limit := len(line)
	if limit > 128 {
		limit = 128
	}
	from := 0
	for {
		start := indexFrom(line, "\x1bP", from, limit)
		if start < 0 || start+3 > limit {
			return false
		}
		i := start + 2
		for i < limit {
			c := line[i]
			if (c >= '0' && c <= '9') || c == ';' {
				i++
				continue
			}
			break
		}
		if i < limit && line[i] == 'q' {
			return true
		}
		from = start + 2
	}
}

func indexFrom(s, substr string, from, limit int) int {
	if from < 0 {
		from = 0
	}
	if from >= limit || from >= len(s) {
		return -1
	}
	end := limit
	if end > len(s) {
		end = len(s)
	}
	rel := strings.Index(s[from:end], substr)
	if rel < 0 {
		return -1
	}
	return from + rel
}

// IsImageLine reports whether line carries an inline image sequence for protocol,
// or a Kitty Unicode placeholder cell (within the first 64 bytes for non-Sixel).
func IsImageLine(line string, protocol ImageProtocol) bool {
	if protocol == ImageProtocolNone {
		return false
	}
	if protocol == ImageProtocolSixel {
		return hasSixelDCSStart(line)
	}
	return hasNeedleBefore(line, string(protocol), 64) || hasNeedleBefore(line, KittyPlaceholder, 64)
}

// IsImageLine reports whether line carries this terminal's image protocol.
func (t TerminalInfo) IsImageLine(line string) bool {
	return IsImageLine(line, t.ImageProtocol)
}
