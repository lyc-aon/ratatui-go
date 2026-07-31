package text

import (
	"unicode"

	"github.com/lyc-aon/ratatui-go/style"
)

const (
	nbsp = "\u00a0"
	zwsp = "\u200b"
)

// StyledGrapheme is a grapheme cluster with an applied style.
//
// Not part of the Text -> Line -> Span hierarchy; used for render iteration.
type StyledGrapheme struct {
	Symbol string
	Style  style.Style
}

// NewStyledGrapheme builds a StyledGrapheme.
func NewStyledGrapheme(symbol string, sty style.Style) StyledGrapheme {
	return StyledGrapheme{Symbol: symbol, Style: sty}
}

// IsWhitespace reports whether the grapheme is whitespace for wrapping purposes.
// ZWSP counts as whitespace; NBSP does not.
func (g StyledGrapheme) IsWhitespace() bool {
	if g.Symbol == zwsp {
		return true
	}
	if g.Symbol == nbsp {
		return false
	}
	if g.Symbol == "" {
		return false
	}
	for _, r := range g.Symbol {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// Width returns the terminal cell width of the grapheme.
func (g StyledGrapheme) Width() int {
	return GraphemeWidth(g.Symbol)
}
