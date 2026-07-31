package text

import (
	"fmt"

	"github.com/lyc-aon/ratatui-go/style"
)

// Spanf builds an unstyled Span from a fmt format string (ratatui-macros span!).
func Spanf(format string, args ...any) Span {
	return RawSpan(fmt.Sprintf(format, args...))
}

// StyledSpanf builds a styled Span from a fmt format string.
// Argument order matches StyledSpan: content first, then style.
func StyledSpanf(format string, sty style.Style, args ...any) Span {
	return StyledSpan(fmt.Sprintf(format, args...), sty)
}

// Linef builds an unstyled Line from a fmt format string.
func Linef(format string, args ...any) Line {
	return RawLine(fmt.Sprintf(format, args...))
}

// StyledLinef builds a styled Line from a fmt format string.
func StyledLinef(format string, sty style.Style, args ...any) Line {
	return StyledLine(fmt.Sprintf(format, args...), sty)
}

// Textf builds unstyled Text from a fmt format string.
func Textf(format string, args ...any) Text {
	return RawText(fmt.Sprintf(format, args...))
}

// StyledTextf builds styled Text from a fmt format string.
func StyledTextf(format string, sty style.Style, args ...any) Text {
	return StyledText(fmt.Sprintf(format, args...), sty)
}
