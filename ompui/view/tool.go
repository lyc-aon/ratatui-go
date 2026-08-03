package view

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// ToolCard is the complete view state of one tool invocation: the call the
// assistant emitted, the live execution the core reports, and the result
// message when it lands. The three arrive on different frames and through
// different channels; the card is what unifies them into a single block.
type ToolCard struct {
	// ID is the tool call id and the card's stable identity.
	ID string
	// Name is the tool name.
	Name string
	// Intent is the model's one-line reason for the call.
	Intent string
	// Arguments is the raw call payload, possibly still streaming.
	Arguments json.RawMessage

	// Running reports an execution in flight.
	Running bool
	// PartialResult is a provisional snapshot streamed before completion.
	PartialResult json.RawMessage
	// Result is the terminal result payload.
	Result json.RawMessage
	// HasResult distinguishes "completed with empty output" from "still going".
	HasResult bool
	// IsError marks a failed execution.
	IsError bool

	StartedAt time.Time
	EndedAt   time.Time
}

// ToolCardFrom assembles a card from the pieces a snapshot carries. Any of the
// three sources may be nil: a call with no execution yet is a pending card, an
// execution with no call block is a card the transcript appends at the tail.
func ToolCardFrom(call *model.ToolCall, exec *model.ToolExecution, result *model.Message) ToolCard {
	var card ToolCard
	if call != nil {
		card.ID = call.ID
		card.Name = call.Name
		card.Intent = call.Intent
		card.Arguments = call.Arguments
	}
	if exec != nil {
		if card.ID == "" {
			card.ID = exec.ID
		}
		if exec.Name != "" {
			card.Name = exec.Name
		}
		if exec.Intent != "" {
			card.Intent = exec.Intent
		}
		if len(exec.Arguments) > 0 {
			card.Arguments = exec.Arguments
		}
		card.Running = exec.Running
		card.PartialResult = exec.PartialResult
		card.StartedAt = exec.StartedAt
		card.EndedAt = exec.EndedAt
		if len(exec.Result) > 0 {
			card.Result = exec.Result
			card.HasResult = true
			card.IsError = exec.IsError
		}
	}
	if result != nil {
		card.HasResult = true
		card.IsError = card.IsError || result.IsError
		if card.Name == "" {
			card.Name = result.ToolName
		}
		if card.ID == "" {
			card.ID = result.ToolCallID
		}
		if len(card.Result) == 0 {
			card.Result = result.Raw
		}
		card.Running = false
	}
	return card
}

// Settled reports whether the card has reached a terminal state. An unsettled
// card keeps the transcript's live region open beneath it.
func (c ToolCard) Settled() bool { return c.HasResult && !c.Running }

// toolStatus is the card's rendered state.
type toolStatus uint8

const (
	toolPending toolStatus = iota
	toolRunning
	toolDone
	toolFailed
)

func (c ToolCard) status() toolStatus {
	switch {
	case c.IsError:
		return toolFailed
	case c.HasResult:
		return toolDone
	case c.Running:
		return toolRunning
	default:
		return toolPending
	}
}

// statusGlyph returns the state marker and its style.
//
// Deliberately static, unlike OMP's per-frame spinner. A ticking glyph rewrites
// the card's first row on every frame, which pins the whole block out of the
// commit-safe prefix: a long-running tool's settled head then reaches neither
// native scrollback nor the viewport once it outgrows the screen. Motion lives
// in the working indicator, which is chrome and never enters scrollback.
func (r Renderer) statusGlyph(status toolStatus) (string, StyleFunc) {
	sym := r.theme.Symbols
	switch status {
	case toolRunning:
		return sym.Running, r.theme.Accent
	case toolDone:
		return sym.Success, r.theme.Success
	case toolFailed:
		return sym.Error, r.theme.Error
	default:
		return sym.Pending, r.theme.Muted
	}
}

// ToolRows renders one tool call at its configured visibility. It is retained as
// the public single-card helper; Transcript groups adjacent calls into one trail
// so the header count and tree rails cover the whole run.
func (r Renderer) ToolRows(card ToolCard, width int) []string {
	return r.toolRows(card, width, 0)
}

func (r Renderer) toolRows(card ToolCard, width int, frame uint64) []string {
	// Existing callers that only use ToolsExpanded retain OMP's historical
	// bounded preview card. Hermes' grouped trail begins once a host opts into
	// an explicit detail mode.
	if !r.opts.FocusView && !r.opts.detailModesExplicit() {
		return r.legacyToolRows(card, width)
	}
	return r.TrailRows(Trail{Cards: []ToolCard{card}}, width, frame)
}

func (r Renderer) legacyToolRows(card ToolCard, width int) []string {
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)
	status := card.status()
	glyph, glyphStyle := r.statusGlyph(status)

	out := make([]string, 0, 8)
	out = append(out, inset+r.legacyToolHeader(card, status, glyph, glyphStyle, layout))

	args, argsOK := parseJSON(card.Arguments)
	bodyIndent := inset + "  "
	if argsOK && r.opts.ToolsExpanded {
		if rows := r.toolArgTree(args, layout, bodyIndent); len(rows) > 0 {
			out = append(out, rows...)
		}
	} else if argsOK {
		connector := r.theme.Symbols.TreeLast
		budget := layout.Body - ansitext.VisibleWidth(connector) - 2
		if preview := formatArgsInline(args, budget, r.theme.Symbols.Ellipsis); preview != "" {
			out = append(out, inset+" "+apply(r.theme.Dim, connector+" "+preview))
		}
	}
	return append(out, r.toolBody(card, layout, bodyIndent)...)
}

func (r Renderer) legacyToolHeader(card ToolCard, status toolStatus, glyph string, glyphStyle StyleFunc, layout Layout) string {
	name := card.Name
	if name == "" {
		name = "tool"
	}
	head := apply(glyphStyle, glyph) + " " + apply(r.theme.ToolTitle, apply(r.theme.Bold, name))
	if card.Intent != "" && !layout.Micro {
		head += apply(r.theme.Muted, ": "+flattenLine(card.Intent))
	}
	if meta := r.toolMeta(card, status, layout); meta != "" {
		head += apply(r.theme.Dim, r.theme.Symbols.Sep+meta)
	}
	return fit(head, layout.Body, r.theme.Symbols.Ellipsis)
}

// toolMeta assembles the dim trailing cluster: elapsed time and a failure tag.
// Narrow terminals drop it entirely rather than truncate the intent to fit it.
func (r Renderer) toolMeta(card ToolCard, status toolStatus, layout Layout) string {
	if layout.Narrow {
		if status == toolFailed {
			return "failed"
		}
		return ""
	}
	parts := make([]string, 0, 2)
	if elapsed, ok := card.elapsed(r.opts.Now); ok {
		parts = append(parts, formatDuration(elapsed))
	}
	switch status {
	case toolFailed:
		parts = append(parts, "failed")
	case toolRunning:
		parts = append(parts, "running")
	}
	return strings.Join(parts, r.theme.Symbols.Sep)
}

// elapsed reports the card's duration. A completed card measures itself; a
// running one needs an injected clock, and without one it stays time-free so
// renders remain a pure function of the snapshot.
func (c ToolCard) elapsed(now time.Time) (time.Duration, bool) {
	if c.StartedAt.IsZero() {
		return 0, false
	}
	end := c.EndedAt
	if end.IsZero() {
		if now.IsZero() {
			return 0, false
		}
		end = now
	}
	d := end.Sub(c.StartedAt)
	if d <= 0 {
		return 0, false
	}
	return d, true
}

// toolArgTree renders the expanded argument view.
func (r Renderer) toolArgTree(args jsonValue, layout Layout, indent string) []string {
	rows, truncated := renderJSONTree(args, r.theme,
		r.opts.jsonTreeOptions(layout.Body-ansitext.VisibleWidth(indent)))
	if len(rows) == 0 {
		return nil
	}
	out := make([]string, 0, len(rows)+3)
	out = append(out, "", indent+apply(r.theme.Dim, "arguments"))
	for _, row := range rows {
		out = append(out, indent+row)
	}
	if truncated {
		out = append(out, indent+apply(r.theme.Dim, r.theme.Symbols.Ellipsis))
	}
	return append(out, "")
}

// toolBody renders the result (or the provisional partial result) bounded to
// the current expansion level.
func (r Renderer) toolBody(card ToolCard, layout Layout, indent string) []string {
	payload := card.Result
	provisional := false
	if !card.HasResult {
		if len(card.PartialResult) == 0 {
			return nil
		}
		payload, provisional = card.PartialResult, true
	}

	output := parseToolOutput(payload)
	bodyWidth := layout.Body - ansitext.VisibleWidth(indent)
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	var out []string
	for i, img := range output.images {
		img.Key = card.ID + ":" + strconv.Itoa(i)
		out = append(out, indentRows(r.ImageRows(img, bodyWidth), indent)...)
	}

	text := strings.TrimRight(sanitizeText(output.text), " \t\n")
	if text == "" {
		if len(out) == 0 && card.HasResult {
			out = append(out, indent+apply(r.theme.Dim, "(no output)"))
		}
		return out
	}

	style := r.theme.ToolOutput
	if card.IsError {
		style = r.theme.Error
	}

	// A JSON result is a structure, not prose: the tree is readable where a
	// single wrapped blob is not, and it is bounded by construction.
	if tree, ok := r.toolJSONBody(text, layout, indent); ok {
		out = append(out, tree...)
		if provisional {
			out = append(out, indent+fit(r.streamingLabel(), bodyWidth, r.theme.Symbols.Ellipsis))
		}
		return out
	}

	limit := r.opts.toolPreviewLines()
	lines := strings.Split(text, "\n")
	shown := clampInt(limit, 0, len(lines))
	for _, line := range lines[:shown] {
		out = append(out, indent+apply(style, fit(replaceTabs(line), bodyWidth, r.theme.Symbols.Ellipsis)))
	}
	if hidden := len(lines) - shown; hidden > 0 {
		more := apply(r.theme.Dim, r.theme.Symbols.Ellipsis+" "+strconv.Itoa(hidden)+" more "+pluralize("line", hidden))
		out = append(out, indent+fit(joinMeta("  ", more, r.expandHint()), bodyWidth, r.theme.Symbols.Ellipsis))
	} else if provisional {
		out = append(out, indent+fit(r.streamingLabel(), bodyWidth, r.theme.Symbols.Ellipsis))
	} else if hint := r.expandHint(); hint != "" {
		out = append(out, indent+fit(hint, bodyWidth, r.theme.Symbols.Ellipsis))
	}
	return out
}

// streamingLabel marks a provisional body that the final result will replace.
func (r Renderer) streamingLabel() string {
	return apply(r.theme.Dim, "streaming"+r.theme.Symbols.Ellipsis)
}

// toolJSONBody renders a JSON result as a bounded tree. Reports false when the
// payload is not JSON so the caller falls back to line output.
func (r Renderer) toolJSONBody(text string, layout Layout, indent string) ([]string, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return nil, false
	}
	value, ok := parseJSON([]byte(trimmed))
	if !ok {
		return nil, false
	}
	rows, truncated := renderJSONTree(value, r.theme,
		r.opts.jsonTreeOptions(layout.Body-ansitext.VisibleWidth(indent)))
	if len(rows) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(rows)+1)
	for _, row := range rows {
		out = append(out, indent+row)
	}
	switch {
	case truncated:
		out = append(out, indent+apply(r.theme.Dim, r.theme.Symbols.Ellipsis))
	case !r.opts.toolsExpanded():
		if hint := r.expandHint(); hint != "" {
			out = append(out, indent+hint)
		}
	}
	return out, true
}

// expandHint renders the collapsed-state affordance, or "" when expanded or
// when the host configured no key label.
func (r Renderer) expandHint() string {
	if r.opts.toolsExpanded() || r.opts.ExpandHint == "" {
		return ""
	}
	sym := r.theme.Symbols
	return apply(r.theme.Dim, sym.BracketLeft+r.opts.ExpandHint+": expand"+sym.BracketRight)
}

// toolOutput is the normalized view of a tool result payload.
type toolOutput struct {
	text   string
	images []ImageRequest
}

type wireContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Data     string `json:"data"`
	MIMEType string `json:"mimeType"`
}

// parseToolOutput normalizes the several shapes a tool result arrives in: a
// bare string, a content-block array, a result object wrapping one, or a whole
// toolResult message. Anything unrecognized is surfaced as its own raw text
// rather than dropped — losing a result is worse than showing it plainly.
func parseToolOutput(raw json.RawMessage) toolOutput {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return toolOutput{}
	}
	switch raw[0] {
	case '"':
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return toolOutput{text: text}
		}
		return toolOutput{text: string(raw)}
	case '[':
		var blocks []wireContentBlock
		if err := json.Unmarshal(raw, &blocks); err == nil {
			return collectToolBlocks(blocks)
		}
		return toolOutput{text: string(raw)}
	case '{':
		var wrapper struct {
			Content json.RawMessage `json:"content"`
			Output  string          `json:"output"`
			Text    string          `json:"text"`
		}
		if err := json.Unmarshal(raw, &wrapper); err == nil {
			if len(bytes.TrimSpace(wrapper.Content)) > 0 {
				out := parseToolOutput(wrapper.Content)
				if out.text != "" || len(out.images) > 0 {
					return out
				}
			}
			if wrapper.Output != "" {
				return toolOutput{text: wrapper.Output}
			}
			if wrapper.Text != "" {
				return toolOutput{text: wrapper.Text}
			}
		}
		// An unrecognized object is still data the operator may need; the JSON
		// body path renders it as a bounded tree.
		return toolOutput{text: string(raw)}
	default:
		return toolOutput{text: string(raw)}
	}
}

func collectToolBlocks(blocks []wireContentBlock) toolOutput {
	var out toolOutput
	var b strings.Builder
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(block.Text)
		case "image":
			out.images = append(out.images, ImageRequest{Base64: block.Data, MIMEType: block.MIMEType})
		}
	}
	out.text = b.String()
	return out
}
