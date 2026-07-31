package ansitext

import (
	"unicode"

	"github.com/lyc-aon/ratatui-go/text"
	"github.com/rivo/uniseg"
)

// VisibleWidth returns the terminal cell width of s, excluding ANSI/OSC escape
// sequences (except OSC 66 text-sizing spans, which contribute
// scale × (explicit w or payload width)). Tabs count as DefaultTabWidth cells.
// Ambiguous-width East Asian characters are narrow (1), matching OMP /
// unicode-width non-CJK tables via the text package grapheme width.
//
// Plain printable-ASCII inputs take an allocation-free fast path.
func VisibleWidth(s string) int {
	if s == "" {
		return 0
	}

	// Fast path: entire string is printable ASCII / tabs — no ESC, no high bytes.
	if w, ok := asciiVisibleWidth(s); ok {
		return w
	}

	width := 0
	i := 0
	n := len(s)
	for i < n {
		if s[i] == esc {
			if seqLen := ansiSeqLen(s, i); seqLen > 0 {
				seq := s[i : i+seqLen]
				if info, ok := osc66Info(seq); ok {
					width += info.width
				}
				i += seqLen
				continue
			}
			// Malformed ESC: skip one byte (zero width), matching natives.
			i++
			continue
		}

		// Collect a non-escape run.
		start := i
		ascii := true
		for i < n && s[i] != esc {
			if s[i] > 0x7f {
				ascii = false
			}
			i++
		}
		seg := s[start:i]
		if ascii {
			width += asciiRunWidth(seg)
		} else {
			width += plainVisibleWidth(seg)
		}
	}
	return width
}

// asciiVisibleWidth returns (width, true) when s is only tabs and printable
// ASCII (no ESC, no non-printable controls). Other inputs fall through.
func asciiVisibleWidth(s string) (int, bool) {
	tabs := 0
	for i := range s {
		c := s[i]
		if c == '\t' {
			tabs++
			continue
		}
		if c < 0x20 || c > 0x7e {
			return 0, false
		}
	}
	// printable count = len - tabs; each tab adds DefaultTabWidth instead of 1
	// so width = len + tabs*(DefaultTabWidth-1)
	if tabs == 0 {
		return len(s), true
	}
	return len(s) + tabs*(DefaultTabWidth-1), true
}

func asciiRunWidth(s string) int {
	w := 0
	for i := range s {
		w += asciiCellWidth(s[i])
	}
	return w
}

func asciiCellWidth(b byte) int {
	switch {
	case b == '\t':
		return DefaultTabWidth
	case b >= 0x20 && b <= 0x7e:
		return 1
	default:
		return 0
	}
}

// plainVisibleWidth measures a string with no ANSI escapes (may contain tabs
// and non-ASCII). Uses grapheme clustering for non-ASCII runs.
func plainVisibleWidth(s string) int {
	if s == "" {
		return 0
	}
	if w, ok := asciiVisibleWidth(s); ok {
		return w
	}
	width := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		width += graphemeCellWidth(g)
	}
	return width
}

// graphemeCellWidth is the terminal width of one grapheme cluster.
// Tabs are DefaultTabWidth; other controls are 0; otherwise text.GraphemeWidth.
func graphemeCellWidth(g string) int {
	if g == "\t" {
		return DefaultTabWidth
	}
	if g == "" {
		return 0
	}
	// Single ASCII byte.
	if len(g) == 1 {
		return asciiCellWidth(g[0])
	}
	// Control-only grapheme → 0 (uniseg may still group controls).
	if isControlGrapheme(g) {
		return 0
	}
	w := text.GraphemeWidth(g)
	if w < 0 {
		return 0
	}
	return w
}

func isControlGrapheme(g string) bool {
	for _, r := range g {
		if r == '\t' {
			return false
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// walkVisible calls fn for each visible grapheme in a plain (no-ANSI) string
// with its cell width. fn returns false to stop.
func walkVisible(s string, fn func(g string, w int) bool) {
	if s == "" {
		return
	}
	// ASCII fast path: per-byte, no grapheme scanner allocation.
	ascii := true
	for i := range s {
		if s[i] > 0x7f {
			ascii = false
			break
		}
	}
	if ascii {
		for i := range s {
			b := s[i]
			w := asciiCellWidth(b)
			if w == 0 {
				continue
			}
			if !fn(s[i:i+1], w) {
				return
			}
		}
		return
	}
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		w := graphemeCellWidth(g)
		if w == 0 && isControlGrapheme(g) {
			continue
		}
		if !fn(g, w) {
			return
		}
	}
}
