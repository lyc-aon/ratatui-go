package ansitext

import "strings"

// SliceByColumn extracts length visible columns starting at startCol from line.
// ANSI/OSC sequences are preserved so the returned substring keeps styling that
// applies inside the window. Wide graphemes are never split.
//
// Matches OMP sliceByColumn with strict=false: a wide grapheme that straddles
// the end boundary is kept (its full width is counted). OSC 66 spans that fully
// fit are kept atomic; partial overlaps expand to the underlying payload cells
// only (no partial wrappers). Trailing non-OSC66 escapes immediately after the
// window are included so a reset that follows kept text is not dropped.
//
// startCol < 0 is treated as 0. length <= 0 returns "".
func SliceByColumn(line string, startCol, length int) string {
	if length <= 0 || line == "" {
		return ""
	}
	if startCol < 0 {
		startCol = 0
	}
	out, _ := sliceByColumn(line, startCol, length, false)
	return out
}

func sliceByColumn(line string, startCol, length int, strict bool) (string, int) {
	endCol := startCol + length
	var b strings.Builder
	b.Grow(min(len(line), length*2+16))

	outW := 0
	currentCol := 0
	i := 0
	n := len(line)

	// Pending ANSI collected before the window opens; flushed on first kept cell.
	type pending struct{ start, length int }
	var pend []pending

	flushPend := func() {
		for _, p := range pend {
			b.WriteString(line[p.start : p.start+p.length])
		}
		pend = pend[:0]
	}

	for i < n && currentCol < endCol {
		if line[i] == esc {
			if seqLen := ansiSeqLen(line, i); seqLen > 0 {
				seq := line[i : i+seqLen]
				if info, ok := osc66Info(seq); ok {
					spanStart := currentCol
					spanEnd := currentCol + info.width
					if spanStart >= startCol && spanEnd <= endCol {
						flushPend()
						b.WriteString(seq)
						outW += info.width
					} else if spanStart < endCol && spanEnd > startCol {
						// Partial OSC 66: map visual overlap onto payload cells.
						overlapStart := max(0, startCol-spanStart)
						overlapEnd := min(spanEnd, endCol) - spanStart
						overlapLen := max(0, overlapEnd-overlapStart)
						pStart, pLen := osc66PayloadRange(overlapStart, overlapLen, info.scale, strict)
						pw := appendPlainRange(&b, info.payload, pStart, pLen, strict, flushPend)
						outW += pw
					}
					currentCol = spanEnd
					i += seqLen
					continue
				}
				if currentCol >= startCol {
					b.WriteString(seq)
				} else {
					pend = append(pend, pending{i, seqLen})
				}
				i += seqLen
				continue
			}
			// Malformed ESC.
			if currentCol >= startCol {
				b.WriteByte(esc)
			}
			i++
			continue
		}

		// Non-escape run.
		start := i
		ascii := true
		for i < n && line[i] != esc {
			if line[i] > 0x7f {
				ascii = false
			}
			i++
		}
		seg := line[start:i]

		if ascii {
			for j := range seg {
				if currentCol >= endCol {
					break
				}
				u := seg[j]
				gw := asciiCellWidth(u)
				inRange := currentCol >= startCol
				fits := !strict || currentCol+gw <= endCol
				if inRange && fits && gw > 0 {
					flushPend()
					b.WriteByte(u)
					outW += gw
				}
				currentCol += gw
			}
		} else {
			walkVisible(seg, func(g string, gw int) bool {
				if currentCol >= endCol {
					return false
				}
				inRange := currentCol >= startCol
				fits := !strict || currentCol+gw <= endCol
				if inRange && fits {
					flushPend()
					b.WriteString(g)
					outW += gw
				}
				currentCol += gw
				return currentCol < endCol
			})
		}
	}

	// Trailing ANSI (resets etc.) immediately after the window, not OSC 66.
	for i < n {
		if line[i] != esc {
			break
		}
		seqLen := ansiSeqLen(line, i)
		if seqLen == 0 {
			break
		}
		seq := line[i : i+seqLen]
		if _, ok := osc66Info(seq); ok {
			break
		}
		b.WriteString(seq)
		i += seqLen
	}

	return b.String(), outW
}

func osc66PayloadRange(visualStart, visualLen, scale int, strict bool) (start, length int) {
	if scale < 1 {
		scale = 1
	}
	visualEnd := visualStart + visualLen
	var payloadStart, payloadEnd int
	if strict {
		payloadStart = divCeil(visualStart, scale)
		payloadEnd = visualEnd / scale
	} else {
		payloadStart = visualStart / scale
		payloadEnd = divCeil(visualEnd, scale)
	}
	return payloadStart, max(0, payloadEnd-payloadStart)
}

func divCeil(n, d int) int {
	if n == 0 {
		return 0
	}
	return 1 + (n-1)/d
}

// appendPlainRange appends a plain (no ANSI) visible column range from data.
// beforeFirst is called once before the first kept cell is written.
func appendPlainRange(b *strings.Builder, data string, startCol, length int, strict bool, beforeFirst func()) int {
	if length <= 0 || data == "" {
		return 0
	}
	endCol := startCol + length
	outW := 0
	currentCol := 0
	wrote := false

	write := func(g string, gw int) {
		if !wrote {
			beforeFirst()
			wrote = true
		}
		b.WriteString(g)
		outW += gw
	}

	// Prefer ASCII scan when possible.
	ascii := true
	for i := range data {
		if data[i] > 0x7f {
			ascii = false
			break
		}
	}
	if ascii {
		for i := range data {
			if currentCol >= endCol {
				break
			}
			u := data[i]
			gw := asciiCellWidth(u)
			inRange := currentCol >= startCol
			fits := !strict || currentCol+gw <= endCol
			if inRange && fits && gw > 0 {
				write(data[i:i+1], gw)
			}
			currentCol += gw
		}
		return outW
	}

	walkVisible(data, func(g string, gw int) bool {
		if currentCol >= endCol {
			return false
		}
		inRange := currentCol >= startCol
		fits := !strict || currentCol+gw <= endCol
		if inRange && fits {
			write(g, gw)
		}
		currentCol += gw
		return currentCol < endCol
	})
	return outW
}
