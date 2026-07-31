package richtext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

const tabSpaces = "   " // ansitext.DefaultTabWidth == 3

func replaceTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	return strings.ReplaceAll(s, "\t", tabSpaces)
}

func padding(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func applyBackgroundToLine(line string, width int, bgFn func(string, int) string) string {
	if bgFn != nil {
		return bgFn(line, width)
	}
	vis := ansitext.VisibleWidth(line)
	pad := width - vis
	if pad < 0 {
		pad = 0
	}
	return line + padding(pad)
}

func padLineToWidth(line string, width int) string {
	vis := ansitext.VisibleWidth(line)
	pad := width - vis
	if pad <= 0 {
		return line
	}
	return line + padding(pad)
}

func splitTerminalLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	for len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimRightSpaceRunes(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t'
	})
}

// underline wraps s in SGR underline on/off. Used for links/H1 when Theme has
// no separate Underline hook.
func underline(s string) string {
	if s == "" {
		return s
	}
	return "\x1b[4m" + s + "\x1b[24m"
}

func formatHyperlink(text, target string, enabled bool) string {
	if !enabled || target == "" {
		return text
	}
	safe := strings.ReplaceAll(target, "\x1b", "")
	safe = strings.ReplaceAll(safe, "\x07", "")
	if safe == "" {
		return text
	}
	return "\x1b]8;;" + safe + "\x07" + text + "\x1b]8;;\x07"
}

func isOsc66Line(line string) bool {
	return strings.Contains(line, "\x1b]66;")
}

func encodeTextSized(text string, scale int, widthCells int) string {
	// Strip BEL/ST-breaking bytes from payload.
	var b strings.Builder
	b.Grow(len(text) + 24)
	b.WriteString("\x1b]66;")
	meta := false
	if scale > 0 {
		b.WriteByte('s')
		b.WriteByte('=')
		b.WriteByte(byte('0' + scale))
		meta = true
	}
	if widthCells >= 0 {
		if meta {
			b.WriteByte(':')
		}
		b.WriteByte('w')
		b.WriteByte('=')
		b.WriteString(itoa(widthCells))
	}
	b.WriteByte(';')
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '\x1b' || r == '\x07' || r == '\x00' {
			b.WriteByte(' ')
		} else {
			b.WriteString(text[i : i+size])
		}
		i += size
	}
	b.WriteString("\x1b\\")
	return b.String()
}

func encodeTextSizedHeading(text string, scale int) string {
	var out strings.Builder
	var asciiRun strings.Builder
	flush := func() {
		if asciiRun.Len() == 0 {
			return
		}
		out.WriteString(encodeTextSized(asciiRun.String(), scale, -1))
		asciiRun.Reset()
	}
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		seg := text[i : i+size]
		if r >= 0x20 && r <= 0x7e {
			asciiRun.WriteString(seg)
		} else {
			flush()
			w := ansitext.VisibleWidth(seg)
			out.WriteString(encodeTextSized(seg, scale, w))
		}
		i += size
	}
	flush()
	return out.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func getHrFillChar(sourceChar rune, themeHR string) string {
	isASCII := themeHR == "-"
	switch sourceChar {
	case '=':
		return "="
	case '═':
		if isASCII {
			return "="
		}
		return "═"
	case '━':
		if isASCII {
			return "-"
		}
		return "━"
	case '─':
		if isASCII {
			return "-"
		}
		return "─"
	case '–':
		if isASCII {
			return "-"
		}
		return "–"
	case '—':
		if isASCII {
			return "-"
		}
		return "—"
	default:
		return themeHR
	}
}

// longestWordWidth returns the widest whitespace-delimited token's visible width,
// capped at maxWidth when maxWidth > 0.
func longestWordWidth(text string, maxWidth int) int {
	longest := 0
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		w := ansitext.VisibleWidth(text[start:end])
		if w > longest {
			longest = w
		}
		start = -1
	}
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if unicode.IsSpace(r) {
			flush(i)
		} else if start < 0 {
			start = i
		}
		i += size
	}
	flush(len(text))
	if maxWidth > 0 && longest > maxWidth {
		return maxWidth
	}
	if longest < 1 {
		return 1
	}
	return longest
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
