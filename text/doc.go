// Package text provides styled text primitives: Span, Line, Text, and Masked.
//
// Hierarchy: Text is a list of Line; Line is a list of Span; Span is a
// contiguous run of graphemes sharing one Style. Masked hides a string behind
// a repeated mask character (Unicode scalar count). Widths are terminal cell
// widths via uniseg (never rune counts). Grapheme clusters are never split.
//
// Depends only on layout and style.
package text
