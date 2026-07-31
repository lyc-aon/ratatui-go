package media

import "strings"

// controlByte reports whether b is a C0/C1 control or DEL unsafe inside OSC /
// APC metadata payloads (names, filenames).
func controlByte(b byte) bool {
	return b < 0x20 || b == 0x7f || (b >= 0x80 && b <= 0x9f)
}

// SanitizeName strips unsafe control bytes from a filename/metadata string.
// Empty after strip stays empty. Does not allocate when already clean.
func SanitizeName(name string) string {
	if name == "" {
		return ""
	}
	if strings.IndexFunc(name, func(r rune) bool {
		if r > 0xff {
			return false
		}
		return controlByte(byte(r))
	}) < 0 {
		return name
	}
	var b strings.Builder
	b.Grow(len(name))
	for i := range len(name) {
		c := name[i]
		if controlByte(c) {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// RejectUnsafeName reports whether name contains any control byte that must
// never appear in protocol metadata. Callers that refuse rather than strip
// should use this.
func RejectUnsafeName(name string) bool {
	for i := range len(name) {
		if controlByte(name[i]) {
			return true
		}
	}
	return false
}
