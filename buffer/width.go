package buffer

import (
	"unicode"

	"github.com/lyc-aon/ratatui-go/text"
	"github.com/rivo/uniseg"
)

// StringWidth returns the terminal display width of s in cells.
// Delegates to text.GraphemeWidth (uniseg + halfwidth dakuten compensation).
func StringWidth(s string) int {
	return text.GraphemeWidth(s)
}

// containsControl reports whether s contains any Unicode control character.
func containsControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// firstGrapheme returns the first grapheme cluster of s and the remainder.
// ok is false when s is empty.
func firstGrapheme(s string) (cluster, rest string, ok bool) {
	if s == "" {
		return "", "", false
	}
	cluster, rest, _, _ = uniseg.FirstGraphemeClusterInString(s, -1)
	return cluster, rest, true
}
