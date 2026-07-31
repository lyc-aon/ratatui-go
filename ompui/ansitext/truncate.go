package ansitext

import "strings"

// TruncateToWidth shortens s so its visible width is at most maxWidth, preserving
// ANSI/OSC sequences on the kept prefix. ellipsis is appended after a trailing
// SegmentReset when any SGR was copied into the output (so styles do not bleed
// into the ellipsis or following cells).
//
// When s already fits, the original string is returned unchanged (same reference
// semantics as OMP for the no-truncation path). maxWidth <= 0 yields "".
//
// Wide graphemes and full OSC 66 spans that would overflow the content budget
// (maxWidth − VisibleWidth(ellipsis)) are omitted; partial OSC 66 falls back to
// plain payload cells. Grapheme clusters are never split.
//
// Pass ellipsis "" to omit (OMP Ellipsis.Omit); "…" for Unicode; "..." for ASCII.
func TruncateToWidth(s string, maxWidth int, ellipsis string) string {
	if maxWidth < 0 {
		maxWidth = 0
	}
	if s == "" {
		return ""
	}

	// Fast path matching OMP: every code unit is at most 3 cells, so
	// len*3 fitting means no truncation needed. (OMP uses UTF-16 unit*3;
	// byte*3 is a conservative equivalent upper bound for BMP+emoji.)
	if ellipsis == "" && len(s)*3 <= maxWidth {
		return s
	}

	textW := VisibleWidth(s)
	if textW <= maxWidth {
		return s
	}

	ellipsisW := VisibleWidth(ellipsis)
	targetW := maxWidth - ellipsisW
	if targetW <= 0 {
		return fitEllipsis(ellipsis, maxWidth)
	}

	var b strings.Builder
	b.Grow(min(len(s), maxWidth*4+len(ellipsis)+8))
	w := 0
	i := 0
	n := len(s)
	sawSGR := false

	for i < n {
		if s[i] == esc {
			if seqLen := ansiSeqLen(s, i); seqLen > 0 {
				seq := s[i : i+seqLen]
				if info, ok := osc66Info(seq); ok {
					spanEnd := w + info.width
					if spanEnd <= targetW {
						b.WriteString(seq)
						w = spanEnd
						i += seqLen
						if w >= targetW {
							break
						}
						continue
					}
					// Partial: emit plain payload prefix only.
					if w < targetW {
						remaining := targetW - w
						pw := appendPlainRange(&b, info.payload, 0, remaining, true, func() {})
						w += pw
					}
					break
				}
				b.WriteString(seq)
				if isSGR(seq) {
					sawSGR = true
				}
				i += seqLen
				continue
			}
			b.WriteByte(esc)
			i++
			continue
		}

		start := i
		ascii := true
		for i < n && s[i] != esc {
			if s[i] > 0x7f {
				ascii = false
			}
			i++
		}
		seg := s[start:i]

		if ascii {
			stop := false
			for j := range seg {
				u := seg[j]
				gw := asciiCellWidth(u)
				if w+gw > targetW {
					stop = true
					break
				}
				if gw > 0 {
					b.WriteByte(u)
					w += gw
				}
			}
			if stop || w >= targetW {
				break
			}
		} else {
			stop := false
			walkVisible(seg, func(g string, gw int) bool {
				if w+gw > targetW {
					stop = true
					return false
				}
				b.WriteString(g)
				w += gw
				return true
			})
			if stop {
				break
			}
		}
	}

	if sawSGR {
		b.WriteString(SegmentReset)
	}
	b.WriteString(ellipsis)
	return b.String()
}

func fitEllipsis(ellipsis string, maxWidth int) string {
	if ellipsis == "" || maxWidth <= 0 {
		return ""
	}
	if VisibleWidth(ellipsis) <= maxWidth {
		return ellipsis
	}
	var b strings.Builder
	w := 0
	walkVisible(ellipsis, func(g string, gw int) bool {
		if w+gw > maxWidth {
			return false
		}
		b.WriteString(g)
		w += gw
		return true
	})
	return b.String()
}
