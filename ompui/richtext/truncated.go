package richtext

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// TruncatedText shows a single truncated line with optional padding.
type TruncatedText struct {
	text     string
	paddingX int
	paddingY int
	cacheW   int
	cache    []string
	hasCache bool
}

// NewTruncatedText constructs a single-line truncating renderer.
func NewTruncatedText(text string, paddingX, paddingY int) *TruncatedText {
	return &TruncatedText{
		text:     text,
		paddingX: paddingX,
		paddingY: paddingY,
		cacheW:   -1,
	}
}

// SetText replaces content and invalidates the cache.
func (t *TruncatedText) SetText(text string) {
	if text == t.text {
		return
	}
	t.text = text
	t.Invalidate()
}

// Text returns the raw source.
func (t *TruncatedText) Text() string { return t.text }

// SetPadding updates padding.
func (t *TruncatedText) SetPadding(paddingX, paddingY int) {
	if t.paddingX == paddingX && t.paddingY == paddingY {
		return
	}
	t.paddingX = paddingX
	t.paddingY = paddingY
	t.Invalidate()
}

// Invalidate drops the render cache.
func (t *TruncatedText) Invalidate() {
	t.cacheW = -1
	t.cache = nil
	t.hasCache = false
}

// Render truncates the first line to fit width after horizontal padding.
// Does not pad the content line to full width (OMP: avoid trailing spaces on copy).
func (t *TruncatedText) Render(width int) []string {
	if t.hasCache && t.cacheW == width {
		return t.cache
	}
	result := make([]string, 0, 1+t.paddingY*2)
	empty := padding(width)
	for i := 0; i < t.paddingY; i++ {
		result = append(result, empty)
	}

	avail := width - t.paddingX*2
	if avail < 1 {
		avail = 1
	}

	single := t.text
	if i := strings.IndexByte(single, '\n'); i >= 0 {
		single = single[:i]
	}
	display := ansitext.TruncateToWidth(single, avail, "…")
	left := padding(t.paddingX)
	right := padding(t.paddingX)
	result = append(result, left+display+right)

	for i := 0; i < t.paddingY; i++ {
		result = append(result, empty)
	}

	t.cacheW = width
	t.cache = result
	t.hasCache = true
	return result
}
