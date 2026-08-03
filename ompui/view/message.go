package view

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/media"
	"github.com/lyc-aon/ratatui-go/ompui/model"
	"github.com/lyc-aon/ratatui-go/ompui/richtext"
)

// OSC 133 shell-integration zone markers around a user turn. The prompt-start
// marker lets terminals fold the transcript by turn. Command-start (`C`) is
// deliberately absent: the transcript emits no matching command-finished
// marker, so terminals would group every later row under the first prompt.
const (
	osc133ZoneStart = "\x1b]133;A\x07"
	osc133ZoneEnd   = "\x1b]133;B\x07"
)

// Sentinels OMP stamps on aborted turns. A silent abort is an expected internal
// transition and a user interrupt is already visible in the UI, so neither
// earns a red line in the transcript.
const (
	silentAbortMarker   = "__omp.silent_abort__"
	userInterruptLabel  = "Interrupted by user"
	genericAbortMessage = "Request was aborted"
)

// messageHasReflowingMarkdown reports whether any text block of a message is
// still reflowing.
func messageHasReflowingMarkdown(msg model.Message) bool {
	for _, block := range msg.Content {
		if block.Kind == model.ContentText && HasReflowingMarkdown(block.Text) {
			return true
		}
	}
	return false
}

// Renderer turns model values into transcript rows for one theme and option
// set. It holds no mutable state, so a value copy is safe to share.
type Renderer struct {
	theme Theme
	opts  Options

	// thinkingMD is the markdown theme with every hook re-tinted for reasoning
	// bodies; thinkingTint restores that tint after any nested span resets.
	thinkingMD    richtext.Theme
	syntheticMD   richtext.Theme
	thinkingTint  func(string) string
	syntheticTint func(string) string
	userTint      func(string) string
	// railWidth is the cell width of one tree stem level. Every level costs the
	// same, so a deep trail's rows stay column-aligned under both glyph presets.
	railWidth int
}

// NewRenderer binds a theme and options into a row producer.
func NewRenderer(theme Theme, opts Options) Renderer {
	r := Renderer{theme: theme, opts: opts}
	r.thinkingMD = tintedMarkdownTheme(theme.Markdown, theme.ThinkingText, theme.Italic)
	r.syntheticMD = tintedMarkdownTheme(theme.Markdown, theme.SyntheticText, nil)
	r.thinkingTint = lineTinter(compose(theme.Italic, theme.ThinkingText))
	r.syntheticTint = lineTinter(theme.SyntheticText)
	r.userTint = lineTinter(theme.UserText)
	r.railWidth = maxGlyphWidth(theme.Symbols.TreeVertical) + 1
	return r
}

// Theme returns the bound theme.
func (r Renderer) Theme() Theme { return r.theme }

// Options returns the bound options.
func (r Renderer) Options() Options { return r.opts }

func maxGlyphWidth(values ...string) int {
	best := 0
	for _, value := range values {
		if w := ansitext.VisibleWidth(value); w > best {
			best = w
		}
	}
	if best < 1 {
		return 1
	}
	return best
}

// tintedMarkdownTheme re-points every markdown hook through an extra style so a
// reasoning body reads as one continuous quiet voice instead of a normal
// document that happens to sit behind a rule.
func tintedMarkdownTheme(base richtext.Theme, color, italic StyleFunc) richtext.Theme {
	tint := compose(italic, color)
	wrap := func(fn StyleFunc) StyleFunc { return compose(tint, fn) }
	out := base
	out.Heading1 = wrap(base.Heading1)
	out.Heading2 = wrap(base.Heading2)
	out.Heading3 = wrap(base.Heading3)
	out.Bold = wrap(base.Bold)
	out.Italic = wrap(base.Italic)
	out.Strikethrough = wrap(base.Strikethrough)
	out.Code = wrap(base.Code)
	out.CodeBlock = wrap(base.CodeBlock)
	out.Quote = wrap(base.Quote)
	out.ListBullet = wrap(base.ListBullet)
	out.Link = wrap(base.Link)
	out.LinkURL = wrap(base.LinkURL)
	out.TableHeader = wrap(base.TableHeader)
	// A quieted body is never worth a syntax-highlighted code block or a
	// rendered diagram; both would out-shout the answer they precede.
	out.HighlightCode = nil
	out.ResolveMermaidASCII = nil
	return out
}

// lineTinter returns a function that paints a whole rendered row in style,
// re-opening the style after any nested span that reset the foreground. Without
// the re-open, the first inline code span in a reasoning paragraph would drop
// the rest of the line back to the default color.
func lineTinter(style StyleFunc) func(string) string {
	if style == nil {
		return func(line string) string { return line }
	}
	open, closing := styleAffixes(style)
	if open == "" {
		return func(line string) string { return apply(style, line) }
	}
	return func(line string) string {
		if strings.TrimSpace(line) == "" {
			return line
		}
		if strings.Contains(line, "\x1b") {
			line = strings.ReplaceAll(line, "\x1b[39m", "\x1b[39m"+open)
			line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+open)
		}
		return open + line + closing
	}
}

// styleAffixes recovers the opening and closing escape runs a StyleFunc emits,
// using the same sentinel probe richtext uses for its own style prefixes.
func styleAffixes(style StyleFunc) (open, closing string) {
	if style == nil {
		return "", ""
	}
	const sentinel = "\x00"
	styled := style(sentinel)
	i := strings.Index(styled, sentinel)
	if i < 0 {
		return "", ""
	}
	return styled[:i], styled[i+len(sentinel):]
}

// renderMarkdown lays out markdown at width and strips the renderer's
// right-hand padding so selected rows copy without trailing whitespace.
func renderMarkdown(source string, theme richtext.Theme, width int) []string {
	if strings.TrimSpace(source) == "" {
		return nil
	}
	if width < 1 {
		width = 1
	}
	md := richtext.NewMarkdown(source, theme, richtext.MarkdownOptions{})
	rendered := md.Render(width)
	out := make([]string, len(rendered))
	for i, line := range rendered {
		out[i] = trimPad(line)
	}
	return out
}

// indentRows prefixes every non-blank row with pad, leaving blank rows blank so
// the transcript's plain-blank separator detection keeps working.
func indentRows(rows []string, pad string) []string {
	if pad == "" {
		return rows
	}
	for i, row := range rows {
		if row == "" {
			continue
		}
		rows[i] = pad + row
	}
	return rows
}

// MessageKind classifies a transcript message for rendering.
type MessageKind uint8

const (
	// KindUser is an operator turn.
	KindUser MessageKind = iota
	// KindAssistant is a model turn.
	KindAssistant
	// KindToolResult is a tool result; the transcript folds these into the
	// matching tool card rather than rendering them standalone.
	KindToolResult
	// KindSummary is a compaction or branch summary divider.
	KindSummary
	// KindCustom is an extension/hook message the Go frontend has no dedicated
	// view for. Rendered through the honest labelled fallback.
	KindCustom
)

// ClassifyMessage maps a message role onto a render kind.
func ClassifyMessage(msg model.Message) MessageKind {
	switch msg.Role {
	case "user", "developer":
		return KindUser
	case "assistant":
		return KindAssistant
	case "toolResult":
		return KindToolResult
	case "compactionSummary", "branchSummary":
		return KindSummary
	default:
		return KindCustom
	}
}

// messageExtras are the fields the Go frontend reads out of the preserved raw
// payload because they are not part of the decoded core shape. The names match
// Hermes' Msg so a core that already speaks to the TypeScript frontend needs no
// translation layer.
type messageExtras struct {
	Steering   bool   `json:"steering"`
	CustomType string `json:"customType"`
	Display    *bool  `json:"display"`

	// Kind is the block kind: event, diff, slash, trail, intro, or panel. It
	// drives both the visual band and the grouping gaps.
	Kind string `json:"kind"`

	// Todos carries a plan block. TodoCollapsedByDefault archives a settled plan
	// behind its chevron; TodoIncomplete flags a turn that ended with open tasks.
	Todos                  json.RawMessage `json:"todos"`
	TodoCollapsedByDefault bool            `json:"todoCollapsedByDefault"`
	TodoIncomplete         bool            `json:"todoIncomplete"`

	// ThinkingTokens and ToolTokens are the core's reported token counts for a
	// turn's working area. Zero reasoning tokens fall back to an estimate; zero
	// tool tokens simply drop the suffix, because guessing them would be a lie.
	ThinkingTokens int `json:"thinkingTokens"`
	ToolTokens     int `json:"toolTokens"`
}

func readExtras(msg model.Message) messageExtras {
	var extras messageExtras
	if len(msg.Raw) > 0 {
		_ = json.Unmarshal(msg.Raw, &extras)
	}
	return extras
}

// Hermes block kinds carried in messageExtras.Kind.
const (
	blockKindEvent = "event"
	blockKindDiff  = "diff"
	blockKindSlash = "slash"
	blockKindTrail = "trail"
)

// messageText concatenates the text blocks of a message.
func messageText(msg model.Message) string {
	if len(msg.Content) == 1 && msg.Content[0].Kind == model.ContentText {
		return msg.Content[0].Text
	}
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Kind == model.ContentText {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// UserMessage renders an operator turn: a marker column, the prose behind it,
// and OSC 133 prompt-zone framing so terminals can fold by turn.
//
// The marker distinguishes the three ways a turn can arrive — typed, steered
// mid-run, or injected by the harness — with a glyph and a color rather than a
// label, so the distinction costs no rows and reads at a glance.
func (r Renderer) UserMessage(msg model.Message, width int) []string {
	extras := readExtras(msg)
	sym := r.theme.Symbols

	marker, markerStyle := sym.UserCursor, r.theme.UserGutter
	tint, mdTheme := r.userTint, r.theme.Markdown
	switch {
	case msg.Synthetic || msg.Role == "developer":
		marker, markerStyle = sym.SyntheticCursor, r.theme.SyntheticGutter
		tint, mdTheme = r.syntheticTint, r.syntheticMD
	case extras.Steering:
		marker, markerStyle = sym.SteerCursor, r.theme.SteerGutter
	}

	layout := r.opts.layout(width)
	gutter := ansitext.VisibleWidth(marker) + 1
	bodyWidth := layout.Prose - gutter
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	// A slash echo is a command the operator typed, not prose the model will
	// read: it keeps the operator gutter but renders verbatim and quiet, so
	// markdown inside a command argument is never reinterpreted.
	var rows []string
	if extras.Kind == blockKindSlash {
		if text := strings.TrimSpace(messageText(msg)); text != "" {
			rows = r.wrapPlain(text, bodyWidth, r.theme.Muted)
		}
	} else {
		rows = renderMarkdown(messageText(msg), mdTheme, bodyWidth)
		for i, row := range rows {
			rows[i] = tint(row)
		}
		rows = append(rows, r.imageRows(msg.Content, bodyWidth)...)
	}
	if len(rows) == 0 {
		return nil
	}

	inset := padding(layout.Inset)
	head := inset + apply(markerStyle, marker) + padding(gutter-ansitext.VisibleWidth(marker))
	continuation := inset + padding(gutter)

	out := make([]string, len(rows))
	for i, row := range rows {
		switch {
		case i == 0:
			out[i] = head + row
		case row == "":
			out[i] = ""
		default:
			out[i] = continuation + row
		}
	}
	if r.opts.ShowTimestamps && msg.Timestamp > 0 {
		out[0] += apply(r.theme.Dim, "  "+time.UnixMilli(msg.Timestamp).Format("15:04"))
	}
	out[0] = osc133ZoneStart + out[0]
	out[len(out)-1] += osc133ZoneEnd
	return out
}

// AssistantMessage renders a model turn.
//
// Under an explicit detail mode the turn takes Hermes' shape: the working-area
// trail (reasoning + grouped tool calls) first, then a `Response` separator, then
// the answer prose. Without one it keeps OMP's historical layout — reasoning
// behind a quiet rule, inline with the prose — so hosts that never opted into
// section modes see no change.
//
// frame drives the reasoning pulse while reasoning is hidden and still
// streaming; it is a plain counter, never a wall clock.
func (r Renderer) AssistantMessage(msg model.Message, width int, frame uint64) []string {
	if r.opts.FocusView || !r.opts.detailModesExplicit() {
		return r.legacyAssistantMessage(msg, width, frame)
	}
	trail := r.TrailRows(r.MessageTrail(msg), width, frame)
	body := r.AssistantBody(msg, width, len(trail) > 0)
	switch {
	case len(trail) == 0:
		return body
	case len(body) == 0:
		return trail
	default:
		return append(append(trail, ""), body...)
	}
}

// AssistantBody renders only the answer half of a model turn: prose, images, and
// any terminal error. responseSeparator prepends Hermes' `└─ Response` rule,
// which is what keeps a final answer from reading as one more trail row.
//
// The transcript renders the trail as its own block so a running call keeps its
// own live region, and calls this for the answer.
func (r Renderer) AssistantBody(msg model.Message, width int, responseSeparator bool) []string {
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)
	focus := r.opts.FocusView

	var out []string
	hasProse, hasToolCalls := false, false
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentToolCall:
			hasToolCalls = true
		case model.ContentText:
			text := strings.TrimSpace(block.Text)
			if text == "" {
				continue
			}
			hasProse = true
			out = append(out, indentRows(renderMarkdown(text, r.theme.Markdown, layout.Prose), inset)...)
		}
	}
	if !focus {
		out = append(out, indentRows(r.imageRows(msg.Content, layout.Prose), inset)...)
	}
	if responseSeparator && hasProse {
		out = append([]string{inset + r.responseSeparatorRow(layout), ""}, out...)
	}

	abortsSuppressed := hasToolCalls && !focus
	switch {
	case msg.StopReason == "aborted" && !abortsSuppressed && shouldRenderAbort(msg.Error):
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, inset+apply(r.theme.Error, r.theme.Symbols.Aborted+" "+abortLabel(msg.Error)))
	case msg.StopReason == "error":
		out = append(out, r.errorRows(msg.Error, layout, len(out) > 0)...)
	}
	if msg.Error != "" && shouldRenderAbort(msg.Error) &&
		msg.StopReason != "aborted" && msg.StopReason != "error" {
		out = append(out, r.errorRows(msg.Error, layout, len(out) > 0)...)
	}
	return out
}

// responseSeparatorRow is the rule between a turn's working area and its answer.
func (r Renderer) responseSeparatorRow(layout Layout) string {
	row := apply(r.theme.Border, r.theme.Symbols.TreeLast+" ") + apply(r.theme.Dim, "Response")
	return fit(row, layout.Body, r.theme.Symbols.Ellipsis)
}

// assistantBodyRenders reports whether [Renderer.AssistantBody] would draw a row.
// A turn that has only requested tools has no answer yet, and a block that draws
// nothing must not exist: the transcript would otherwise reserve a separator for
// it and strand a blank row between the trail above and the next turn below.
func (r Renderer) assistantBodyRenders(msg model.Message) bool {
	hasToolCalls := false
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				return true
			}
		case model.ContentImage:
			if !r.opts.FocusView {
				return true
			}
		case model.ContentToolCall:
			hasToolCalls = true
		}
	}
	switch {
	case msg.StopReason == "aborted":
		return (!hasToolCalls || r.opts.FocusView) && shouldRenderAbort(msg.Error)
	case msg.StopReason == "error":
		return true
	}
	return msg.Error != "" && shouldRenderAbort(msg.Error)
}

// MessageTrail extracts a message's working area: its reasoning and the calls it
// requested. Cards carry only what the call block itself knows; the transcript
// folds in live executions and results before rendering.
func (r Renderer) MessageTrail(msg model.Message) Trail {
	extras := readExtras(msg)
	trail := Trail{ReasoningTokens: extras.ThinkingTokens, ToolTokens: extras.ToolTokens}

	var reasoning strings.Builder
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentThinking:
			text := strings.TrimSpace(block.Text)
			if text == "" {
				continue
			}
			if reasoning.Len() > 0 {
				reasoning.WriteString("\n\n")
			}
			reasoning.WriteString(text)
		case model.ContentRedactedThinking:
			trail.ReasoningRedacted = true
		case model.ContentToolCall:
			if block.ToolCall != nil {
				trail.Cards = append(trail.Cards, ToolCardFrom(block.ToolCall, nil, nil))
			}
		}
	}
	trail.Reasoning = reasoning.String()
	trail.ReasoningActive = reasoningIsActive(msg)
	return trail
}

// reasoningIsActive reports that the model is reasoning right now: the turn is
// still streaming and its trailing block is reasoning. A tool call in the message
// ends it — the work has moved to the tool.
func reasoningIsActive(msg model.Message) bool {
	if !msg.Streaming {
		return false
	}
	active := false
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentToolCall:
			return false
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				active = false
			}
		case model.ContentThinking:
			if strings.TrimSpace(block.Text) != "" {
				active = true
			}
		case model.ContentRedactedThinking:
			// The provider can redact an in-flight reasoning block. It is still
			// active work, so it earns the same honest live state.
			active = true
		}
	}
	return active
}

// legacyAssistantMessage is OMP's pre-detail-mode turn layout, retained for
// hosts that never set an explicit section mode and for focus view, which keeps
// the answer above the failure backstop it belongs to.
func (r Renderer) legacyAssistantMessage(msg model.Message, width int, frame uint64) []string {
	if r.opts.FocusView {
		return r.AssistantBody(msg, width, false)
	}
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)
	thinkingMode := r.opts.thinkingMode()
	var out []string

	hasVisible := false
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				hasVisible = true
			}
		case model.ContentThinking:
			if thinkingMode != DetailModeHidden && strings.TrimSpace(block.Text) != "" {
				hasVisible = true
			}
		case model.ContentRedactedThinking:
			if thinkingMode != DetailModeHidden {
				hasVisible = true
			}
		}
	}

	hasToolCalls := false
	for i, block := range msg.Content {
		switch block.Kind {
		case model.ContentToolCall:
			hasToolCalls = true
		case model.ContentText:
			text := strings.TrimSpace(block.Text)
			if text == "" {
				continue
			}
			out = append(out, indentRows(renderMarkdown(text, r.theme.Markdown, layout.Prose), inset)...)
		case model.ContentThinking:
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			rows := r.thinkingSectionRows(block.Text, width, thinkingMode, false)
			if len(rows) == 0 {
				continue
			}
			out = append(out, rows...)
			if hasVisibleContentAfter(msg.Content[i+1:], thinkingMode) {
				out = append(out, "")
			}
		case model.ContentRedactedThinking:
			rows := r.thinkingSectionRows("", width, thinkingMode, true)
			if len(rows) == 0 {
				continue
			}
			out = append(out, rows...)
			if hasVisibleContentAfter(msg.Content[i+1:], thinkingMode) {
				out = append(out, "")
			}
		}
	}

	if r.shouldPulseThinking(msg) {
		if hasVisible {
			out = append(out, "")
		}
		out = append(out, inset+r.ThinkingPulse(frame))
	}

	out = append(out, indentRows(r.imageRows(msg.Content, layout.Prose), inset)...)

	switch {
	case msg.StopReason == "aborted" && !hasToolCalls && shouldRenderAbort(msg.Error):
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, inset+apply(r.theme.Error, r.theme.Symbols.Aborted+" "+abortLabel(msg.Error)))
	case msg.StopReason == "error":
		out = append(out, r.errorRows(msg.Error, layout, len(out) > 0)...)
	}
	if msg.Error != "" && shouldRenderAbort(msg.Error) &&
		msg.StopReason != "aborted" && msg.StopReason != "error" {
		out = append(out, r.errorRows(msg.Error, layout, len(out) > 0)...)
	}
	return out
}

// hasVisibleContentAfter reports whether any later block still renders, so a
// reasoning block only pays for a trailing blank row when something follows it
// inside the same message.
func hasVisibleContentAfter(rest []model.ContentBlock, thinkingMode DetailMode) bool {
	for _, block := range rest {
		switch block.Kind {
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				return true
			}
		case model.ContentThinking:
			if thinkingMode != DetailModeHidden && strings.TrimSpace(block.Text) != "" {
				return true
			}
		case model.ContentRedactedThinking:
			if thinkingMode != DetailModeHidden {
				return true
			}
		}
	}
	return false
}

// shouldPulseThinking mirrors OMP: the pulse stands in for suppressed reasoning
// only while the turn is still streaming, no tool call has started, and the
// trailing block is reasoning — i.e. the model is thinking right now.
func (r Renderer) shouldPulseThinking(msg model.Message) bool {
	if r.opts.FocusView || r.opts.thinkingMode() != DetailModeHidden || !msg.Streaming {
		return false
	}
	tail := ""
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentToolCall:
			// A hidden running tool supplies its own generic pulse. A visible
			// tool card supplies a running status, so neither case needs a
			// duplicate reasoning pulse.
			return false
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				tail = "text"
			}
		case model.ContentThinking:
			if strings.TrimSpace(block.Text) != "" {
				tail = "thinking"
			}
		case model.ContentRedactedThinking:
			// The provider can explicitly redact an in-flight reasoning block.
			// It is still active work, so hidden mode needs the same honest
			// pulse as ordinary reasoning rather than silently going blank.
			tail = "thinking"
		}
	}
	return tail == "thinking"
}

// ThinkingPulse renders the single-glyph reasoning indicator for a frame index.
func (r Renderer) ThinkingPulse(frame uint64) string {
	frames := r.theme.Symbols.ThinkingPulse
	if len(frames) == 0 {
		return apply(r.theme.ThinkingText, "thinking")
	}
	return apply(r.theme.ThinkingText, frames[frameIndex(frame, frames)])
}

// thinkingSectionRows is OMP's reasoning body: full prose behind a quiet rule.
// Only the legacy layout reaches it — an explicit detail mode renders reasoning
// as a Thinking panel inside the trail tree instead.
func (r Renderer) thinkingSectionRows(text string, width int, mode DetailMode, redacted bool) []string {
	if mode == DetailModeHidden {
		return nil
	}
	if redacted {
		layout := r.opts.layout(width)
		return []string{padding(layout.Inset) + apply(r.theme.Dim, r.theme.Symbols.QuoteBar+" reasoning redacted by provider")}
	}
	return r.ThinkingRows(text, width)
}

func detailChevron(symbols Symbols, expanded bool) string {
	if symbols.Rule == "-" {
		if expanded {
			return "v"
		}
		return ">"
	}
	if expanded {
		return "▾"
	}
	return "▸"
}

// ThinkingRows renders a visible reasoning body behind a dim vertical rule.
// The rule is what makes reasoning skimmable: the eye can drop the whole column
// without reading it, and the answer beneath stays flush and copyable.
func (r Renderer) ThinkingRows(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	layout := r.opts.layout(width)
	bar := r.theme.Symbols.QuoteBar
	barWidth := ansitext.VisibleWidth(bar) + 1
	bodyWidth := layout.Prose - barWidth
	if bodyWidth < 1 {
		bodyWidth = 1
	}
	rows := renderMarkdown(text, r.thinkingMD, bodyWidth)
	inset := padding(layout.Inset)
	gutter := apply(r.theme.ThinkingGutter, bar) + " "
	for i, row := range rows {
		rows[i] = inset + gutter + r.thinkingTint(row)
	}
	return rows
}

// errorRows renders a turn-ending provider error, bounded so a proxy's HTML
// error page cannot become the transcript.
//
// The `Error: ` label costs cells too, so the message is budgeted against what
// remains after it. Below that the label wins and the message sheds: an operator
// on a 20-column pane needs to know a turn failed more than they need the fourth
// word of the reason.
func (r Renderer) errorRows(message string, layout Layout, spaceBefore bool) []string {
	const label = "Error: "
	budget := layout.Body - len(label)
	if budget < 1 {
		budget = 1
	}
	body := previewLines(message, maxTranscriptErrorLines, budget, r.theme.Symbols.Ellipsis)
	if len(body) == 0 {
		body = []string{"Unknown error"}
	}
	inset := padding(layout.Inset)
	ellipsis := r.theme.Symbols.Ellipsis
	out := make([]string, 0, len(body)+1)
	if spaceBefore {
		out = append(out, "")
	}
	out = append(out, inset+fit(apply(r.theme.Error, label+body[0]), layout.Body, ellipsis))
	for _, line := range body[1:] {
		out = append(out, inset+fit("  "+apply(r.theme.Error, line), layout.Body, ellipsis))
	}
	return out
}

func shouldRenderAbort(errorMessage string) bool {
	return errorMessage != silentAbortMarker && errorMessage != userInterruptLabel
}

func abortLabel(errorMessage string) string {
	if errorMessage != "" && errorMessage != genericAbortMessage && errorMessage != silentAbortMarker {
		return flattenLine(errorMessage)
	}
	return "Operation aborted"
}

// imageRows renders the image blocks of a message.
func (r Renderer) imageRows(blocks []model.ContentBlock, width int) []string {
	var out []string
	for i, block := range blocks {
		if block.Kind != model.ContentImage {
			continue
		}
		out = append(out, r.ImageRows(ImageRequest{
			Key:      "content:" + strconv.Itoa(i),
			Base64:   block.Data,
			MIMEType: block.MIMEType,
		}, width)...)
	}
	return out
}

// ImageRows renders one image through the configured adapter, falling back to
// honest metadata — media type and pixel size — when no adapter is installed or
// the adapter declines. Never a fake thumbnail, never silence.
func (r Renderer) ImageRows(req ImageRequest, width int) []string {
	if r.opts.ImageAdapter != nil {
		if comp := r.opts.ImageAdapter(req); comp != nil {
			frame := comp.Render(width)
			if len(frame.Lines) > 0 {
				return component.CloneLines(frame.Lines)
			}
		}
	}
	return []string{apply(r.theme.ToolOutput, imageFallbackLabel(req))}
}

// imageFallbackLabel mirrors OMP imageFallback: filename, media type, and pixel
// dimensions when they can be sniffed from the payload.
func imageFallbackLabel(req ImageRequest) string {
	parts := make([]string, 0, 3)
	if req.Filename != "" {
		parts = append(parts, media.SanitizeName(req.Filename))
	}
	mime := req.MIMEType
	if mime == "" {
		mime = "unknown"
	}
	parts = append(parts, "["+mime+"]")
	if dims, ok := media.GetImageDimensions(req.Base64, req.MIMEType); ok {
		parts = append(parts, strconv.Itoa(dims.WidthPx)+"x"+strconv.Itoa(dims.HeightPx))
	}
	return "[Image: " + strings.Join(parts, " ") + "]"
}

// SummaryMessage renders a compaction or branch divider: a labelled rule plus a
// bounded body. The rule earns its ink here — a context boundary is exactly the
// place a reader needs to know the history above was rewritten.
func (r Renderer) SummaryMessage(msg model.Message, width int) []string {
	if r.opts.FocusView {
		return nil
	}
	layout := r.opts.layout(width)
	label := "context compacted"
	if msg.Role == "branchSummary" {
		label = "branch summary"
	}
	inset := padding(layout.Inset)
	labelText := " " + label + " "
	fill := layout.Body - ansitext.VisibleWidth(labelText)
	if fill < 0 {
		fill = 0
	}
	head := inset + apply(r.theme.BorderMuted, rule(r.theme.Symbols.Rule, fill/2)) +
		apply(r.theme.Muted, labelText) +
		apply(r.theme.BorderMuted, rule(r.theme.Symbols.Rule, fill-fill/2))

	out := []string{head}
	for _, line := range previewLines(messageText(msg), maxBannerLines, layout.Body-2, r.theme.Symbols.Ellipsis) {
		out = append(out, inset+" "+apply(r.theme.Dim, line))
	}
	return out
}

// CustomMessage renders every block the transcript has no first-class view for.
//
// Hermes' own block kinds get their own voice — a timeline event is a dim marker,
// a diff segment is a patch, a slash echo is a quiet command line, a plan is a
// todo panel, a long system note collapses behind a chevron. Anything genuinely
// unknown still never drops content silently: the custom type is labelled, the
// body renders as markdown, and a collapsed body states how many lines it hid.
func (r Renderer) CustomMessage(msg model.Message, width int) []string {
	if r.opts.FocusView {
		return nil
	}
	extras := readExtras(msg)
	if extras.Display != nil && !*extras.Display {
		return nil
	}
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)
	text := strings.TrimSpace(messageText(msg))

	if rows := r.TodoBlockRows(msg, width); rows != nil {
		return rows
	}
	switch extras.Kind {
	case blockKindEvent:
		return r.eventRow(text, layout)
	case blockKindDiff:
		return r.diffRows(messageText(msg), layout)
	case blockKindTrail:
		// A trail block with no plan, reasoning, or calls carries only a token
		// tally. It draws nothing rather than an empty gutter row, which keeps it
		// transparent to the grouping gaps.
		return nil
	}
	if msg.Role == "system" && r.opts.detailModesExplicit() {
		return r.systemNoteRows(text, layout)
	}

	label := extras.CustomType
	if label == "" {
		label = msg.Role
	}
	if label == "" {
		label = "message"
	}
	out := []string{inset + apply(r.theme.Muted, apply(r.theme.Bold, label))}
	if text == "" {
		return out
	}
	if !r.opts.toolsExpanded() {
		lines := strings.Split(text, "\n")
		if len(lines) > maxTranscriptErrorLines {
			hidden := len(lines) - maxTranscriptErrorLines
			text = strings.Join(lines[:maxTranscriptErrorLines], "\n") + "\n\n" +
				r.theme.Symbols.Ellipsis + " " + strconv.Itoa(hidden) + " more " + pluralize("line", hidden)
		}
	}
	return append(out, indentRows(renderMarkdown(text, r.theme.Markdown, layout.Prose-1), inset+" ")...)
}

// systemCollapseChars is where a system note (a system prompt, an AGENTS.md
// dump) stops being a note and becomes a wall. Past it only the first line shows,
// behind a collapsed chevron that states the size.
const systemCollapseChars = 400

// systemNoteRows renders a system note behind the assistant gutter: a short one
// reads inline, a long one collapses to its first line plus a character count.
func (r Renderer) systemNoteRows(text string, layout Layout) []string {
	if text == "" {
		return nil
	}
	inset := padding(layout.Inset)
	if len(text) <= systemCollapseChars {
		marker := r.theme.Symbols.SyntheticCursor
		gutterWidth := maxGlyphWidth(marker) + 1
		gutter := apply(r.theme.Muted, marker+padding(gutterWidth-maxGlyphWidth(marker)))
		continuation := padding(gutterWidth)
		body := r.wrapPlain(text, layout.Prose-gutterWidth, r.theme.Muted)
		out := make([]string, 0, len(body))
		for i, row := range body {
			prefix := continuation
			if i == 0 {
				prefix = gutter
			}
			out = append(out, trimPad(inset+prefix+row))
		}
		return out
	}
	first := flattenLine(strings.SplitN(text, "\n", 2)[0])
	head := apply(r.theme.Accent, detailChevron(r.theme.Symbols, false)+" ") +
		apply(r.theme.Muted, compactPreview(first, 120, r.theme.Symbols.Ellipsis)) +
		apply(r.theme.Dim, r.theme.Symbols.Sep+formatNumber(int64(len(text)))+" chars")
	return []string{inset + fit(head, layout.Body, r.theme.Symbols.Ellipsis)}
}
