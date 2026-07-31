package view

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/michaelkelly/ratatui-go/ompui/ansitext"
	"github.com/michaelkelly/ratatui-go/ompui/component"
	"github.com/michaelkelly/ratatui-go/ompui/media"
	"github.com/michaelkelly/ratatui-go/ompui/model"
	"github.com/michaelkelly/ratatui-go/ompui/richtext"
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

// codeFencePattern matches an opening or closing fence: three or more
// backticks/tildes plus an info string.
var codeFencePattern = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")

// tableDelimiterPattern matches a GFM table delimiter row (`| --- | :--: |`,
// with or without bounding pipes). The header row alone renders as prose; this
// delimiter is what makes markdown lay a table out.
var tableDelimiterPattern = regexp.MustCompile(`^ {0,3}\|?(?:[ \t]*:?-+:?[ \t]*\|)+[ \t]*:?-*:?[ \t]*$`)

// mermaidInfoPattern matches a mermaid fence info string.
var mermaidInfoPattern = regexp.MustCompile(`^mermaid\b`)

// HasReflowingMarkdown reports whether text currently contains markdown whose
// layout is not yet permanent: an open mermaid fence (the diagram reshapes as
// source arrives) or a GFM table (columns re-align as rows arrive).
//
// This is what keeps a streaming table out of native scrollback. Committing an
// intermediate layout strands a stale fragment in immutable history that only a
// full repaint can clear, so such a block stays wholly repaintable and commits
// once, at its final layout.
//
// Fence-aware: table delimiters inside ordinary fenced code (shell pipes, ASCII
// separators, doc examples) are ignored, so a long streamed code block is never
// held back. A delimiter counts only directly under a pipe-bearing header row,
// outside any code fence.
func HasReflowingMarkdown(text string) bool {
	fence := ""
	prev := ""
	for _, line := range strings.Split(text, "\n") {
		match := codeFencePattern.FindStringSubmatch(line)
		if fence != "" {
			// Inside a code block: only a bare matching closing fence ends it.
			if match != nil && strings.TrimSpace(match[2]) == "" &&
				match[1][0] == fence[0] && len(match[1]) >= len(fence) {
				fence = ""
			}
			continue
		}
		if match != nil {
			if mermaidInfoPattern.MatchString(strings.TrimSpace(match[2])) {
				return true
			}
			fence = match[1]
			prev = ""
			continue
		}
		if strings.Contains(prev, "|") && tableDelimiterPattern.MatchString(line) {
			return true
		}
		prev = line
	}
	return false
}

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
	// gutter is the fixed user-marker column width, sized to the widest marker
	// so consecutive turns with different markers stay column-aligned.
	gutter int
}

// NewRenderer binds a theme and options into a row producer.
func NewRenderer(theme Theme, opts Options) Renderer {
	r := Renderer{theme: theme, opts: opts}
	r.thinkingMD = tintedMarkdownTheme(theme.Markdown, theme.ThinkingText, theme.Italic)
	r.syntheticMD = tintedMarkdownTheme(theme.Markdown, theme.SyntheticText, nil)
	r.thinkingTint = lineTinter(compose(theme.Italic, theme.ThinkingText))
	r.syntheticTint = lineTinter(theme.SyntheticText)
	r.userTint = lineTinter(theme.UserText)
	sym := theme.Symbols
	r.gutter = 1 + maxGlyphWidth(sym.UserCursor, sym.SteerCursor, sym.SyntheticCursor)
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
// payload because they are not part of the decoded core shape.
type messageExtras struct {
	Steering   bool   `json:"steering"`
	CustomType string `json:"customType"`
	Display    *bool  `json:"display"`
}

func readExtras(msg model.Message) messageExtras {
	var extras messageExtras
	if len(msg.Raw) > 0 {
		_ = json.Unmarshal(msg.Raw, &extras)
	}
	return extras
}

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
	bodyWidth := layout.Prose - r.gutter
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	rows := renderMarkdown(messageText(msg), mdTheme, bodyWidth)
	for i, row := range rows {
		rows[i] = tint(row)
	}
	rows = append(rows, r.imageRows(msg.Content, bodyWidth)...)
	if len(rows) == 0 {
		return nil
	}

	inset := padding(layout.Inset)
	head := inset + apply(markerStyle, marker) + padding(r.gutter-ansitext.VisibleWidth(marker))
	continuation := inset + padding(r.gutter)

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

// AssistantMessage renders a model turn: prose flush in the reading column,
// reasoning behind a quiet rule, images, and any terminal error.
//
// frame drives the reasoning pulse while reasoning is hidden and still
// streaming; it is a plain counter, never a wall clock.
func (r Renderer) AssistantMessage(msg model.Message, width int, frame uint64) []string {
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)
	var out []string

	hasVisible := false
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				hasVisible = true
			}
		case model.ContentThinking:
			if !r.opts.HideThinking && strings.TrimSpace(block.Text) != "" {
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
			if r.opts.HideThinking || strings.TrimSpace(block.Text) == "" {
				continue
			}
			out = append(out, r.ThinkingRows(block.Text, width)...)
			if hasVisibleContentAfter(msg.Content[i+1:], r.opts.HideThinking) {
				out = append(out, "")
			}
		case model.ContentRedactedThinking:
			out = append(out, inset+apply(r.theme.Dim, r.theme.Symbols.QuoteBar+" reasoning redacted by provider"))
		}
	}

	if r.shouldPulseThinking(msg) {
		if hasVisible {
			out = append(out, "")
		}
		out = append(out, inset+r.ThinkingPulse(frame))
	}

	out = append(out, indentRows(r.imageRows(msg.Content, layout.Prose), inset)...)

	if !hasToolCalls {
		switch {
		case msg.StopReason == "aborted" && shouldRenderAbort(msg.Error):
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, inset+apply(r.theme.Error, r.theme.Symbols.Aborted+" "+abortLabel(msg.Error)))
		case msg.StopReason == "error":
			out = append(out, r.errorRows(msg.Error, layout, len(out) > 0)...)
		}
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
func hasVisibleContentAfter(rest []model.ContentBlock, hideThinking bool) bool {
	for _, block := range rest {
		switch block.Kind {
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				return true
			}
		case model.ContentThinking:
			if !hideThinking && strings.TrimSpace(block.Text) != "" {
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
	if !r.opts.HideThinking || !msg.Streaming {
		return false
	}
	tail := ""
	for _, block := range msg.Content {
		switch block.Kind {
		case model.ContentToolCall:
			return false
		case model.ContentText:
			if strings.TrimSpace(block.Text) != "" {
				tail = "text"
			}
		case model.ContentThinking:
			if strings.TrimSpace(block.Text) != "" {
				tail = "thinking"
			}
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
func (r Renderer) errorRows(message string, layout Layout, spaceBefore bool) []string {
	body := previewLines(message, maxTranscriptErrorLines, layout.Body-2, r.theme.Symbols.Ellipsis)
	if len(body) == 0 {
		body = []string{"Unknown error"}
	}
	inset := padding(layout.Inset)
	out := make([]string, 0, len(body)+1)
	if spaceBefore {
		out = append(out, "")
	}
	out = append(out, inset+apply(r.theme.Error, "Error: "+body[0]))
	for _, line := range body[1:] {
		out = append(out, inset+"  "+apply(r.theme.Error, line))
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

// CustomMessage renders an extension or hook message the Go frontend has no
// dedicated view for. It never drops content silently: the custom type is
// labelled, the body renders as markdown, and a collapsed body states how many
// lines are hidden.
func (r Renderer) CustomMessage(msg model.Message, width int) []string {
	extras := readExtras(msg)
	if extras.Display != nil && !*extras.Display {
		return nil
	}
	label := extras.CustomType
	if label == "" {
		label = msg.Role
	}
	if label == "" {
		label = "message"
	}

	layout := r.opts.layout(width)
	inset := padding(layout.Inset)
	out := []string{inset + apply(r.theme.Muted, apply(r.theme.Bold, label))}

	text := strings.TrimSpace(messageText(msg))
	if text == "" {
		return out
	}
	if !r.opts.ToolsExpanded {
		lines := strings.Split(text, "\n")
		if len(lines) > maxTranscriptErrorLines {
			hidden := len(lines) - maxTranscriptErrorLines
			text = strings.Join(lines[:maxTranscriptErrorLines], "\n") + "\n\n" +
				r.theme.Symbols.Ellipsis + " " + strconv.Itoa(hidden) + " more " + pluralize("line", hidden)
		}
	}
	return append(out, indentRows(renderMarkdown(text, r.theme.Markdown, layout.Prose-1), inset+" ")...)
}
