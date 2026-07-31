package interact

import (
	"strings"
	"unicode/utf8"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
)

// Style paints a string. Identity when nil.
type Style func(string) string

func applyStyle(fn Style, s string) string {
	if fn == nil {
		return s
	}
	return fn(s)
}

// padSpaces returns n ASCII spaces. n <= 0 yields "".
func padSpaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// replaceTabs expands tabs to ansitext.DefaultTabWidth spaces.
func replaceTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", ansitext.DefaultTabWidth))
}

// sanitizeSingleLine collapses newlines/tabs/runs of whitespace for list labels.
func sanitizeSingleLine(text string) string {
	text = replaceTabs(text)
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

// firstCellGlyph returns the first grapheme of value when it is exactly one
// cell wide; otherwise fallback.
func firstCellGlyph(value, fallback string) string {
	if value == "" {
		return fallback
	}
	gr := uniseg.NewGraphemes(value)
	if !gr.Next() {
		return fallback
	}
	g := gr.Str()
	if ansitext.VisibleWidth(g) == 1 {
		return g
	}
	return fallback
}

// applyBackground pads line to width cells then runs bg over the whole row.
func applyBackground(line string, width int, bg Style) string {
	vis := ansitext.VisibleWidth(line)
	pad := maxInt(0, width-vis)
	padded := line + padSpaces(pad)
	if bg == nil {
		return padded
	}
	return bg(padded)
}

// padToWidth right-pads plain/ANSI line to width cells with spaces.
func padToWidth(line string, width int) string {
	vis := ansitext.VisibleWidth(line)
	if vis >= width {
		return line
	}
	return line + padSpaces(width-vis)
}

// plainSlice extracts length columns starting at startCol from plain text
// (no ANSI). Wide graphemes are never split. strict drops a wide grapheme that
// would overflow the end boundary. Returns text and its visible width.
func plainSlice(s string, startCol, length int, strict bool) (string, int) {
	if length <= 0 || s == "" {
		return "", 0
	}
	if startCol < 0 {
		startCol = 0
	}
	endCol := startCol + length
	var b strings.Builder
	b.Grow(minInt(len(s), length*4+8))
	col := 0
	outW := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		gw := ansitext.VisibleWidth(g)
		if gw <= 0 {
			if col >= startCol && col < endCol {
				b.WriteString(g)
			}
			continue
		}
		next := col + gw
		if next <= startCol {
			col = next
			continue
		}
		if col >= endCol {
			break
		}
		if col < startCol {
			col = next
			continue
		}
		if strict && next > endCol {
			break
		}
		b.WriteString(g)
		outW += gw
		col = next
		if col >= endCol {
			break
		}
	}
	return b.String(), outW
}

// prevGraphemeByteLen returns the byte length of the grapheme ending at cursor.
func prevGraphemeByteLen(s string, cursor int) int {
	if cursor <= 0 {
		return 0
	}
	if cursor > len(s) {
		cursor = len(s)
	}
	before := s[:cursor]
	gr := uniseg.NewGraphemes(before)
	last := ""
	for gr.Next() {
		last = gr.Str()
	}
	if last == "" {
		_, size := utf8.DecodeLastRuneInString(before)
		return size
	}
	return len(last)
}

// nextGraphemeByteLen returns the byte length of the grapheme starting at cursor.
func nextGraphemeByteLen(s string, cursor int) int {
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(s) {
		return 0
	}
	after := s[cursor:]
	gr := uniseg.NewGraphemes(after)
	if !gr.Next() {
		_, size := utf8.DecodeRuneInString(after)
		return size
	}
	return len(gr.Str())
}

// dropLastGrapheme removes the last grapheme from s.
func dropLastGrapheme(s string) string {
	if s == "" {
		return s
	}
	n := prevGraphemeByteLen(s, len(s))
	if n <= 0 {
		return ""
	}
	return s[:len(s)-n]
}

// cleanPasteText applies single-line paste sanitization: drop CR/LF, expand
// tabs, strip remaining C0/DEL controls.
func cleanPasteText(pasted string) string {
	if pasted == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(pasted))
	for i := 0; i < len(pasted); i++ {
		c := pasted[i]
		if c == '\r' || c == '\n' {
			continue
		}
		b.WriteByte(c)
	}
	text := norm.NFC.String(strings.ToValidUTF8(replaceTabs(b.String()), ""))
	var out strings.Builder
	out.Grow(len(text))
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// invertCell wraps s in reverse-video SGR. Empty s becomes a space.
func invertCell(s string) string {
	if s == "" {
		s = " "
	}
	return "\x1b[7m" + s + "\x1b[27m"
}

// effectivePaddingX returns non-negative horizontal padding.
// ignoreTight keeps base as-is (overlay hosts set this). Without a global
// tight flag, base padding is used unchanged.
func effectivePaddingX(base int, ignoreTight bool) int {
	_ = ignoreTight
	return maxInt(0, base)
}

// sameLines reports identical string slices (length + element equality).
func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
