package attachments

import (
	"strings"
)

// mentionBoundary chars match OMP MENTION_BOUNDARY_REGEX = /[\s([{<"'`]/
func isMentionBoundary(text string, index int) bool {
	if index <= 0 {
		return index == 0
	}
	switch text[index-1] {
	case ' ', '\t', '\n', '\r', '\f', '\v',
		'(', '[', '{', '<', '"', '\'', '`':
		return true
	default:
		return false
	}
}

// leading / trailing punctuation stripped from unquoted mentions (OMP).
const (
	leadingPunct  = "`\"'([{<"
	trailingPunct = ")]}>.,;:!?\"'`"
)

func sanitizeMentionPath(raw string) string {
	cleaned := strings.TrimSpace(raw)
	for len(cleaned) > 0 && strings.IndexByte(leadingPunct, cleaned[0]) >= 0 {
		cleaned = cleaned[1:]
	}
	for len(cleaned) > 0 && strings.IndexByte(trailingPunct, cleaned[len(cleaned)-1]) >= 0 {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return strings.TrimSpace(cleaned)
}

// mention is one @path span in the source text.
type mention struct {
	// Raw is the path inside the mention (no @, no quotes).
	Raw string
	// Start is the byte index of '@' (or of '\' when escaped \@).
	Start int
	// End is the exclusive end byte index of the whole token including @/quotes.
	End int
	// Escaped is true for \@ (literal @; not a file mention).
	Escaped bool
	// Quoted is true when path was inside "..." or '...'.
	Quoted bool
}

// scanMentions finds \@ escapes and @path / @"path" / @'path' mentions.
// Order is left-to-right; overlapping is impossible by construction.
func scanMentions(text string) []mention {
	var out []mention
	i := 0
	for i < len(text) {
		// Escaped literal @: \@
		if text[i] == '\\' && i+1 < len(text) && text[i+1] == '@' {
			out = append(out, mention{
				Raw:     "@",
				Start:   i,
				End:     i + 2,
				Escaped: true,
			})
			i += 2
			continue
		}
		if text[i] != '@' {
			i++
			continue
		}
		if !isMentionBoundary(text, i) {
			i++
			continue
		}
		// @"path with spaces" or @'path'
		if i+1 < len(text) && (text[i+1] == '"' || text[i+1] == '\'') {
			q := text[i+1]
			j := i + 2
			for j < len(text) && text[j] != q {
				j++
			}
			if j >= len(text) {
				// unclosed quote: treat rest as path (best-effort)
				raw := strings.TrimSpace(text[i+2:])
				if raw != "" {
					out = append(out, mention{Raw: raw, Start: i, End: len(text), Quoted: true})
				}
				break
			}
			raw := strings.TrimSpace(text[i+2 : j])
			if raw != "" {
				out = append(out, mention{Raw: raw, Start: i, End: j + 1, Quoted: true})
			}
			i = j + 1
			continue
		}
		// unquoted: @[^\s@]+  (OMP FILE_MENTION_REGEX third alt)
		j := i + 1
		for j < len(text) {
			c := text[j]
			if c == '@' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v' {
				break
			}
			j++
		}
		if j == i+1 {
			// lone @
			i++
			continue
		}
		raw := text[i+1 : j]
		cleaned := sanitizeMentionPath(raw)
		if cleaned == "" {
			i = j
			continue
		}
		// If sanitize stripped trailing/leading punct, shrink End so punct stays in prompt.
		end := j
		tmp := raw
		lead := 0
		for len(tmp) > 0 && strings.IndexByte(leadingPunct, tmp[0]) >= 0 {
			tmp = tmp[1:]
			lead++
		}
		if strings.HasPrefix(tmp, cleaned) {
			end = i + 1 + lead + len(cleaned)
		}
		out = append(out, mention{Raw: cleaned, Start: i, End: end, Quoted: false})
		i = j
	}
	return out
}

// rewritePrompt removes successful mention spans and turns \@ into @.
// remove[i] true means mention i should be deleted from the prompt text.
func rewritePrompt(text string, mentions []mention, remove []bool) string {
	if len(mentions) == 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	cursor := 0
	for i, m := range mentions {
		if m.Start < cursor {
			continue
		}
		b.WriteString(text[cursor:m.Start])
		if m.Escaped {
			b.WriteByte('@')
			cursor = m.End
			continue
		}
		if i < len(remove) && remove[i] {
			cursor = m.End
			continue
		}
		// keep original token text
		b.WriteString(text[m.Start:m.End])
		cursor = m.End
	}
	b.WriteString(text[cursor:])
	return collapseDetachedSpaces(b.String())
}

// collapseDetachedSpaces turns runs of ASCII spaces created by token removal
// into a single space, without touching newlines or tabs, then trims edges.
func collapseDetachedSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	spaceRun := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			spaceRun++
			if spaceRun == 1 {
				b.WriteByte(' ')
			}
			continue
		}
		spaceRun = 0
		b.WriteByte(s[i])
	}
	return strings.TrimSpace(b.String())
}
