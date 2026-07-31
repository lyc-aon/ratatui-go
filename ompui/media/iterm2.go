package media

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// ITerm2Options configures OSC 1337 File= inline image emission.
type ITerm2Options struct {
	// Width is cells (int) or a literal like "auto". Zero / nil omits.
	Width any
	// Height is cells (int) or a literal like "auto". Zero / nil omits.
	Height any
	// Name is an optional filename; control bytes are stripped. Empty omits.
	Name string
	// PreserveAspectRatio defaults true when unset (pointer nil).
	PreserveAspectRatio *bool
	// Inline defaults true when unset.
	Inline *bool
}

// EncodeITerm2 builds OSC 1337 File=… BEL. base64Data is appended as-is
// (no re-encode). Name is sanitized; RejectUnsafeName callers may pre-check.
func EncodeITerm2(base64Data string, opt ITerm2Options) string {
	inline := 1
	if opt.Inline != nil && !*opt.Inline {
		inline = 0
	}
	var b strings.Builder
	// OSC 1337 overhead + params + data
	b.Grow(64 + len(base64Data) + len(opt.Name))
	b.WriteString("\x1b]1337;File=inline=")
	b.WriteByte(byte('0' + inline))

	if opt.Width != nil {
		if s := formatITermDim(opt.Width); s != "" {
			b.WriteString(";width=")
			b.WriteString(s)
		}
	}
	if opt.Height != nil {
		if s := formatITermDim(opt.Height); s != "" {
			b.WriteString(";height=")
			b.WriteString(s)
		}
	}
	if opt.Name != "" {
		name := SanitizeName(opt.Name)
		if name != "" {
			b.WriteString(";name=")
			b.WriteString(base64.StdEncoding.EncodeToString([]byte(name)))
		}
	}
	if opt.PreserveAspectRatio != nil && !*opt.PreserveAspectRatio {
		b.WriteString(";preserveAspectRatio=0")
	}
	b.WriteByte(':')
	b.WriteString(base64Data)
	b.WriteByte('\x07')
	return b.String()
}

func formatITermDim(v any) string {
	switch t := v.(type) {
	case int:
		if t == 0 {
			return ""
		}
		return strconv.Itoa(t)
	case int64:
		if t == 0 {
			return ""
		}
		return strconv.FormatInt(t, 10)
	case uint:
		if t == 0 {
			return ""
		}
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		if t == 0 {
			return ""
		}
		return strconv.FormatUint(uint64(t), 10)
	case string:
		return t
	default:
		return ""
	}
}
