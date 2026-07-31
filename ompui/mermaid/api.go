// Package mermaid renders a bounded subset of Mermaid diagrams to ASCII/Unicode
// text for terminal display.
//
// Ported from mermaid-ascii 1.4.0 (github.com/AlexanderGrooff/mermaid-ascii),
// commit 823db562a4439e342541643bbd5cb7d75c930e8e, MIT License
// Copyright (c) 2023 Alexander Grooff.
//
// Supported grammar:
//   - graph / flowchart LR | TD | TB (labels, edge labels, subgraphs, nested
//     subgraphs, back-edges, self-edges)
//   - sequenceDiagram (participants, solid/dotted arrows, notes, loop/opt/alt/par)
//
// Unsupported or malformed input returns (source, false) / error so callers can
// honest-fallback to fenced code. No process execution, terminal writes, or
// global flags. Clip to viewport width is the richtext caller's job.
package mermaid

import (
	"container/list"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
)

// ColorMode selects how Colorize (if set) should emit ANSI.
type ColorMode int

const (
	// ColorNone disables ANSI coloring.
	ColorNone ColorMode = iota
	// ColorANSI256 requests 256-color sequences.
	ColorANSI256
	// ColorTrueColor requests 24-bit sequences.
	ColorTrueColor
)

// Options control rendering symbols and optional injected coloring.
type Options struct {
	// UseASCII forces pure ASCII box/arrow glyphs. When false, Unicode
	// box-drawing is used (default).
	UseASCII bool
	// ColorMode is forwarded to Colorize; adapters may ignore it when Colorize is nil.
	ColorMode ColorMode
	// Colorize optionally wraps a finished plain diagram in ANSI. Receives the
	// full multi-line string. Nil leaves output uncolored.
	Colorize func(plain string, mode ColorMode) string
	// MaxSourceBytes bounds accepted source size (0 = default 64KiB).
	MaxSourceBytes int
	// MaxWidth, when > 0, triggers orientation fallback for graphs that exceed it.
	// Sequence diagrams are never re-oriented; width is advisory only.
	MaxWidth int
}

const (
	defaultMaxSourceBytes = 64 << 10
	cacheCapacity         = 256
)

type cacheEntry struct {
	key string
	val cachedVal
}

type cachedVal struct {
	out string
	ok  bool
	err string // empty if err==nil; store string for copy safety
}

// Renderer is a concurrency-safe mermaid→ASCII adapter with a bounded LRU.
type Renderer struct {
	mu    sync.Mutex
	opts  Options
	cache map[string]*list.Element
	order *list.List
}

// New returns a Renderer with the given options.
func New(opts Options) *Renderer {
	if opts.MaxSourceBytes <= 0 {
		opts.MaxSourceBytes = defaultMaxSourceBytes
	}
	return &Renderer{
		opts:  opts,
		cache: make(map[string]*list.Element, cacheCapacity),
		order: list.New(),
	}
}

// Default is a package-level renderer with Unicode glyphs and no coloring.
var defaultRenderer = New(Options{})

// Render parses and renders source. On unsupported/malformed input it returns
// the original source, false, and a non-nil error. Success returns (ascii, true, nil).
// Failures are cached so repeated bad fences stay cheap.
func Render(source string, opts Options) (string, bool, error) {
	r := New(opts)
	return r.Render(source)
}

// Render implements the instance path with LRU caching.
func (r *Renderer) Render(source string) (out string, ok bool, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			out, ok, err = source, false, fmt.Errorf("mermaid: panic: %v", rec)
		}
	}()

	if r == nil {
		return defaultRenderer.Render(source)
	}

	src := source
	maxBytes := r.opts.MaxSourceBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxSourceBytes
	}
	if len(src) > maxBytes {
		return source, false, fmt.Errorf("mermaid: source exceeds %d bytes", maxBytes)
	}

	key := r.cacheKey(src)
	if v, hit := r.cacheGet(key); hit {
		if !v.ok {
			if v.err != "" {
				return source, false, fmt.Errorf("%s", v.err)
			}
			return source, false, fmt.Errorf("mermaid: render failed")
		}
		return v.out, true, nil
	}

	plain, ok, err := r.renderUncached(src)
	if !ok || err != nil {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		r.cachePut(key, cachedVal{out: "", ok: false, err: errStr})
		if err == nil {
			err = fmt.Errorf("mermaid: unsupported or empty render")
		}
		return source, false, err
	}

	out = plain
	if r.opts.Colorize != nil && r.opts.ColorMode != ColorNone {
		out = r.opts.Colorize(plain, r.opts.ColorMode)
	}
	r.cachePut(key, cachedVal{out: out, ok: true})
	return out, true, nil
}

// ResolveMermaidASCII matches richtext.Theme.ResolveMermaidASCII.
// ok=false falls back to normal fenced-code rendering.
func (r *Renderer) ResolveMermaidASCII(source string, maxWidth int) (string, bool) {
	if r == nil {
		return "", false
	}
	// Copy options with MaxWidth override for this call.
	return r.resolveWithWidth(source, maxWidth)
}

func (r *Renderer) resolveWithWidth(source string, maxWidth int) (string, bool) {
	defer func() { _ = recover() }()
	src := source
	maxBytes := r.opts.MaxSourceBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxSourceBytes
	}
	if len(src) > maxBytes {
		return "", false
	}
	key := r.cacheKeyWidth(src, maxWidth)
	if v, hit := r.cacheGet(key); hit {
		return v.out, v.ok
	}
	plain, ok, err := r.renderUncachedWidth(src, maxWidth)
	if !ok || err != nil || plain == "" {
		r.cachePut(key, cachedVal{ok: false, err: errString(err)})
		return "", false
	}
	out := plain
	if r.opts.Colorize != nil && r.opts.ColorMode != ColorNone {
		out = r.opts.Colorize(plain, r.opts.ColorMode)
	}
	r.cachePut(key, cachedVal{out: out, ok: true})
	return out, true
}

// ResolveMermaidASCII is a package-level hook matching richtext.Theme.ResolveMermaidASCII
// using default Unicode options and the given maxWidth.
func ResolveMermaidASCII(source string, maxWidth int) (string, bool) {
	opts := Options{MaxWidth: maxWidth}
	out, ok, _ := Render(source, opts)
	if !ok {
		return "", false
	}
	return out, true
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (r *Renderer) cacheKey(src string) string {
	return r.cacheKeyWidth(src, r.opts.MaxWidth)
}

func (r *Renderer) cacheKeyWidth(src string, maxWidth int) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(src))
	ascii := byte(0)
	if r.opts.UseASCII {
		ascii = 1
	}
	return fmt.Sprintf("%x|%d|%d|%d", h.Sum64(), maxWidth, ascii, int(r.opts.ColorMode))
}

func (r *Renderer) cacheGet(key string) (cachedVal, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	el, ok := r.cache[key]
	if !ok {
		return cachedVal{}, false
	}
	r.order.MoveToFront(el)
	return el.Value.(cacheEntry).val, true
}

func (r *Renderer) cachePut(key string, val cachedVal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if el, ok := r.cache[key]; ok {
		el.Value = cacheEntry{key: key, val: val}
		r.order.MoveToFront(el)
		return
	}
	el := r.order.PushFront(cacheEntry{key: key, val: val})
	r.cache[key] = el
	for r.order.Len() > cacheCapacity {
		back := r.order.Back()
		if back == nil {
			break
		}
		ent := back.Value.(cacheEntry)
		delete(r.cache, ent.key)
		r.order.Remove(back)
	}
}

func (r *Renderer) renderUncached(source string) (string, bool, error) {
	return r.renderUncachedWidth(source, r.opts.MaxWidth)
}

func (r *Renderer) renderUncachedWidth(source string, maxW int) (string, bool, error) {
	input := strings.TrimSpace(source)
	if input == "" {
		return "", false, fmt.Errorf("mermaid: empty input")
	}

	if IsSequenceDiagram(input) {
		sd, err := ParseSequence(input)
		if err != nil {
			return "", false, err
		}
		cfg := r.diagramConfig()
		out, err := renderSequence(sd, cfg)
		if err != nil {
			return "", false, err
		}
		out = strings.TrimRight(out, "\n")
		if out == "" {
			return "", false, fmt.Errorf("mermaid: empty sequence render")
		}
		return out, true, nil
	}

	// Graph / flowchart path.
	if !looksLikeGraph(input) {
		return "", false, fmt.Errorf("mermaid: unsupported diagram type")
	}

	cfg := r.diagramConfig()
	authored, err := renderGraphWithDirection(input, cfg, "")
	if err != nil {
		return "", false, err
	}
	authored = strings.TrimRight(authored, "\n")
	if authored == "" {
		return "", false, fmt.Errorf("mermaid: empty graph render")
	}

	if maxW <= 0 || maxLineWidth(authored) <= maxW {
		return authored, true, nil
	}

	// Orientation exceeds maxWidth: try forced TD and LR, return narrowest.
	tdSrc := forceGraphDirection(input, "TD")
	lrSrc := forceGraphDirection(input, "LR")
	candidates := []string{authored}
	if td, err := renderGraphWithDirection(tdSrc, cfg, "TD"); err == nil {
		td = strings.TrimRight(td, "\n")
		if td != "" {
			candidates = append(candidates, td)
		}
	}
	if lr, err := renderGraphWithDirection(lrSrc, cfg, "LR"); err == nil {
		lr = strings.TrimRight(lr, "\n")
		if lr != "" {
			candidates = append(candidates, lr)
		}
	}
	best := candidates[0]
	bestW := maxLineWidth(best)
	for _, c := range candidates[1:] {
		w := maxLineWidth(c)
		if w < bestW {
			best, bestW = c, w
		}
	}
	return best, true, nil
}

func (r *Renderer) diagramConfig() *Config {
	cfg := DefaultConfig()
	cfg.UseAscii = r.opts.UseASCII
	cfg.StyleType = "cli"
	cfg.ShowCoords = false
	cfg.Verbose = false
	return cfg
}

func looksLikeGraph(input string) bool {
	for _, line := range strings.Split(input, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		// padding directives may precede graph header
		lt := strings.ToLower(t)
		if strings.HasPrefix(lt, "padding") {
			continue
		}
		if strings.HasPrefix(lt, "graph ") || strings.HasPrefix(lt, "flowchart ") {
			return true
		}
		// Unknown first content line → unsupported (no silent fake success).
		return false
	}
	return false
}

func renderGraphWithDirection(input string, cfg *Config, _ string) (string, error) {
	properties, err := mermaidFileToMap(input, "cli")
	if err != nil {
		return "", err
	}
	if cfg != nil {
		properties.boxBorderPadding = cfg.BoxBorderPadding
		properties.paddingX = cfg.PaddingBetweenX
		properties.paddingY = cfg.PaddingBetweenY
		properties.styleType = cfg.StyleType
		if properties.styleType == "" {
			properties.styleType = "cli"
		}
		properties.useAscii = cfg.UseAscii
	}
	return drawMap(properties), nil
}

// forceGraphDirection rewrites the first graph/flowchart header to the given dir (LR|TD).
func forceGraphDirection(input, dir string) string {
	dir = strings.ToUpper(dir)
	if dir == "TB" {
		dir = "TD"
	}
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		lt := strings.ToLower(trimmed)
		if strings.HasPrefix(lt, "padding") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			kw := strings.ToLower(fields[0])
			if kw == "graph" || kw == "flowchart" {
				// preserve keyword casing roughly
				lines[i] = fields[0] + " " + dir
				return strings.Join(lines, "\n")
			}
		}
		break
	}
	return input
}

func maxLineWidth(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		// strip simple SGR for width if present
		w := stringWidth(stripSGR(line))
		if w > max {
			max = w
		}
	}
	return max
}

func stripSGR(s string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c >= 0x40 && c <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
