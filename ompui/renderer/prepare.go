package renderer

import (
	"strings"
	"unicode/utf8"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
)

// preparedEntry caches one width-fitted, normalized content row (no terminator).
type preparedEntry struct {
	raw   string
	width int
	line  string
	valid bool
}

// prepareCache is a row-aligned prepared-frame cache. preparedValid counts the
// leading rows known prepared against the current composed frame; a stable-
// prefix invalidation lowers it so only the dirty tail is revalidated.
type prepareCache struct {
	meta          []preparedEntry
	prepared      []string
	preparedValid int
}

// InvalidateTo lowers the prepared-valid floor to n (stable-prefix splice).
// n < 0 is treated as 0.
func (c *prepareCache) InvalidateTo(n int) {
	if n < 0 {
		n = 0
	}
	if n < c.preparedValid {
		c.preparedValid = n
	}
}

// Reset clears the cache (geometry / full replace).
func (c *prepareCache) Reset() {
	c.meta = c.meta[:0]
	c.prepared = c.prepared[:0]
	c.preparedValid = 0
}

// PrepareFrame width-fits and normalizes frame rows, reusing cached entries
// when raw+width match. Returns a slice header into the cache (valid until the
// next PrepareFrame / Reset). Image lines pass through untouched.
func (c *prepareCache) PrepareFrame(frame []string, width int, isImage func(string) bool) []string {
	if width < 1 {
		width = 1
	}
	n := len(frame)
	if cap(c.prepared) < n {
		c.prepared = make([]string, n)
		c.meta = make([]preparedEntry, n)
	} else {
		c.prepared = c.prepared[:n]
		if len(c.meta) < n {
			grow := make([]preparedEntry, n)
			copy(grow, c.meta)
			c.meta = grow
		} else {
			c.meta = c.meta[:n]
		}
	}
	start := c.preparedValid
	if start > n {
		start = n
	}
	for i := start; i < n; i++ {
		raw := frame[i]
		cached := c.meta[i]
		if cached.valid && cached.raw == raw && cached.width == width {
			c.prepared[i] = cached.line
			continue
		}
		line := prepareLine(raw, width, isImage)
		c.meta[i] = preparedEntry{raw: raw, width: width, line: line, valid: true}
		c.prepared[i] = line
	}
	c.preparedValid = n
	return c.prepared
}

// prepareLinesArray is the stateless path for overlay-composited windows and
// alt-screen frames (does not touch the persistent cache).
func prepareLinesArray(lines []string, width int, isImage func(string) bool) []string {
	if width < 1 {
		width = 1
	}
	out := make([]string, len(lines))
	for i, raw := range lines {
		out[i] = prepareLine(raw, width, isImage)
	}
	return out
}

func prepareLine(raw string, width int, isImage func(string) bool) string {
	if isImage != nil && isImage(raw) {
		return raw
	}
	source := lineFitSource(raw, width)
	normalized := ansitext.NormalizeTerminalOutput(source)
	if asciiW, ok := ansiASCIILineWidth(normalized, width); ok {
		if asciiW <= width {
			return normalized
		}
	} else if ansitext.VisibleWidth(normalized) <= width {
		return normalized
	}
	return ansitext.TruncateToWidth(normalized, width, "")
}

// lineFitSource clamps an over-long source so width-fit cannot thrash on
// zero-width-heavy prefixes. Appends SegmentReset so styles do not bleed.
func lineFitSource(raw string, width int) string {
	safeWidth := width
	if safeWidth < 1 {
		safeWidth = 1
	}
	maxSource := safeWidth * lineFitSourceWidthMultiplier
	if maxSource < lineFitMinSourceCodeUnits {
		maxSource = lineFitMinSourceCodeUnits
	}
	if maxSource > lineFitMaxSourceCodeUnits {
		maxSource = lineFitMaxSourceCodeUnits
	}
	if len(raw) <= maxSource {
		return raw
	}

	var b strings.Builder
	b.Grow(maxSource)
	cells := 0
	i := 0
	for i < len(raw) && cells < safeWidth {
		if raw[i] == 0x1b {
			end := ansiSequenceEnd(raw, i)
			if end < 0 {
				break
			}
			if ansiSequenceHasVisiblePayload(raw, i) {
				seq := raw[i:end]
				if b.Len()+len(seq) <= maxSource {
					b.WriteString(seq)
					cells += ansitext.VisibleWidth(seq)
				}
			}
			i = end
			continue
		}
		_, size := utf8.DecodeRuneInString(raw[i:])
		if size <= 0 {
			break
		}
		char := raw[i : i+size]
		charW := ansitext.VisibleWidth(char)
		if charW > 0 && cells+charW > safeWidth {
			break
		}
		if b.Len()+len(char) > maxSource {
			if charW > 0 {
				break
			}
			i += size
			continue
		}
		if charW == 0 {
			remaining := safeWidth - cells
			reserved := remaining * 2
			if b.Len()+len(char) > maxSource-reserved {
				i += size
				continue
			}
		}
		b.WriteString(char)
		cells += charW
		i += size
	}
	b.WriteString(ansitext.SegmentReset)
	return b.String()
}

// ansiSequenceEnd returns the exclusive end index of an ESC sequence at start,
// or -1 if truncated/malformed.
func ansiSequenceEnd(line string, start int) int {
	if start+1 >= len(line) {
		return -1
	}
	next := line[start+1]
	switch next {
	case '[': // CSI
		i := start + 2
		for i < len(line) {
			final := line[i]
			if final >= 0x40 && final <= 0x7e {
				return i + 1
			}
			i++
		}
		return -1
	case ']': // OSC
		i := start + 2
		for i < len(line) {
			osc := line[i]
			if osc == 0x07 {
				return i + 1
			}
			if osc == 0x1b && i+1 < len(line) && line[i+1] == '\\' {
				return i + 2
			}
			i++
		}
		return -1
	default:
		if start+2 <= len(line) {
			return start + 2
		}
		return -1
	}
}

// ansiSequenceHasVisiblePayload is true for OSC 66 text-sizing spans.
func ansiSequenceHasVisiblePayload(line string, start int) bool {
	return start+4 < len(line) &&
		line[start+1] == ']' &&
		line[start+2] == '6' &&
		line[start+3] == '6' &&
		line[start+4] == ';'
}

// ansiASCIILineWidth returns the cell width when line is only printable ASCII
// plus well-formed CSI/OSC (non-66) escapes. ok=false means fall back to
// VisibleWidth (non-ASCII, OSC66, or malformed).
func ansiASCIILineWidth(line string, maxWidth int) (int, bool) {
	col := 0
	i := 0
	for i < len(line) {
		code := line[i]
		if code == 0x1b {
			if i+1 >= len(line) {
				return 0, false
			}
			next := line[i+1]
			switch next {
			case '[':
				j := i + 2
				for j < len(line) {
					final := line[j]
					if final >= 0x40 && final <= 0x7e {
						break
					}
					j++
				}
				if j >= len(line) {
					return 0, false
				}
				i = j + 1
				continue
			case ']':
				if i+4 < len(line) &&
					line[i+2] == '6' && line[i+3] == '6' && line[i+4] == ';' {
					return 0, false
				}
				j := i + 2
				for j < len(line) {
					osc := line[j]
					if osc == 0x07 {
						i = j + 1
						break
					}
					if osc == 0x1b && j+1 < len(line) && line[j+1] == '\\' {
						i = j + 2
						break
					}
					j++
				}
				if j >= len(line) {
					return 0, false
				}
				continue
			default:
				return 0, false
			}
		}
		if code < 0x20 || code > 0x7e {
			return 0, false
		}
		col++
		if col > maxWidth {
			return col, true
		}
		i++
	}
	return col, true
}
