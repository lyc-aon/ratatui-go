package ledger

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SGR CSI sequences of the form ESC [ params m, matching OMP's SGR_SEQUENCE.
// Params may include digits, ';' and ':' (colon-form extended colors).
func stripSGR(s string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c == 'm' {
					j++
					i = j
					break
				}
				if (c >= '0' && c <= '9') || c == ';' || c == ':' {
					j++
					continue
				}
				// Not a well-formed SGR; emit the ESC and continue.
				b.WriteByte(s[i])
				i++
				break
			}
			if j >= len(s) && i < len(s) && s[i] == 0x1b {
				// Truncated CSI — emit remaining literally.
				b.WriteString(s[i:])
				return b.String()
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// RowsEquivalent reports whether two rows match ignoring SGR styling.
// Theme restyles keep alignment; pointer/byte identity short-circuits.
//
// Mirrors tui.ts rowsEquivalent.
func RowsEquivalent(a, b string) bool {
	if a == b {
		return true
	}
	return stripSGR(a) == stripSGR(b)
}

// IsBlankRow reports whether a row is empty after stripping SGR and trimming
// Unicode spaces. Mirrors tui.ts isBlankRow.
func IsBlankRow(row string) bool {
	if len(row) == 0 {
		return true
	}
	return strings.TrimFunc(stripSGR(row), unicode.IsSpace) == ""
}
