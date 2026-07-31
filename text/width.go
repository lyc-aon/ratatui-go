package text

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Halfwidth katakana sound marks: unicode-width treats these as zero-width
// (Grapheme_Extend) but terminals typically render them as one cell each.
const (
	halfwidthKatakanaVoicedSoundMark     = '\uFF9E'
	halfwidthKatakanaSemiVoicedSoundMark = '\uFF9F'
)

// GraphemeWidth returns the terminal cell width of s.
//
// Uses uniseg grapheme cluster iteration and StringWidth, then compensates for
// halfwidth katakana dakuten/handakuten (U+FF9E / U+FF9F) the way Ratatui does.
// Never splits grapheme clusters. Returns 0 for empty input.
func GraphemeWidth(s string) int {
	if s == "" {
		return 0
	}
	// Fast path: single ASCII byte.
	if len(s) == 1 && s[0] < utf8.RuneSelf {
		if s[0] < 0x20 || s[0] == 0x7f {
			// Control chars: callers usually filter these; width 0 keeps layout sane.
			return 0
		}
		return 1
	}
	w := uniseg.StringWidth(s)
	if w < 0 {
		w = 0
	}
	return w + countHalfwidthSoundMarks(s)
}

func countHalfwidthSoundMarks(s string) int {
	n := 0
	for _, r := range s {
		if r == halfwidthKatakanaVoicedSoundMark || r == halfwidthKatakanaSemiVoicedSoundMark {
			n++
		}
	}
	return n
}

// Graphemes returns the grapheme clusters of s, excluding clusters that contain
// a control character (matching Span::styled_graphemes filtering).
func Graphemes(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		if graphemeHasControl(g) {
			continue
		}
		out = append(out, g)
	}
	return out
}

func graphemeHasControl(g string) bool {
	for _, r := range g {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// Truncate trims s from the end so its terminal width is at most maxWidth.
// Returns the truncated string and its actual width (may be < maxWidth when a
// wide grapheme would overflow).
//
// If maxWidth <= 0, returns ("", 0). Does not split grapheme clusters.
func Truncate(s string, maxWidth int) (string, int) {
	if maxWidth <= 0 || s == "" {
		return "", 0
	}
	if GraphemeWidth(s) <= maxWidth {
		return s, GraphemeWidth(s)
	}
	var b strings.Builder
	width := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		g := gr.Str()
		if graphemeHasControl(g) {
			continue
		}
		gw := GraphemeWidth(g)
		if width+gw > maxWidth {
			break
		}
		b.WriteString(g)
		width += gw
	}
	return b.String(), width
}

// TruncateStart trims s from the start so its terminal width is at most maxWidth.
// Returns the remaining suffix and its actual width.
//
// Mirrors unicode_truncate::unicode_truncate_start used by Line rendering when
// right/center alignment clips the left side. Does not split grapheme clusters.
func TruncateStart(s string, maxWidth int) (string, int) {
	if maxWidth <= 0 || s == "" {
		return "", 0
	}
	total := GraphemeWidth(s)
	if total <= maxWidth {
		return s, total
	}
	gs := Graphemes(s)
	width := 0
	start := len(gs)
	for i := len(gs) - 1; i >= 0; i-- {
		gw := GraphemeWidth(gs[i])
		if width+gw > maxWidth {
			break
		}
		width += gw
		start = i
	}
	if start >= len(gs) {
		return "", 0
	}
	return strings.Join(gs[start:], ""), width
}
