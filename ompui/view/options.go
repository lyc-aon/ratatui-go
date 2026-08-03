package view

import (
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/component"
)

// Responsive breakpoints in terminal cells.
//
// narrowWidth is where two-part rows (left cluster + right-aligned cluster)
// stop fitting and stack or shed segments. microWidth is where decoration has
// to go entirely and only the primary token of each row survives.
const (
	narrowWidth = 60
	microWidth  = 34

	// defaultProseWidth caps markdown body columns. Terminal prose at 200
	// columns is unreadable for the same reason a 200-character web paragraph
	// is: the eye loses the line on the return sweep. Code fences and tables
	// live inside the same cap so a wide block never desynchronizes from the
	// paragraph column above it.
	defaultProseWidth = 100

	// Bounded tool output. Collapsed cards stay glanceable; expanded cards stay
	// bounded so a runaway result cannot flood scrollback.
	defaultToolPreviewLines  = 4
	defaultToolExpandedLines = 24

	// Bounded inline error bodies (a proxy's HTML 502 page must not become the
	// transcript).
	maxTranscriptErrorLines = 8
	maxBannerLines          = 3
)

// ImageRequest describes one image block the transcript wants rendered.
type ImageRequest struct {
	// Key is stable for the life of the image: message identity plus block
	// index, or toolCallID plus result index. An adapter should reuse the same
	// component for the same key so graphics ids and budget slots survive
	// re-renders.
	Key string
	// Base64 is the raw base64 payload from the model.
	Base64 string
	// MIMEType is the declared media type.
	MIMEType string
	// Filename is a display hint; usually empty.
	Filename string
}

// ImageAdapter turns an image block into a component. Returning nil falls back
// to the honest metadata line (`[Image: [image/png] 1024x768]`), which is also
// what happens when no adapter is installed. The adapter owns the image budget,
// transmit, and purge lifecycle; the view only asks for rows.
type ImageAdapter func(req ImageRequest) component.Component

// DetailMode controls whether a transcript detail section is omitted, summarized,
// or rendered in full. The empty value defers to the legacy boolean option for
// that section, so existing hosts retain their current behavior.
type DetailMode string

const (
	DetailModeHidden    DetailMode = "hidden"
	DetailModeCollapsed DetailMode = "collapsed"
	DetailModeExpanded  DetailMode = "expanded"
)

// Valid reports whether m is one of Hermes' supported detail section modes.
func (m DetailMode) Valid() bool {
	switch m {
	case DetailModeHidden, DetailModeCollapsed, DetailModeExpanded:
		return true
	default:
		return false
	}
}

// ParseDetailMode parses a Hermes detail mode without making callers duplicate
// normalization rules for persisted configuration values.
func ParseDetailMode(value string) (DetailMode, bool) {
	mode := DetailMode(strings.ToLower(strings.TrimSpace(value)))
	return mode, mode.Valid()
}

// Options configures every view in this package. The zero value is valid and
// renders the quiet default: collapsed tools, no timestamps, prose capped at
// [defaultProseWidth], metadata fallback for images.
type Options struct {
	// Tight drops the horizontal breathing room around block bodies.
	Tight bool
	// ToolsExpanded is the legacy ctrl+o expansion state. ToolsMode wins when
	// it is one of the supported detail modes.
	ToolsExpanded bool
	// ToolsMode controls tool detail visibility independently of reasoning.
	// Its empty value falls back to ToolsExpanded for compatibility.
	ToolsMode DetailMode
	// ShowTimestamps adds a dim clock to user turns. OMP does not show these by
	// default and neither does this package.
	ShowTimestamps bool
	// HideThinking is the legacy reasoning visibility switch. ThinkingMode wins
	// when it is one of the supported detail modes.
	HideThinking bool
	// ThinkingMode controls reasoning visibility independently of tool cards.
	// Its empty value falls back to HideThinking for compatibility.
	ThinkingMode DetailMode
	// ProseOnlyThinking strips non-prose scaffolding from reasoning text.
	ProseOnlyThinking bool

	// FocusView reduces the transcript to operator prompts and assistant prose.
	// It overrides detail modes for thinking, tools, summaries, and custom
	// activity while retaining visible failures and turn-ending errors.
	FocusView bool

	// MaxProseWidth caps markdown body columns. 0 uses [defaultProseWidth];
	// a negative value disables the cap.
	MaxProseWidth int
	// ToolPreviewLines bounds a collapsed tool body. 0 uses the default.
	ToolPreviewLines int
	// ToolExpandedLines bounds an expanded tool body. 0 uses the default.
	ToolExpandedLines int

	// ExpandHint is the key label shown on collapsed tool cards, e.g. "ctrl+o".
	// Empty hides the hint entirely.
	ExpandHint string

	// Now, when non-zero, lets running tool cards and the status rule show
	// deterministic elapsed time. Left zero the view stays time-free, which
	// keeps renders a pure function of the snapshot.
	Now time.Time

	// AnimationFrame, when positive, overrides the snapshot-derived frame index
	// used by spinners and the reasoning pulse. Hosts that want smooth motion
	// advance this from their own scheduler; the view never owns a timer.
	AnimationFrame int

	// ImageAdapter renders image blocks as real graphics. See [ImageAdapter].
	ImageAdapter ImageAdapter
}

func (o Options) proseWidth(width int) int {
	limit := o.MaxProseWidth
	if limit == 0 {
		limit = defaultProseWidth
	}
	if limit < 0 || width < limit {
		return width
	}
	return limit
}

func (o Options) toolPreviewLines() int {
	if o.toolsExpanded() {
		if o.ToolExpandedLines > 0 {
			return o.ToolExpandedLines
		}
		return defaultToolExpandedLines
	}
	if o.ToolPreviewLines > 0 {
		return o.ToolPreviewLines
	}
	return defaultToolPreviewLines
}

func (o Options) thinkingMode() DetailMode {
	if o.ThinkingMode.Valid() {
		return o.ThinkingMode
	}
	if o.HideThinking {
		return DetailModeHidden
	}
	return DetailModeExpanded
}

func (o Options) toolsMode() DetailMode {
	if o.ToolsMode.Valid() {
		return o.ToolsMode
	}
	if o.ToolsExpanded {
		return DetailModeExpanded
	}
	return DetailModeCollapsed
}

func (o Options) toolsExpanded() bool {
	return o.toolsMode() == DetailModeExpanded
}

// detailModesExplicit reports whether the host opted into Hermes' section modes.
// One valid mode is enough: the transcript's structure is shared by both
// sections, so it cannot be half Hermes and half OMP.
func (o Options) detailModesExplicit() bool {
	return o.ThinkingMode.Valid() || o.ToolsMode.Valid()
}

// frame returns the animation cycle position for a snapshot generation.
func (o Options) frame(generation uint64) uint64 {
	if o.AnimationFrame > 0 {
		return uint64(o.AnimationFrame)
	}
	return generation
}

// Layout is the resolved geometry for one render width.
type Layout struct {
	// Width is the full render width in cells.
	Width int
	// Body is the width available to block content after the tight/loose inset.
	Body int
	// Prose is the width markdown bodies wrap to (Body capped by MaxProseWidth).
	Prose int
	// Inset is the left inset applied to block bodies.
	Inset int
	// Narrow is true below [narrowWidth]: right-aligned clusters stack or drop.
	Narrow bool
	// Micro is true below [microWidth]: only the primary token of a row fits.
	Micro bool
}

func (o Options) layout(width int) Layout {
	if width < 1 {
		width = 1
	}
	inset := 1
	if o.Tight || width < microWidth {
		inset = 0
	}
	body := width - inset
	if body < 1 {
		body = 1
		inset = 0
	}
	return Layout{
		Width:  width,
		Body:   body,
		Prose:  o.proseWidth(body),
		Inset:  inset,
		Narrow: width < narrowWidth,
		Micro:  width < microWidth,
	}
}

// frameCache memoizes one component's rows per width and keeps the frame
// generation stable while the rows are byte-identical.
type frameCache struct {
	lines []string
	width int
	valid bool
	gen   component.Gen
}

func (c *frameCache) invalidate() {
	c.valid = false
	c.lines = nil
	c.width = 0
}

// store installs lines for width, reusing the previous slice (and generation)
// when the content is unchanged so containers can skip recomposition.
func (c *frameCache) store(width int, lines []string) component.Frame {
	if c.valid && c.width == width && linesEqual(c.lines, lines) {
		return component.NewFrame(c.lines, c.gen.Current())
	}
	c.lines = lines
	c.width = width
	c.valid = true
	return component.NewFrame(lines, c.gen.Next())
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
