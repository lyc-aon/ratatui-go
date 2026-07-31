// Package latex is the pure-Go OMP math engine: LaTeX fragments to Unicode
// (inline) and multi-line Box layout (display), plus prose scanners and
// pandoc-style anti-currency $…$ delimiter rules.
//
// Public entry points:
//
//	ToUnicode(src) string
//	ToBlock(src) []string
//	InlineMathSpanEnd(text, open) int
//	IsBareMathEnvironment(env) bool
//	RenderMathInText(text) string
//
// Malformed or unclosed input degrades to readable source text and never panics.
package latex
