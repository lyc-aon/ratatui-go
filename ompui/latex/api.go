package latex

import (
	"container/list"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// ToUnicode converts a bare LaTeX math fragment (no surrounding $ / \(
// delimiters) to its best-effort Unicode rendering. Unknown commands degrade
// to their bare name; \\ becomes a newline. Always returns a string (never panics).
func ToUnicode(src string) string {
	if src == "" {
		return src
	}
	if v, ok := unicodeCache.get(src); ok {
		return v
	}
	out := newLatexParser(src).render()
	unicodeCache.put(src, out)
	return out
}

// IsBareMathEnvironment reports whether env is a math environment safe to
// auto-render without $ / \[ delimiters. The trailing * of starred variants is
// ignored; text-mode environments (tabular, itemize, …) return false.
func IsBareMathEnvironment(env string) bool {
	if strings.HasSuffix(env, "*") {
		env = env[:len(env)-1]
	}
	_, ok := bareMathEnvironments[env]
	return ok
}

// InlineMathSpanEnd returns the index of the $ that closes an inline math span
// opened at open (the index of the opening $), or -1 when the run is not inline
// math. Applies pandoc's anti-currency heuristics.
func InlineMathSpanEnd(text string, open int) int {
	if open < 0 || open >= len(text) || text[open] != '$' {
		return -1
	}
	if open+1 >= len(text) {
		return -1
	}
	after := text[open+1]
	if after == ' ' || after == '\t' || after == '\n' || after == '$' {
		return -1
	}
	for j := open + 1; j < len(text); j++ {
		ch := text[j]
		if ch == '\\' {
			j++ // skip next
			continue
		}
		if ch == '\n' {
			return -1
		}
		if ch == '$' {
			prev := text[j-1]
			if prev == ' ' || prev == '\t' {
				return -1
			}
			if j+1 < len(text) {
				next := text[j+1]
				if next >= '0' && next <= '9' {
					continue // currency: keep scanning
				}
			}
			if strings.TrimSpace(text[open+1:j]) == "" {
				return -1
			}
			return j
		}
	}
	return -1
}

var bareMathLineCommand = regexp.MustCompile(
	`\\(?:operatorname|frac|dfrac|tfrac|cfrac|genfrac|sqrt|sum|prod|coprod|int|iint|iiint|lim|alpha|beta|gamma|delta|epsilon|varepsilon|theta|lambda|mu|sigma|phi|varphi|pi|omega|infty|partial|nabla|forall|exists|mathbb|mathcal|mathscr|mathbf|mathrm|left|right|begin|phantom|hphantom|vphantom|cdots|ldots|dots|to|rightarrow|leftarrow|leq|geq|neq|times|cdot|overline|underline|vec|hat|bar|textcolor|color|normalcolor|colorbox|fcolorbox)\b`,
)

// RenderMathInText scans prose for math spans — $$…$$, \[…\] (display) and
// $…$, \(…\) (inline) — and replaces each with its Unicode rendering, leaving
// everything else verbatim. Also converts bare math environments and
// math-shaped lines. Newlines inside a span collapse to spaces.
func RenderMathInText(text string) string {
	if text == "" {
		return text
	}
	if !strings.Contains(text, "$") &&
		!strings.Contains(text, `\(`) &&
		!strings.Contains(text, `\[`) &&
		!strings.Contains(text, `\begin`) &&
		!bareMathLineCommand.MatchString(text) {
		return text
	}

	conv := func(inner string) string {
		return replaceNewlinesWithSpace(ToUnicode(inner))
	}
	var out strings.Builder
	i := 0
	n := len(text)
	for i < n {
		c := text[i]
		if c == '\\' {
			if i+1 >= n {
				out.WriteByte(c)
				i++
				continue
			}
			d := text[i+1]
			if d == '\\' {
				out.WriteString(`\\`)
				i += 2
				continue
			}
			if d == '(' {
				closeIdx := strings.Index(text[i+2:], `\)`)
				if closeIdx >= 0 {
					close := i + 2 + closeIdx
					out.WriteString(conv(text[i+2 : close]))
					i = close + 2
					continue
				}
			} else if d == '[' {
				closeIdx := strings.Index(text[i+2:], `\]`)
				if closeIdx >= 0 {
					close := i + 2 + closeIdx
					out.WriteString(conv(text[i+2 : close]))
					i = close + 2
					continue
				}
			} else if d == '$' {
				out.WriteByte('$')
				i += 2
				continue
			}
			out.WriteByte(c)
			i++
			continue
		}
		if c == '$' {
			if i+1 < n && text[i+1] == '$' {
				closeIdx := strings.Index(text[i+2:], "$$")
				if closeIdx >= 0 {
					close := i + 2 + closeIdx
					if strings.TrimSpace(text[i+2:close]) != "" {
						out.WriteString(conv(text[i+2 : close]))
						i = close + 2
						continue
					}
				}
				out.WriteString("$$")
				i += 2
				continue
			}
			close := InlineMathSpanEnd(text, i)
			if close != -1 {
				out.WriteString(conv(text[i+1 : close]))
				i = close + 1
				continue
			}
			out.WriteByte('$')
			i++
			continue
		}
		// copy one rune to preserve utf8
		_, size := utf8.DecodeRuneInString(text[i:])
		out.WriteString(text[i : i+size])
		i += size
	}
	return renderBareMathInText(out.String())
}

func renderBareMathInText(text string) string {
	var out strings.Builder
	i := 0
	for {
		begin := strings.Index(text[i:], `\begin{`)
		if begin < 0 {
			out.WriteString(renderBareMathLines(text[i:]))
			return out.String()
		}
		begin += i
		envStart := begin + len(`\begin{`)
		envEnd := strings.IndexByte(text[envStart:], '}')
		if envEnd < 0 {
			out.WriteString(renderBareMathLines(text[i:]))
			return out.String()
		}
		envEnd += envStart
		env := text[envStart:envEnd]
		closeToken := `\end{` + env + `}`
		closeRel := strings.Index(text[envEnd+1:], closeToken)
		if closeRel < 0 {
			// Unterminated \begin: convert lines up to env end, then rescan past it.
			out.WriteString(renderBareMathLines(text[i : envEnd+1]))
			i = envEnd + 1
			continue
		}
		close := envEnd + 1 + closeRel
		blockEnd := close + len(closeToken)
		if !IsBareMathEnvironment(env) {
			out.WriteString(renderBareMathLines(text[i:begin]))
			out.WriteString(text[begin:blockEnd])
			i = blockEnd
			continue
		}
		lineStart := strings.LastIndexByte(text[:begin], '\n') + 1
		prefix := text[lineStart:begin]
		start := begin
		if strings.Contains(prefix, `\`) || strings.Contains(prefix, "=") {
			start = lineStart
		}
		if start == begin && strings.TrimSpace(prefix) == "" && lineStart > 0 {
			previousLineEnd := lineStart - 1
			previousLineStart := 0
			if previousLineEnd > 0 {
				previousLineStart = strings.LastIndexByte(text[:previousLineEnd], '\n') + 1
			}
			previousLine := text[previousLineStart:previousLineEnd]
			if barePrevLinePull.MatchString(previousLine) {
				start = previousLineStart
			}
		}
		out.WriteString(renderBareMathLines(text[i:start]))
		out.WriteString(replaceNewlinesWithSpace(ToUnicode(text[start:blockEnd])))
		i = blockEnd
	}
}

var barePrevLinePull = regexp.MustCompile(`[=([{]\s*$`)

func renderBareMathLines(text string) string {
	var out strings.Builder
	lineStart := 0
	for i := 0; i <= len(text); i++ {
		if i != len(text) && text[i] != '\n' {
			continue
		}
		line := text[lineStart:i]
		if shouldRenderBareMathLine(line) {
			out.WriteString(replaceNewlinesWithSpace(ToUnicode(line)))
		} else {
			out.WriteString(line)
		}
		if i != len(text) {
			out.WriteByte('\n')
		}
		lineStart = i + 1
	}
	return out.String()
}

func shouldRenderBareMathLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.Contains(trimmed, `\`) {
		return false
	}
	if m := bareEnvLine.FindStringSubmatch(trimmed); m != nil {
		if !IsBareMathEnvironment(m[1]) {
			return false
		}
	}
	if !bareMathLineCommand.MatchString(trimmed) {
		return false
	}
	if strings.HasPrefix(trimmed, `\`) {
		return true
	}
	return bareMathLineShape.MatchString(trimmed)
}

var (
	bareEnvLine      = regexp.MustCompile(`\\(?:begin|end)\{([^}]*)\}`)
	bareMathLineShape = regexp.MustCompile(`[=<>^_{}&]`)
)

// ---------------------------------------------------------------------------
// Bounded count-only LRU for pure ToUnicode conversions.
// ---------------------------------------------------------------------------

const unicodeCacheCap = 256

type lruCache struct {
	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
	cap   int
}

type lruEntry struct {
	key, val string
}

func newLRU(cap int) *lruCache {
	return &lruCache{
		ll:    list.New(),
		items: make(map[string]*list.Element, cap),
		cap:   cap,
	}
}

func (c *lruCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*lruEntry).val, true
	}
	return "", false
}

func (c *lruCache) put(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*lruEntry).val = val
		return
	}
	el := c.ll.PushFront(&lruEntry{key: key, val: val})
	c.items[key] = el
	for c.ll.Len() > c.cap {
		back := c.ll.Back()
		if back == nil {
			break
		}
		c.ll.Remove(back)
		delete(c.items, back.Value.(*lruEntry).key)
	}
}

var unicodeCache = newLRU(unicodeCacheCap)
