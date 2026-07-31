// Package highlight is a pure Chroma-backed syntax highlighter for ompui
// richtext. It never writes to a terminal, executes processes, or touches
// global formatter flags.
//
// Pin: github.com/alecthomas/chroma/v2 v2.27.0
package highlight

import (
	"container/list"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// ColorMode selects the SGR color space used for token foregrounds.
type ColorMode int

const (
	// ColorNone emits plain text (no SGR).
	ColorNone ColorMode = iota
	// ColorANSI256 emits 256-color foregrounds (38;5;N).
	ColorANSI256
	// ColorTrueColor emits 24-bit foregrounds (38;2;R;G;B).
	ColorTrueColor
)

// Theme names a Chroma style. Empty uses DefaultTheme.
type Theme string

// DefaultTheme is the Chroma style used when none is configured.
const DefaultTheme Theme = "monokai"

const (
	// CacheSize is the bounded LRU capacity keyed by source+lang+theme+mode.
	CacheSize = 256
	// MaxSourceBytes caps input size to bound chroma/regexp2 cost.
	MaxSourceBytes = 256 << 10
	// MaxLineBytes caps a single line inside a source.
	MaxLineBytes = 16 << 10
	// MaxLines caps the number of output lines retained from a source.
	MaxLines = 4000
)

// Options configure a Highlighter.
type Options struct {
	// Theme is a Chroma style name (e.g. "monokai", "github", "dracula").
	Theme Theme
	// ColorMode selects truecolor / ansi256 / none.
	ColorMode ColorMode
	// MaxSourceBytes overrides the default source size bound (0 = default).
	MaxSourceBytes int
	// MaxLineBytes overrides the default per-line bound (0 = default).
	MaxLineBytes int
	// MaxLines overrides the default line-count bound (0 = default).
	MaxLines int
}

// Highlighter is a concurrency-safe syntax highlighter with lexer/style caches
// and a bounded source LRU.
type Highlighter struct {
	opts Options

	mu sync.Mutex

	// coalesced lexers by lowercase language key
	lexers map[string]chroma.Lexer
	// resolved token styles per theme name
	styleCache map[string]*styleTable

	cache map[string]*list.Element
	order *list.List
}

type styleTable struct {
	style *chroma.Style
	// memoized StyleEntry by TokenType
	entries map[chroma.TokenType]chroma.StyleEntry
	// pre-rendered SGR open prefixes by TokenType for current ColorMode
	sgr  map[chroma.TokenType]string
	mode ColorMode
}

type cacheEntry struct {
	key   string
	lines []string
}

// New builds a Highlighter. Options are copied; later mutations of the input
// struct do not affect the instance.
func New(opts Options) *Highlighter {
	if opts.Theme == "" {
		opts.Theme = DefaultTheme
	}
	if opts.MaxSourceBytes <= 0 {
		opts.MaxSourceBytes = MaxSourceBytes
	}
	if opts.MaxLineBytes <= 0 {
		opts.MaxLineBytes = MaxLineBytes
	}
	if opts.MaxLines <= 0 {
		opts.MaxLines = MaxLines
	}
	return &Highlighter{
		opts:       opts,
		lexers:     make(map[string]chroma.Lexer),
		styleCache: make(map[string]*styleTable),
		cache:      make(map[string]*list.Element, CacheSize),
		order:      list.New(),
	}
}

// HighlightCode matches richtext.Theme.HighlightCode: each returned string is
// one already-styled source line with no trailing newline. Unknown languages
// fall back to the plain lexer. Errors and panics degrade to plain lines.
func (h *Highlighter) HighlightCode(code, lang string) []string {
	if h == nil {
		return plainLines(code, MaxLineBytes, MaxLines)
	}
	defer func() { _ = recover() }()

	maxSrc := h.opts.MaxSourceBytes
	maxLine := h.opts.MaxLineBytes
	maxLines := h.opts.MaxLines

	if len(code) > maxSrc {
		code = code[:maxSrc]
	}

	key := h.cacheKey(code, lang)
	if lines, ok := h.cacheGet(key); ok {
		return lines
	}

	lines := h.highlight(code, lang, maxLine, maxLines)
	h.cachePut(key, lines)
	return lines
}

// Func returns a closure matching richtext.Theme.HighlightCode.
func (h *Highlighter) Func() func(code, lang string) []string {
	return h.HighlightCode
}

// Highlight is a one-shot helper using the given options (no shared cache).
func Highlight(code, lang string, opts Options) []string {
	return New(opts).HighlightCode(code, lang)
}

func (h *Highlighter) highlight(code, lang string, maxLine, maxLines int) (out []string) {
	defer func() {
		if rec := recover(); rec != nil {
			out = plainLines(code, maxLine, maxLines)
		}
	}()

	if h.opts.ColorMode == ColorNone {
		return plainLines(code, maxLine, maxLines)
	}

	lex := h.lexerFor(lang)
	toks, err := chroma.Tokenise(lex, nil, code)
	if err != nil {
		return plainLines(code, maxLine, maxLines)
	}

	st := h.styleTable()
	lineToks := chroma.SplitTokensIntoLines(toks)
	if len(lineToks) == 0 {
		return []string{""}
	}
	if len(lineToks) > maxLines {
		lineToks = lineToks[:maxLines]
	}

	out = make([]string, 0, len(lineToks))
	for _, line := range lineToks {
		out = append(out, formatLine(line, st, maxLine))
	}
	return out
}

func formatLine(toks []chroma.Token, st *styleTable, maxLine int) string {
	var b strings.Builder
	// Estimate: source plus a few SGR sequences.
	size := 0
	for _, t := range toks {
		size += len(t.Value) + 16
	}
	b.Grow(size)

	active := ""
	written := 0
	for _, tok := range toks {
		val := tok.Value
		// SplitTokensIntoLines keeps the trailing '\n' on the line's last token.
		if strings.HasSuffix(val, "\n") {
			val = strings.TrimSuffix(val, "\n")
			// Also drop bare \r if present from EnsureLF edge cases.
			val = strings.TrimSuffix(val, "\r")
		}
		if val == "" {
			continue
		}
		if written >= maxLine {
			break
		}
		if written+len(val) > maxLine {
			val = truncateUTF8Bytes(val, maxLine-written)
		}
		open := st.sgrFor(tok.Type)
		if open != active {
			if active != "" {
				b.WriteString("\x1b[0m")
			}
			if open != "" {
				b.WriteString(open)
			}
			active = open
		}
		b.WriteString(val)
		written += len(val)
	}
	if active != "" {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

func (h *Highlighter) lexerFor(lang string) chroma.Lexer {
	key := strings.ToLower(strings.TrimSpace(lang))
	// Common aliases
	switch key {
	case "":
		key = "plaintext"
	case "golang":
		key = "go"
	case "dockerfile":
		key = "docker"
	case "yml":
		key = "yaml"
	case "js":
		key = "javascript"
	case "ts":
		key = "typescript"
	case "py":
		key = "python"
	case "sh", "shell", "zsh", "bash":
		key = "bash"
	case "md":
		key = "markdown"
	case "rs":
		key = "rust"
	case "c++", "cpp", "cxx", "h", "hpp":
		key = "c++"
	case "cs", "csharp":
		key = "c#"
	case "kt":
		key = "kotlin"
	case "plaintext", "text", "plain", "txt":
		key = "plaintext"
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if lex, ok := h.lexers[key]; ok {
		return lex
	}
	lex := lexers.Get(key)
	if lex == nil {
		// Try Match by filename-ish
		if key != "plaintext" {
			lex = lexers.Get("." + key)
		}
	}
	if lex == nil {
		lex = lexers.Fallback
	}
	lex = chroma.Coalesce(lex)
	h.lexers[key] = lex
	return lex
}

func (h *Highlighter) styleTable() *styleTable {
	name := string(h.opts.Theme)
	if name == "" {
		name = string(DefaultTheme)
	}
	cacheKey := fmt.Sprintf("%s|%d", strings.ToLower(name), int(h.opts.ColorMode))

	h.mu.Lock()
	defer h.mu.Unlock()
	if st, ok := h.styleCache[cacheKey]; ok {
		return st
	}
	sty := styles.Get(name)
	if sty == nil {
		sty = styles.Fallback
	}
	st := &styleTable{
		style:   sty,
		entries: make(map[chroma.TokenType]chroma.StyleEntry, 64),
		sgr:     make(map[chroma.TokenType]string, 64),
		mode:    h.opts.ColorMode,
	}
	h.styleCache[cacheKey] = st
	return st
}

func (st *styleTable) sgrFor(tt chroma.TokenType) string {
	if s, ok := st.sgr[tt]; ok {
		return s
	}
	entry, ok := st.entries[tt]
	if !ok {
		entry = st.style.Get(tt)
		st.entries[tt] = entry
	}
	s := buildSGR(entry, st.mode)
	st.sgr[tt] = s
	return s
}

// buildSGR emits a single open SGR for the token. Background is intentionally
// omitted (no background bleed into the terminal).
func buildSGR(e chroma.StyleEntry, mode ColorMode) string {
	if mode == ColorNone {
		return ""
	}
	var parts []string
	if e.Bold == chroma.Yes {
		parts = append(parts, "1")
	}
	if e.Italic == chroma.Yes {
		parts = append(parts, "3")
	}
	if e.Underline == chroma.Yes {
		parts = append(parts, "4")
	}
	if e.Colour.IsSet() {
		r, g, b := int(e.Colour.Red()), int(e.Colour.Green()), int(e.Colour.Blue())
		switch mode {
		case ColorTrueColor:
			parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d", r, g, b))
		case ColorANSI256:
			parts = append(parts, fmt.Sprintf("38;5;%d", rgbToANSI256(r, g, b)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

func rgbToANSI256(r, g, b int) int {
	if r == g && g == b {
		if r < 8 {
			return 16
		}
		if r > 248 {
			return 231
		}
		return int(math.Round(((float64(r)-8)/247)*24)) + 232
	}
	ri := int(math.Round(float64(r) / 255 * 5))
	gi := int(math.Round(float64(g) / 255 * 5))
	bi := int(math.Round(float64(b) / 255 * 5))
	return 16 + 36*ri + 6*gi + bi
}

func plainLines(code string, maxLine, maxLines int) []string {
	code = strings.ReplaceAll(code, "\r\n", "\n")
	code = strings.ReplaceAll(code, "\r", "\n")
	// Trim one trailing newline so split matches fenced-body convention.
	code = strings.TrimSuffix(code, "\n")
	if code == "" {
		return []string{""}
	}
	lines := strings.Split(code, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for i, line := range lines {
		lines[i] = truncateUTF8Bytes(line, maxLine)
	}
	return lines
}

func truncateUTF8Bytes(s string, limit int) string {
	if limit < 0 {
		limit = 0
	}
	if len(s) <= limit {
		return s
	}
	end := limit
	for end > 0 && !utf8.ValidString(s[:end]) {
		end--
	}
	return s[:end]
}

func (h *Highlighter) cacheKey(code, lang string) string {
	hh := fnv.New64a()
	_, _ = hh.Write([]byte(lang))
	_, _ = hh.Write([]byte{0})
	_, _ = hh.Write([]byte(h.opts.Theme))
	_, _ = hh.Write([]byte{0})
	_, _ = hh.Write([]byte{byte(h.opts.ColorMode)})
	_, _ = hh.Write([]byte{0})
	_, _ = hh.Write([]byte(code))
	return fmt.Sprintf("%x", hh.Sum64())
}

func (h *Highlighter) cacheGet(key string) ([]string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	el, ok := h.cache[key]
	if !ok {
		return nil, false
	}
	h.order.MoveToFront(el)
	// Return a copy so callers cannot mutate the cached slice header contents
	// in a way that races; strings themselves are immutable.
	src := el.Value.(cacheEntry).lines
	out := make([]string, len(src))
	copy(out, src)
	return out, true
}

func (h *Highlighter) cachePut(key string, lines []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Store a private copy.
	stored := make([]string, len(lines))
	copy(stored, lines)
	if el, ok := h.cache[key]; ok {
		el.Value = cacheEntry{key: key, lines: stored}
		h.order.MoveToFront(el)
		return
	}
	el := h.order.PushFront(cacheEntry{key: key, lines: stored})
	h.cache[key] = el
	for h.order.Len() > CacheSize {
		back := h.order.Back()
		if back == nil {
			break
		}
		ent := back.Value.(cacheEntry)
		delete(h.cache, ent.key)
		h.order.Remove(back)
	}
}
