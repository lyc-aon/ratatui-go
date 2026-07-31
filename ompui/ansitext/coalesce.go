package ansitext

import "strings"

// CoalesceAdjacentSGR merges runs of byte-adjacent SGR sequences
// (CSI [0-9;:]* m) into one CSI. Only SGR is touched; text, cursor moves, OSC,
// hyperlinks and image payloads pass through verbatim.
//
// Rules matching OMP coalesceAdjacentSgr:
//   - Empty params (CSI m) normalize to "0" when merged.
//   - A group flushes before appending the next list when the previous list
//     ended mid extended-color (38/48/58;2 with <3 channels, or ;5 with no
//     index) so the next code cannot be absorbed as a missing channel.
//   - Each emitted CSI is capped at 16 parameter tokens.
//   - Returns the original string when nothing merges (single index scan).
func CoalesceAdjacentSGR(line string) string {
	if line == "" || !strings.Contains(line, "\x1b[") {
		return line
	}
	n := len(line)
	var out strings.Builder
	// Delay allocation until a real merge is confirmed.
	copiedUpto := 0
	i := 0
	merged := false

	for i < n {
		if line[i] != esc || i+1 >= n || line[i+1] != '[' {
			i++
			continue
		}
		// Scan candidate SGR: ESC [ params m
		j := i + 2
		for j < n && isSGRParamByte(line[j]) {
			j++
		}
		if j >= n || line[j] != 'm' {
			// Not SGR (cursor move etc.)
			i = j
			continue
		}
		// Collect adjacent SGR run.
		params := []string{line[i+2 : j]}
		k := j + 1
		for k < n && line[k] == esc && k+1 < n && line[k+1] == '[' {
			p := k + 2
			for p < n && isSGRParamByte(line[p]) {
				p++
			}
			if p >= n || line[p] != 'm' {
				break
			}
			params = append(params, line[k+2:p])
			k = p + 1
		}
		if len(params) > 1 {
			if !merged {
				out.Grow(n)
				merged = true
			}
			out.WriteString(line[copiedUpto:i])
			emitMergedSGR(&out, params)
			copiedUpto = k
		}
		i = k
	}
	if !merged {
		return line
	}
	out.WriteString(line[copiedUpto:])
	return out.String()
}

func emitMergedSGR(out *strings.Builder, params []string) {
	var group strings.Builder
	groupTokens := 0
	groupOpenSafe := true

	flush := func() {
		if group.Len() == 0 {
			return
		}
		out.WriteByte(esc)
		out.WriteByte('[')
		out.WriteString(group.String())
		out.WriteByte('m')
		group.Reset()
		groupTokens = 0
	}

	for _, raw := range params {
		norm := raw
		if len(norm) == 0 {
			norm = "0"
		}
		tk := countParamTokens(norm)
		if groupTokens > 0 && (!groupOpenSafe || groupTokens+tk > mergeTokenCap) {
			flush()
		}
		if group.Len() == 0 {
			group.WriteString(norm)
		} else {
			group.WriteByte(';')
			group.WriteString(norm)
		}
		groupTokens += tk
		groupOpenSafe = !endsWithIncompleteExtendedColor(norm)
	}
	flush()
}

func countParamTokens(norm string) int {
	tk := 1
	for i := range norm {
		c := norm[i]
		if c == ';' || c == ':' {
			tk++
		}
	}
	return tk
}

// endsWithIncompleteExtendedColor reports whether params (semicolon-separated)
// end mid extended-color introducer in the ambiguous semicolon form:
// 38/48/58;2 with fewer than three channel values, or 38/48/58;5 with no index.
// Colon form is self-delimiting and treated as complete.
func endsWithIncompleteExtendedColor(params string) bool {
	// Split on ';' only — colon tokens stay inside a single field and never
	// equal bare "38", so the scan treats colon form as complete.
	parts := strings.Split(params, ";")
	i := 0
	for i < len(parts) {
		tok := parts[i]
		if tok == "38" || tok == "48" || tok == "58" {
			if i+1 >= len(parts) {
				return true // introducer, no mode
			}
			mode := parts[i+1]
			switch mode {
			case "2":
				// need r,g,b → indices i+2,i+3,i+4 must exist
				if i+4 >= len(parts) {
					return true
				}
				i += 5
				continue
			case "5":
				if i+2 >= len(parts) {
					return true
				}
				i += 3
				continue
			}
		}
		i++
	}
	return false
}
