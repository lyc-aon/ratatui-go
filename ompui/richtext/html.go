package richtext

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	htmlEntityRe = regexp.MustCompile(`(?i)&(amp|lt|gt|quot|apos|nbsp|#\d+|#x[0-9a-fA-F]+);`)
	htmlTagRe    = regexp.MustCompile(`(?i)</?(?:br|p|ol|ul|li|span|text)\b(?:\s[^>]*)?\s*/?>`)
)

// normalizeHtmlEntitiesForTerminal expands the common named entities and
// numeric character references OMP handles. Unknown entities are left intact.
func normalizeHtmlEntitiesForTerminal(raw string) string {
	return htmlEntityRe.ReplaceAllStringFunc(raw, func(match string) string {
		// strip & and ;
		inner := match[1 : len(match)-1]
		lower := strings.ToLower(inner)
		switch lower {
		case "nbsp":
			return " "
		case "lt":
			return "<"
		case "gt":
			return ">"
		case "quot":
			return `"`
		case "apos":
			return "'"
		case "amp":
			return "&"
		}
		if strings.HasPrefix(lower, "#x") {
			v, err := strconv.ParseInt(lower[2:], 16, 32)
			if err != nil {
				return match
			}
			return codePointString(v)
		}
		if strings.HasPrefix(lower, "#") {
			v, err := strconv.ParseInt(lower[1:], 10, 32)
			if err != nil {
				return match
			}
			return codePointString(v)
		}
		return match
	})
}

func codePointString(v int64) string {
	if v < 0 || v > 0x10ffff {
		return ""
	}
	r := rune(v)
	if !utf8.ValidRune(r) {
		return ""
	}
	return string(r)
}

type htmlListState struct {
	kind string // "ol" | "ul"
	next int
}

type htmlNormState struct {
	lists          []htmlListState
	openItems      []bool
	itemHasContent []bool
}

func newHTMLNormState() *htmlNormState {
	return &htmlNormState{}
}

func htmlTagName(tag string) string {
	// </? name
	t := strings.TrimSpace(tag)
	t = strings.TrimPrefix(t, "<")
	t = strings.TrimPrefix(t, "/")
	t = strings.TrimSpace(t)
	end := 0
	for end < len(t) {
		c := t[end]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == ':' || c == '-' {
			end++
			continue
		}
		break
	}
	return strings.ToLower(t[:end])
}

func htmlOlStart(tag string) int {
	re := regexp.MustCompile(`(?i)\bstart\s*=\s*(?:"(\d+)"|'(\d+)'|(\d+))`)
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return 1
	}
	for _, g := range m[1:] {
		if g != "" {
			n, err := strconv.Atoi(g)
			if err == nil {
				return n
			}
		}
	}
	return 1
}

func appendHTMLLineBreak(output string, force bool) string {
	trimmed := strings.TrimRight(output, " \t")
	if !force && strings.HasSuffix(trimmed, "\n") {
		return trimmed
	}
	return trimmed + "\n"
}

func htmlListIndent(state *htmlNormState) string {
	n := len(state.lists) - 1
	if n < 0 {
		n = 0
	}
	return strings.Repeat("  ", n)
}

func appendHTMLListBreak(output string, state *htmlNormState) string {
	indent := htmlListIndent(state)
	if strings.HasSuffix(output, indent+"\n") {
		return output
	}
	return appendHTMLLineBreak(output, false)
}

func markCurrentHTMLItemContent(state *htmlNormState, text string) {
	if strings.TrimSpace(text) != "" && len(state.itemHasContent) > 0 {
		state.itemHasContent[len(state.itemHasContent)-1] = true
	}
}

func isAtEmptyHTMLListItem(state *htmlNormState) bool {
	if len(state.itemHasContent) == 0 {
		return false
	}
	i := len(state.itemHasContent) - 1
	return state.openItems[i] && !state.itemHasContent[i]
}

// normalizeHtmlForTerminal converts a small HTML subset used in model output
// into plain terminal text (lists, br, p). Unknown tags pass through.
func normalizeHtmlForTerminal(raw string, state *htmlNormState) string {
	if state == nil {
		state = newHTMLNormState()
	}
	var output strings.Builder
	last := 0
	matches := htmlTagRe.FindAllStringIndex(raw, -1)
	for _, loc := range matches {
		tag := raw[loc[0]:loc[1]]
		textBefore := normalizeHtmlEntitiesForTerminal(raw[last:loc[0]])
		name := htmlTagName(tag)
		isInline := name == "span" || name == "text"
		if isInline || strings.TrimSpace(textBefore) != "" {
			output.WriteString(textBefore)
			markCurrentHTMLItemContent(state, textBefore)
		}
		last = loc[1]

		isClosing := strings.HasPrefix(strings.TrimSpace(tag), "</")
		isSelfClosing := strings.HasSuffix(strings.TrimSpace(tag), "/>") || name == "br"

		switch name {
		case "span", "text":
			// drop tag
		case "br":
			s := appendHTMLLineBreak(output.String(), true)
			output.Reset()
			output.WriteString(s)
		case "p":
			if isClosing {
				s := appendHTMLLineBreak(output.String(), false)
				output.Reset()
				output.WriteString(s)
			} else {
				cur := output.String()
				if strings.TrimSpace(cur) != "" && !strings.HasSuffix(cur, "\n") && !isAtEmptyHTMLListItem(state) {
					s := appendHTMLLineBreak(cur, false)
					output.Reset()
					output.WriteString(s)
				}
			}
		case "ol":
			if isClosing {
				if len(state.lists) > 0 {
					state.lists = state.lists[:len(state.lists)-1]
				}
				if len(state.openItems) > 0 {
					state.openItems = state.openItems[:len(state.openItems)-1]
				}
				if len(state.itemHasContent) > 0 {
					state.itemHasContent = state.itemHasContent[:len(state.itemHasContent)-1]
				}
			} else if !isSelfClosing {
				if len(state.openItems) > 0 && state.openItems[len(state.openItems)-1] {
					s := appendHTMLListBreak(output.String(), state)
					output.Reset()
					output.WriteString(s)
				}
				state.lists = append(state.lists, htmlListState{kind: "ol", next: htmlOlStart(tag)})
				state.openItems = append(state.openItems, false)
				state.itemHasContent = append(state.itemHasContent, false)
			}
		case "ul":
			if isClosing {
				if len(state.lists) > 0 {
					state.lists = state.lists[:len(state.lists)-1]
				}
				if len(state.openItems) > 0 {
					state.openItems = state.openItems[:len(state.openItems)-1]
				}
				if len(state.itemHasContent) > 0 {
					state.itemHasContent = state.itemHasContent[:len(state.itemHasContent)-1]
				}
			} else if !isSelfClosing {
				if len(state.openItems) > 0 && state.openItems[len(state.openItems)-1] {
					s := appendHTMLListBreak(output.String(), state)
					output.Reset()
					output.WriteString(s)
				}
				state.lists = append(state.lists, htmlListState{kind: "ul", next: 1})
				state.openItems = append(state.openItems, false)
				state.itemHasContent = append(state.itemHasContent, false)
			}
		case "li":
			if isClosing {
				s := appendHTMLLineBreak(output.String(), false)
				output.Reset()
				output.WriteString(s)
				break
			}
			if len(state.openItems) > 0 {
				idx := len(state.openItems) - 1
				if state.openItems[idx] {
					s := appendHTMLListBreak(output.String(), state)
					output.Reset()
					output.WriteString(s)
				}
				state.openItems[idx] = true
				state.itemHasContent[idx] = false
			} else {
				cur := output.String()
				if strings.TrimSpace(cur) != "" && !strings.HasSuffix(cur, "\n") {
					s := appendHTMLLineBreak(cur, false)
					output.Reset()
					output.WriteString(s)
				}
			}
			indent := htmlListIndent(state)
			var list *htmlListState
			if len(state.lists) > 0 {
				list = &state.lists[len(state.lists)-1]
			}
			if list != nil && list.kind == "ol" {
				output.WriteString(indent)
				output.WriteString(itoa(list.next))
				output.WriteString(". ")
				list.next++
			} else {
				output.WriteString(indent)
				output.WriteString("• ")
			}
		default:
			output.WriteString(tag)
		}
	}
	remaining := normalizeHtmlEntitiesForTerminal(raw[last:])
	markCurrentHTMLItemContent(state, remaining)
	output.WriteString(remaining)
	return output.String()
}
