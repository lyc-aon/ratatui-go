package view

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
)

// Trail is one working-area block: the reasoning the model produced and the tool
// calls it made, rendered as Hermes' ToolTrail accordion.
//
// A trail is assembled from a snapshot once (see Transcript.buildSpecs) and is
// the only structure that renders tool calls under an explicit DetailMode. The
// cards stay in call order, which is what makes the count in `Tool calls (N)`
// and the tree rails deterministic across frames.
type Trail struct {
	// Reasoning is the concatenated reasoning text of the block.
	Reasoning string
	// ReasoningRedacted marks reasoning the provider withheld. It still opens a
	// Thinking panel: silence would read as "the model did not think".
	ReasoningRedacted bool
	// ReasoningActive marks reasoning that is still arriving.
	ReasoningActive bool
	// ReasoningTokens is the core's reported reasoning token count. Zero falls
	// back to a rough estimate over the text.
	ReasoningTokens int
	// ToolTokens is the core's reported tool token count.
	ToolTokens int
	// Cards are the block's tool calls in call order.
	Cards []ToolCard
}

// Empty reports a trail with nothing to render at any visibility.
func (t Trail) Empty() bool {
	return len(t.Cards) == 0 && !t.ReasoningRedacted && !t.ReasoningActive &&
		strings.TrimSpace(t.Reasoning) == ""
}

// Settled reports whether every card has reached a terminal state and reasoning
// has stopped arriving.
func (t Trail) Settled() bool {
	if t.ReasoningActive {
		return false
	}
	for i := range t.Cards {
		if !t.Cards[i].Settled() {
			return false
		}
	}
	return true
}

// failures counts the cards that ended in error.
func (t Trail) failures() int {
	n := 0
	for i := range t.Cards {
		if t.Cards[i].IsError {
			n++
		}
	}
	return n
}

// pending reports whether any card is still in flight.
func (t Trail) pending() bool {
	for i := range t.Cards {
		if !t.Cards[i].Settled() {
			return true
		}
	}
	return false
}

// trailShowsDetails mirrors Hermes showDetails: whether a turn's own working area
// is visible enough to earn a `Response` rule above its answer. Reasoning that is
// merely in flight does not count — a rule above a bare pulse would separate the
// answer from nothing.
func (r Renderer) trailShowsDetails(trail Trail) bool {
	if r.opts.FocusView {
		return trail.failures() > 0
	}
	thinking := r.opts.thinkingMode() != DetailModeHidden &&
		(strings.TrimSpace(trail.Reasoning) != "" || trail.ReasoningRedacted)
	tools := r.opts.toolsMode() != DetailModeHidden && len(trail.Cards) > 0
	return thinking || tools
}

// trailPanelsVisible reports whether [Renderer.TrailRows] draws at least one
// accordion panel. When it does not, the trail falls back to its backstop.
func (r Renderer) trailPanelsVisible(trail Trail) bool {
	thinkingMode, toolsMode := r.opts.thinkingMode(), r.opts.toolsMode()
	if thinkingMode == DetailModeHidden && toolsMode == DetailModeHidden {
		return false
	}
	thinking := thinkingMode != DetailModeHidden &&
		(strings.TrimSpace(trail.Reasoning) != "" || trail.ReasoningRedacted)
	tools := toolsMode != DetailModeHidden && len(trail.Cards) > 0
	return thinking || tools
}

// trailPulses reports whether the trail renders the generic hidden-work pulse.
// The pulse is the one row whose content depends on the frame counter, so the
// transcript folds that counter into its hash only here.
func (r Renderer) trailPulses(trail Trail) bool {
	if r.opts.FocusView || r.trailPanelsVisible(trail) || trail.failures() > 0 {
		return false
	}
	return trail.pending() || trail.ReasoningActive
}

// treeRail is the per-level stem cell: a vertical rule when the level still has
// siblings below, blank when it does not.
func (r Renderer) treeRail(on bool) string {
	if !on {
		return padding(r.railWidth)
	}
	return r.theme.Symbols.TreeVertical + padding(r.railWidth-ansitext.VisibleWidth(r.theme.Symbols.TreeVertical))
}

// treeLead builds the stem prefix for one tree row: one cell per ancestor level
// plus this row's branch connector. Port of Hermes treeLead.
func (r Renderer) treeLead(rails []bool, last bool) string {
	var b strings.Builder
	b.Grow((len(rails)+1)*r.railWidth + 2)
	for _, on := range rails {
		b.WriteString(r.treeRail(on))
	}
	branch := r.theme.Symbols.TreeBranch
	if last {
		branch = r.theme.Symbols.TreeLast
	}
	b.WriteString(branch)
	b.WriteByte(' ')
	return b.String()
}

// nextRails extends rails for the children of a row: a mid branch keeps its stem
// running past its children, a last branch does not.
func nextRails(rails []bool, last bool) []bool {
	out := make([]bool, len(rails)+1)
	copy(out, rails)
	out[len(rails)] = !last
	return out
}

// trailPanel is one accordion section: a header row plus, when open, a body
// keyed on the rails its children inherit.
type trailPanel struct {
	header string
	body   func(rails []bool) []string
}

// TrailRows renders a working-area block as Hermes' ToolTrail: a Thinking panel,
// a grouped `Tool calls (N)` panel, and a token rollup, all hung off one tree.
//
// Visibility is per section. A hidden section contributes no row at all — not an
// empty header, not a stem — because a stem with nothing under it is exactly the
// phantom gap the grouping rules exist to prevent.
func (r Renderer) TrailRows(trail Trail, width int, frame uint64) []string {
	if r.opts.FocusView {
		return r.focusTrailRows(trail, width)
	}
	layout := r.opts.layout(width)
	if !r.trailPanelsVisible(trail) {
		// Every section this trail could fill resolved to hidden, or it has
		// nothing to fill them with. A failure is still the operator's business,
		// so it survives as a backstop.
		return r.hiddenTrailRows(trail, layout, frame)
	}
	thinkingMode, toolsMode := r.opts.thinkingMode(), r.opts.toolsMode()

	reasoning := thinkingPreview(trail.Reasoning, DetailModeExpanded, thinkingCotMax,
		r.opts.ProseOnlyThinking, r.theme.Symbols.Ellipsis)
	hasThinking := thinkingMode != DetailModeHidden &&
		(reasoning != "" || trail.ReasoningRedacted || trail.ReasoningActive)
	hasTools := toolsMode != DetailModeHidden && len(trail.Cards) > 0

	reasoningTokens := trail.ReasoningTokens
	if reasoningTokens <= 0 {
		reasoningTokens = estimateTokensRough(trail.Reasoning)
	}
	if !hasThinking {
		reasoningTokens = 0
	}
	toolTokens := trail.ToolTokens
	if !hasTools {
		toolTokens = 0
	}

	panels := make([]trailPanel, 0, 2)
	if hasThinking {
		panels = append(panels, r.thinkingPanel(reasoning, trail, layout, thinkingMode, reasoningTokens))
	}
	if hasTools {
		panels = append(panels, r.toolsPanel(trail.Cards, layout, toolsMode, toolTokens))
	}

	total := len(panels)
	rollup := ""
	if reasoningTokens > 0 && toolTokens > 0 {
		rollup = "~" + fmtK(reasoningTokens+toolTokens) + " total"
		total++
	}

	out := make([]string, 0, total+4)
	inset := padding(layout.Inset)
	for i := range panels {
		last := i == total-1
		lead := r.treeLead(nil, last)
		out = append(out, inset+fit(apply(r.theme.Dim, lead)+panels[i].header, layout.Body, r.theme.Symbols.Ellipsis))
		if panels[i].body != nil {
			out = append(out, panels[i].body(nextRails(nil, last))...)
		}
	}
	if rollup != "" {
		lead := r.treeLead(nil, true)
		row := apply(r.theme.Dim, lead) + apply(r.theme.Accent, r.theme.Symbols.IconTokens+" ") + apply(r.theme.Dim, rollup)
		out = append(out, inset+fit(row, layout.Body, r.theme.Symbols.Ellipsis))
	}
	return out
}

// focusTrailRows keeps only the failure backstop: focus mode promises a quiet
// transcript, and a silently swallowed tool failure is not quiet, it is wrong.
func (r Renderer) focusTrailRows(trail Trail, width int) []string {
	layout := r.opts.layout(width)
	if n := trail.failures(); n > 0 {
		return []string{padding(layout.Inset) + r.failureBackstop(n, layout)}
	}
	return nil
}

// hiddenTrailRows deliberately reveals no reasoning, call name, argument, or
// result. A generic pulse proves hidden work is still in flight; a failure stays
// visible rather than disappearing into a mode the operator forgot they set.
func (r Renderer) hiddenTrailRows(trail Trail, layout Layout, frame uint64) []string {
	inset := padding(layout.Inset)
	if n := trail.failures(); n > 0 {
		return []string{inset + r.failureBackstop(n, layout)}
	}
	if trail.pending() || trail.ReasoningActive {
		return []string{inset + fit(r.ThinkingPulse(frame), layout.Body, r.theme.Symbols.Ellipsis)}
	}
	return nil
}

// failureBackstop names how many calls failed and nothing else about them.
func (r Renderer) failureBackstop(count int, layout Layout) string {
	label := "Tool call failed"
	if count > 1 {
		label = strconv.Itoa(count) + " tool calls failed"
	}
	return fit(apply(r.theme.Error, r.theme.Symbols.Error+" "+label), layout.Body, r.theme.Symbols.Ellipsis)
}

// thinkingPanel builds the Thinking section. A live panel wears the bright bold
// title; a settled one recedes into the chrome.
func (r Renderer) thinkingPanel(reasoning string, trail Trail, layout Layout, mode DetailMode, tokens int) trailPanel {
	expanded := mode == DetailModeExpanded
	title := apply(r.theme.Muted, "Thinking")
	if trail.ReasoningActive {
		title = apply(r.theme.Bold, apply(r.theme.Text, "Thinking"))
	}
	suffix := ""
	if tokens > 0 {
		suffix = "~" + fmtK(tokens) + " tokens"
	}
	panel := trailPanel{header: r.chevronHeader(title, "", suffix, expanded)}
	if !expanded {
		return panel
	}
	panel.body = func(rails []bool) []string {
		body := reasoning
		if trail.ReasoningRedacted && body == "" {
			body = "reasoning redacted by provider"
		}
		if body == "" {
			return nil
		}
		return r.treeTextRows(rails, true, body, r.theme.ThinkingText, layout)
	}
	return panel
}

// toolsPanel builds the grouped `Tool calls (N)` section. The count is the whole
// point of grouping: one glance says how much work the turn did, and the rows
// under it stay in call order so a re-render never reshuffles them.
func (r Renderer) toolsPanel(cards []ToolCard, layout Layout, mode DetailMode, tokens int) trailPanel {
	expanded := mode == DetailModeExpanded
	suffix := ""
	if tokens > 0 {
		suffix = "~" + fmtK(tokens) + " tokens"
	}
	panel := trailPanel{
		header: r.chevronHeader(apply(r.theme.Muted, "Tool calls"), " ("+strconv.Itoa(len(cards))+")", suffix, expanded),
	}
	if !expanded {
		return panel
	}
	panel.body = func(rails []bool) []string {
		out := make([]string, 0, len(cards)*3)
		for i := range cards {
			last := i == len(cards)-1
			out = append(out, r.toolGroupRows(cards[i], rails, last, layout)...)
		}
		return out
	}
	return panel
}

// toolGroupRows renders one call: the summary row plus its argument and result
// detail rows, each hung off the summary's own stem.
//
// A settled call keeps the neutral bullet and lets its colour carry the outcome,
// exactly as the oracle does; an in-flight call swaps the bullet for its status
// glyph, which is the one thing a static transcript can say that a spinner does.
func (r Renderer) toolGroupRows(card ToolCard, rails []bool, last bool, layout Layout) []string {
	glyph, glyphStyle := r.theme.Symbols.Bullet, r.theme.ToolTitle
	var call, detail string
	failed := false
	if card.Settled() {
		if parsed, ok := parseToolTrailResultLine(r.toolTrailLine(card)); ok {
			call, detail, failed = parsed.call, parsed.detail, parsed.failed
		}
	} else {
		call = r.toolTrailLine(card)
		glyph, glyphStyle = r.statusGlyph(card.status())
	}

	label, duration := splitToolDuration(call)
	rowStyle := r.theme.Text
	if failed {
		rowStyle = r.theme.Error
	}
	row := apply(r.theme.Dim, r.treeLead(rails, last)) +
		apply(glyphStyle, glyph+" ") +
		apply(rowStyle, label)
	if duration != "" {
		row += apply(r.theme.Dim, duration)
	}

	inset := padding(layout.Inset)
	out := make([]string, 0, 4)
	out = append(out, inset+fit(row, layout.Body, r.theme.Symbols.Ellipsis))
	if detail == "" {
		return out
	}
	detailStyle := r.theme.Dim
	if failed {
		detailStyle = r.theme.Error
	}
	return append(out, r.treeTextRows(nextRails(rails, last), true, detail, detailStyle, layout)...)
}

// toolTrailLine builds the trail line for a card: the settled Hermes line with
// its `Args`/`Result` blocks behind ` :: `, or the bare call label while the call
// is still in flight. Collapsed and hidden modes never reach here, which is what
// keeps a private call name, its arguments, and its result out of a quiet
// transcript.
func (r Renderer) toolTrailLine(card ToolCard) string {
	name := card.Name
	if name == "" {
		name = "tool"
	}
	seconds, hasDuration := 0.0, false
	if d, ok := card.elapsed(r.opts.Now); ok {
		seconds, hasDuration = d.Seconds(), true
	}
	if !card.Settled() {
		return formatToolCall(name, card.Intent, r.theme.Symbols.Ellipsis) + trailDuration(seconds, hasDuration)
	}
	return buildVerboseToolTrailLine(name, card.Intent, card.IsError, seconds, hasDuration,
		r.toolArgsText(card.Arguments), r.toolResultText(card), r.theme.Symbols.Ellipsis)
}

// toolArgsText renders call arguments one field per line so a trail detail stays
// scannable instead of collapsing into one long object.
func (r Renderer) toolArgsText(raw json.RawMessage) string {
	value, ok := parseJSON(raw)
	if !ok {
		return strings.TrimSpace(string(raw))
	}
	fields := visibleFields(value)
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	for i, field := range fields {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(field.key)
		b.WriteString(": ")
		b.WriteString(flattenLine(formatScalar(field.value, verboseTrailMaxChars, r.theme.Symbols.Ellipsis)))
	}
	return b.String()
}

// toolResultText normalizes the result payload to the text the trail shows.
func (r Renderer) toolResultText(card ToolCard) string {
	payload := card.Result
	if !card.HasResult {
		payload = card.PartialResult
	}
	output := parseToolOutput(payload)
	text := strings.TrimRight(sanitizeText(output.text), " \t\n")
	if text == "" && len(output.images) > 0 {
		return strconv.Itoa(len(output.images)) + " " + pluralize("image", len(output.images))
	}
	if text == "" && card.HasResult {
		return "(no output)"
	}
	return text
}

// chevronHeader renders one accordion header: the open/closed chevron, the
// title, an optional count, and a dim trailing suffix.
func (r Renderer) chevronHeader(title, count, suffix string, expanded bool) string {
	row := apply(r.theme.Accent, detailChevron(r.theme.Symbols, expanded)+" ") + title
	if count != "" {
		row += apply(r.theme.Muted, count)
	}
	if suffix != "" {
		row += apply(r.theme.Dim, "  "+suffix)
	}
	return row
}

// treeTextRows lays a text body out behind a tree stem: the first row carries the
// branch connector, continuation rows align under the body column. This is what
// keeps a multi-line reasoning body or tool result inside its branch instead of
// running back to the left margin.
func (r Renderer) treeTextRows(rails []bool, last bool, body string, style StyleFunc, layout Layout) []string {
	lead := r.treeLead(rails, last)
	leadWidth := ansitext.VisibleWidth(lead)
	bodyWidth := layout.Prose - leadWidth
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	inset := padding(layout.Inset)
	dimLead := apply(r.theme.Dim, lead)
	continuation := padding(leadWidth)

	rows := ansitext.WrapANSI(replaceTabs(body), bodyWidth)
	out := make([]string, 0, len(rows))
	for i, row := range rows {
		prefix := continuation
		if i == 0 {
			prefix = dimLead
		}
		// A grapheme wider than the body column cannot be broken, so the wrap can
		// still overshoot. Bounding here is what keeps the promise that no
		// transcript row is wider than the terminal.
		out = append(out, trimPad(fit(inset+prefix+apply(style, row), layout.Width, r.theme.Symbols.Ellipsis)))
	}
	return out
}
