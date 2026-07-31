package ansitext

import "strings"

// Thai/Lao precomposed AM vowels and their compatibility decompositions.
// Some terminals repaint precomposed forms inconsistently; the decomposed
// sequences have the same cell width and avoid stale-cell artifacts.
const (
	thaiSaraAm     = '\u0e33' // SARA AM
	laoAm          = '\u0eb3' // AM
	thaiNikhahit   = "\u0e4d\u0e32"
	laoNikhahit    = "\u0ecd\u0eb2"
)

// NormalizeTerminalOutput rewrites precomposed Thai/Lao AM vowels to their
// compatibility decompositions for terminal paint. Logical editor content is
// not changed by callers that keep the source separate; this is an output-only
// transform matching OMP normalizeTerminalOutput.
//
// When s contains neither U+0E33 nor U+0EB3 the original string is returned.
func NormalizeTerminalOutput(s string) string {
	if s == "" {
		return s
	}
	if !strings.ContainsRune(s, thaiSaraAm) && !strings.ContainsRune(s, laoAm) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case thaiSaraAm:
			b.WriteString(thaiNikhahit)
		case laoAm:
			b.WriteString(laoNikhahit)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
