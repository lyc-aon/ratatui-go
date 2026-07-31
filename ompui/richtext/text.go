package richtext

import (
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// Text is multi-line plain text with word wrap and optional background.
// Line-string API for later component adapters.
type Text struct {
	text      string
	paddingX  int
	paddingY  int
	customBg  func(line string, width int) string
	cacheText string
	cacheW    int
	cache     []string
	hasCache  bool
}

// NewText constructs a Text renderer. paddingX/Y default usage matches OMP (1,1).
func NewText(text string, paddingX, paddingY int) *Text {
	return &Text{
		text:     text,
		paddingX: paddingX,
		paddingY: paddingY,
	}
}

// SetText replaces content. Returns true when the value changed.
func (t *Text) SetText(text string) bool {
	if text == t.text {
		return false
	}
	t.text = text
	t.invalidate()
	return true
}

// Text returns the raw source string.
func (t *Text) Text() string { return t.text }

// SetPadding sets horizontal and vertical padding cells/rows.
func (t *Text) SetPadding(paddingX, paddingY int) {
	if t.paddingX == paddingX && t.paddingY == paddingY {
		return
	}
	t.paddingX = paddingX
	t.paddingY = paddingY
	t.invalidate()
}

// SetBackground sets an optional full-line background painter.
func (t *Text) SetBackground(bg func(line string, width int) string) {
	t.customBg = bg
	t.invalidate()
}

// Invalidate drops the render cache.
func (t *Text) Invalidate() { t.invalidate() }

func (t *Text) invalidate() {
	t.hasCache = false
	t.cache = nil
	t.cacheText = ""
	t.cacheW = 0
}

// Render wraps and pads the text to width columns.
func (t *Text) Render(width int) []string {
	if t.hasCache && t.cacheText == t.text && t.cacheW == width {
		return t.cache
	}
	if strings.TrimSpace(t.text) == "" {
		out := []string{}
		t.cacheText = t.text
		t.cacheW = width
		t.cache = out
		t.hasCache = true
		return out
	}

	normalized := replaceTabs(t.text)
	padX := t.paddingX
	if padX < 0 {
		padX = 0
	}
	contentWidth := width - padX*2
	if contentWidth < 1 {
		contentWidth = 1
	}
	wrapped := WrapTextWithAnsi(normalized, contentWidth)

	left := padding(padX)
	right := padding(padX)
	content := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		withMargins := left + line + right
		if t.customBg != nil {
			content = append(content, applyBackgroundToLine(withMargins, width, t.customBg))
		} else {
			vis := ansitext.VisibleWidth(withMargins)
			pad := width - vis
			if pad < 0 {
				pad = 0
			}
			content = append(content, withMargins+padding(pad))
		}
	}

	empty := padding(width)
	if t.customBg != nil {
		empty = applyBackgroundToLine(empty, width, t.customBg)
	}
	var topBot []string
	for i := 0; i < t.paddingY; i++ {
		topBot = append(topBot, empty)
	}
	result := make([]string, 0, len(topBot)*2+len(content))
	result = append(result, topBot...)
	result = append(result, content...)
	result = append(result, topBot...)
	if len(result) == 0 {
		result = []string{""}
	}

	t.cacheText = t.text
	t.cacheW = width
	t.cache = result
	t.hasCache = true
	return result
}
