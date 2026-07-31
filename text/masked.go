package text

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Masked wraps a string that is shown as a repeated mask character.
//
// Display and conversions use Unicode scalar counts (runes), matching Ratatui
// Masked::value (chars().map(|_| mask_char)), not grapheme or cell width.
type Masked struct {
	inner    string
	maskChar rune
}

// NewMasked builds a masked string. maskChar is the character repeated for each
// Unicode scalar in s when displayed.
func NewMasked(s string, maskChar rune) Masked {
	return Masked{inner: s, maskChar: maskChar}
}

// MaskChar returns the character used for masking.
func (m Masked) MaskChar() rune {
	return m.maskChar
}

// Value returns the displayed form: one mask character per Unicode scalar in
// the underlying string. The original content is not returned.
func (m Masked) Value() string {
	n := utf8.RuneCountInString(m.inner)
	if n == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(n * utf8.RuneLen(m.maskChar))
	for i := 0; i < n; i++ {
		b.WriteRune(m.maskChar)
	}
	return b.String()
}

// String returns the masked display form (Ratatui Display).
func (m Masked) String() string {
	return m.Value()
}

// GoString returns the underlying unmasked string (Ratatui Debug).
func (m Masked) GoString() string {
	return m.inner
}

// Underlying returns the original unmasked string.
func (m Masked) Underlying() string {
	return m.inner
}

// Span converts the masked value into an unstyled Span.
func (m Masked) Span() Span {
	return RawSpan(m.Value())
}

// Line converts the masked value into a single unstyled Line.
func (m Masked) Line() Line {
	return RawLine(m.Value())
}

// Text converts the masked value into unstyled Text (Ratatui From<Masked> for Text).
func (m Masked) Text() Text {
	return RawText(m.Value())
}

// Format supports fmt verbs. %v and %s use the masked display; %#v uses the
// underlying string (GoString).
func (m Masked) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('#') {
			_, _ = fmt.Fprint(f, m.inner)
			return
		}
		fallthrough
	case 's':
		_, _ = fmt.Fprint(f, m.Value())
	default:
		_, _ = fmt.Fprintf(f, "%%!%c(text.Masked)", verb)
	}
}
