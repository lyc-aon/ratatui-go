package editor

import (
	"regexp"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/ansitext"
	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/fuzzy"
)
var (
	reAtFile   = regexp.MustCompile(`(?:^|[\s])@[^\s]*$`)
	reHashAct  = regexp.MustCompile(`#[^\s#]*$`)
	reEmoji    = regexp.MustCompile(`(?:^|[\s([{>]):[a-zA-Z0-9_+-]*$`)
	reURLToken = regexp.MustCompile(`(?i)(?:^|[\s"'` + "`" + `(<=])[a-z][a-z0-9+.-]*:/{1,2}[^\s"'` + "`" + `()<>]*$`)
	rePathChar = regexp.MustCompile(`^[a-zA-Z0-9.\-_/]$`)
)

func (e *Editor) cancelAutocomplete(notify bool) {
	was := e.acMode != acOff
	e.acRequestID++
	e.acMode = acOff
	e.acItems = nil
	e.acPrefix = ""
	e.acSelected = 0
	e.acPendingRefresh = false
	if notify && was && e.OnAutocompleteCancel != nil {
		e.OnAutocompleteCancel()
	}
}

// handleAutocompleteKey returns true when the key was consumed by the dropdown.
//
// Navigation resolves the tui.select.* actions. Escape and ctrl+p/ctrl+n stay
// literal: escape belongs to the host interrupt contract, and ctrl+p/ctrl+n are
// the dropdown's own emacs aliases — the host relies on them reaching the
// dropdown instead of cycling models while it is open.
func (e *Editor) handleAutocompleteKey(k event.Key) bool {
	if event.MatchesKey(k, keyInterruptEscape) {
		e.cancelAutocomplete(true)
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return true
	}
	if e.matches(k, actSelectUp) || event.MatchesKey(k, keyDropdownPrev) {
		if len(e.acItems) == 0 {
			return true
		}
		if e.acSelected == 0 {
			e.acSelected = len(e.acItems) - 1
		} else {
			e.acSelected--
		}
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return true
	}
	if e.matches(k, actSelectDown) || event.MatchesKey(k, keyDropdownNext) {
		if len(e.acItems) == 0 {
			return true
		}
		e.acSelected = (e.acSelected + 1) % len(e.acItems)
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return true
	}
	if e.matches(k, actSelectPageUp) {
		e.acSelected -= e.acMaxVisible
		if e.acSelected < 0 {
			e.acSelected = 0
		}
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return true
	}
	if e.matches(k, actSelectPageDown) {
		e.acSelected += e.acMaxVisible
		if e.acSelected >= len(e.acItems) {
			e.acSelected = len(e.acItems) - 1
		}
		if e.acSelected < 0 {
			e.acSelected = 0
		}
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return true
	}

	// Tab — apply selection
	if e.matches(k, actTab) {
		e.acceptAutocomplete(false)
		return true
	}

	// Confirm on slash command: apply then fall through to submit
	if e.matches(k, actSelectConfirm) {
		prefix := e.acPrefix
		if findLeadingSlashCommandStart(prefix) >= 0 {
			cur := e.currentLine()
			before := cur[:e.cursorCol]
			if before != e.acPrefix {
				e.cancelAutocomplete(false)
				// fall through — return false so submit runs
				return false
			}
			e.acceptAutocomplete(true)
			// fall through to submit
			return false
		}
		// File path etc.: apply and consume
		e.acceptAutocomplete(false)
		return true
	}

	// Other keys fall through to normal editing; dropdown updates after insert/delete.
	return false
}

func (e *Editor) acceptAutocomplete(keepOpenForChain bool) {
	if e.acProvider == nil || len(e.acItems) == 0 {
		e.cancelAutocomplete(false)
		return
	}
	if e.acSelected < 0 || e.acSelected >= len(e.acItems) {
		e.acSelected = 0
	}
	item := e.acItems[e.acSelected]
	shouldChain := e.isSlashCommandNameAutocompleteSelection()
	result := e.acProvider.ApplyCompletion(cloneLines(e.lines), e.cursorLine, e.cursorCol, item, e.acPrefix)
	e.recordUndo()
	e.applyCompletionResult(result)
	e.cancelAutocomplete(false)
	if e.OnAutocompleteUpdate != nil {
		e.OnAutocompleteUpdate()
	}
	e.fireChange()
	if result.OnApplied != nil {
		// already called in applyCompletionResult
	}
	if shouldChain && e.isCompletedSlashCommandAtCursor() {
		e.tryTriggerAutocomplete(false)
	}
	_ = keepOpenForChain
}

func (e *Editor) handleTabCompletion() {
	if e.acProvider == nil {
		return
	}
	cur := e.currentLine()
	before := cur[:e.cursorCol]
	if e.isInSubmittedSlashCommandContext() && !strings.Contains(strings.TrimLeft(before, " \t"), " ") {
		e.tryTriggerAutocomplete(true)
		return
	}
	e.forceFileAutocomplete(true)
}

func (e *Editor) tryTriggerAutocomplete(explicitTab bool) {
	if e.acProvider == nil {
		return
	}
	if explicitTab {
		if fp, ok := e.acProvider.(ForceFileProvider); ok {
			if !fp.ShouldTriggerFileCompletion(e.lines, e.cursorLine, e.cursorCol) {
				return
			}
		}
	}
	e.dispatchSuggestions(false)
}

func (e *Editor) forceFileAutocomplete(explicitTab bool) {
	if e.acProvider == nil {
		return
	}
	fp, ok := e.acProvider.(ForceFileProvider)
	if !ok {
		e.tryTriggerAutocomplete(true)
		return
	}
	e.acRequestID++
	req := e.acRequestID
	lines := cloneLines(e.lines)
	line, col := e.cursorLine, e.cursorCol
	// Sync path: provider is expected to be fast; host may wrap async.
	suggestions := fp.GetForceFileSuggestions(lines, line, col)
	if req != e.acRequestID {
		return
	}
	if suggestions == nil || len(suggestions.Items) == 0 {
		e.cancelAutocomplete(false)
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return
	}
	if explicitTab && len(suggestions.Items) == 1 {
		item := suggestions.Items[0]
		result := e.acProvider.ApplyCompletion(lines, line, col, item, suggestions.Prefix)
		e.recordUndo()
		e.applyCompletionResult(result)
		e.fireChange()
		return
	}
	e.acPrefix = suggestions.Prefix
	e.acItems = append([]AutocompleteItem(nil), suggestions.Items...)
	e.acSelected = 0
	e.acMode = acForce
	if e.OnAutocompleteUpdate != nil {
		e.OnAutocompleteUpdate()
	}
}

func (e *Editor) dispatchSuggestions(forceMode bool) {
	if e.acProvider == nil {
		return
	}
	e.acRequestID++
	req := e.acRequestID
	lines := cloneLines(e.lines)
	line, col := e.cursorLine, e.cursorCol

	var suggestions *Suggestions
	if forceMode {
		if fp, ok := e.acProvider.(ForceFileProvider); ok {
			suggestions = fp.GetForceFileSuggestions(lines, line, col)
		} else {
			suggestions = e.acProvider.GetSuggestions(lines, line, col)
		}
	} else {
		suggestions = e.acProvider.GetSuggestions(lines, line, col)
	}
	if req != e.acRequestID {
		return
	}
	if suggestions == nil || len(suggestions.Items) == 0 {
		e.cancelAutocomplete(false)
		if e.OnAutocompleteUpdate != nil {
			e.OnAutocompleteUpdate()
		}
		return
	}
	e.acPrefix = suggestions.Prefix
	e.acItems = append([]AutocompleteItem(nil), suggestions.Items...)
	e.acSelected = 0
	if forceMode {
		e.acMode = acForce
	} else {
		e.acMode = acRegular
	}
	if e.OnAutocompleteUpdate != nil {
		e.OnAutocompleteUpdate()
	}
}

func (e *Editor) updateAutocomplete() {
	if e.acMode == acOff || e.acProvider == nil {
		return
	}
	if e.acMode == acForce {
		e.forceFileAutocomplete(false)
		return
	}
	e.dispatchSuggestions(false)
}

func (e *Editor) debouncedUpdateAutocomplete() {
	// No timers in leaf component (Dispose would need them). Immediate update;
	// host may debounce via OnAutocompleteUpdate + BeginAutocompleteRequest.
	e.updateAutocomplete()
}

func (e *Editor) retriggerAutocompleteAtCursor() {
	if e.acMode != acOff {
		e.debouncedUpdateAutocomplete()
		return
	}
	cur := e.currentLine()
	before := cur[:e.cursorCol]
	if e.isInSubmittedSlashCommandContext() {
		e.tryTriggerAutocomplete(false)
	} else if reAtFile.MatchString(before) {
		e.tryTriggerAutocomplete(false)
	} else if reHashAct.MatchString(before) {
		e.tryTriggerAutocomplete(false)
	} else if e.textTriggersURLAutocomplete(before) {
		e.tryTriggerAutocomplete(false)
	}
}

func (e *Editor) maybeTriggerAutocompleteAfterInsert(char string) {
	if e.acMode != acOff {
		e.debouncedUpdateAutocomplete()
		return
	}
	if char == "/" && e.isAtStartOfSubmittedMessage() {
		e.tryTriggerAutocomplete(false)
		return
	}
	if char == "@" {
		cur := e.currentLine()
		before := cur[:e.cursorCol]
		if len(before) == 1 {
			e.tryTriggerAutocomplete(false)
			return
		}
		// char before @
		if len(before) >= 2 {
			prev := before[len(before)-2]
			if prev == ' ' || prev == '\t' {
				e.tryTriggerAutocomplete(false)
			}
		}
		return
	}
	if char == "#" {
		e.tryTriggerAutocomplete(false)
		return
	}
	if rePathChar.MatchString(char) {
		cur := e.currentLine()
		before := cur[:e.cursorCol]
		if e.isInSubmittedSlashCommandContext() {
			e.tryTriggerAutocomplete(false)
		} else if reAtFile.MatchString(before) {
			e.tryTriggerAutocomplete(false)
		} else if reHashAct.MatchString(before) {
			e.tryTriggerAutocomplete(false)
		} else if reEmoji.MatchString(before) {
			e.tryTriggerAutocomplete(false)
		} else if e.textTriggersURLAutocomplete(before) {
			e.tryTriggerAutocomplete(false)
		}
	}
}

func (e *Editor) textTriggersURLAutocomplete(before string) bool {
	return reURLToken.MatchString(before)
}

func (e *Editor) hasOnlyWhitespaceBeforeCursorLine() bool {
	for i := 0; i < e.cursorLine; i++ {
		if strings.TrimSpace(e.lines[i]) != "" {
			return false
		}
	}
	return true
}

func (e *Editor) isAtStartOfSubmittedMessage() bool {
	cur := e.currentLine()
	before := cur[:e.cursorCol]
	trim := strings.TrimSpace(before)
	return e.hasOnlyWhitespaceBeforeCursorLine() && (trim == "" || trim == "/")
}

func (e *Editor) isInSubmittedSlashCommandContext() bool {
	cur := e.currentLine()
	before := cur[:e.cursorCol]
	return e.hasOnlyWhitespaceBeforeCursorLine() && strings.HasPrefix(strings.TrimLeft(before, " \t"), "/")
}

func (e *Editor) isSlashCommandNameAutocompleteSelection() bool {
	if e.acMode != acRegular {
		return false
	}
	cur := e.currentLine()
	before := strings.TrimLeft(cur[:e.cursorCol], " \t")
	return e.isInSubmittedSlashCommandContext() && strings.HasPrefix(before, "/") && !strings.Contains(before, " ")
}

func (e *Editor) isCompletedSlashCommandAtCursor() bool {
	cur := e.currentLine()
	if e.cursorCol != len(cur) {
		return false
	}
	before := strings.TrimLeft(cur[:e.cursorCol], " \t")
	if !e.isInSubmittedSlashCommandContext() {
		return false
	}
	// /^\/\S+ $/
	if len(before) < 3 || before[0] != '/' {
		return false
	}
	if before[len(before)-1] != ' ' {
		return false
	}
	body := before[1 : len(before)-1]
	return body != "" && !strings.ContainsAny(body, " \t")
}

func (e *Editor) inlineHint() string {
	if e.acMode != acOff && len(e.acItems) > 0 {
		idx := e.acSelected
		if idx >= 0 && idx < len(e.acItems) {
			return e.acItems[idx].Hint
		}
	}
	if p, ok := e.acProvider.(InlineHintProvider); ok {
		return p.GetInlineHint(e.lines, e.cursorLine, e.cursorCol)
	}
	return ""
}

// renderAutocompleteLines paints the dropdown below the editor content.
// Never blocks; uses currently cached items.
func (e *Editor) renderAutocompleteLines(width int) []string {
	if e.acMode == acOff || len(e.acItems) == 0 {
		return nil
	}
	if width < 1 {
		width = 1
	}
	maxVis := e.acMaxVisible
	if maxVis > len(e.acItems) {
		maxVis = len(e.acItems)
	}
	// Window around selection
	start := e.acSelected - maxVis/2
	if start < 0 {
		start = 0
	}
	end := start + maxVis
	if end > len(e.acItems) {
		end = len(e.acItems)
		start = end - maxVis
		if start < 0 {
			start = 0
		}
	}

	slash := strings.HasPrefix(e.acPrefix, "/")
	primaryW := 32
	if slash {
		primaryW = 24
	}
	if primaryW > width-4 {
		primaryW = width - 4
	}
	if primaryW < 8 {
		primaryW = min(8, width)
	}

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		item := e.acItems[i]
		selected := i == e.acSelected
		label := item.Label
		if label == "" {
			label = item.Value
		}
		prefix := "  "
		if selected {
			prefix = "> "
		}
		label = ansitext.TruncateToWidth(label, primaryW, "…")
		pad := primaryW - ansitext.VisibleWidth(label)
		if pad < 0 {
			pad = 0
		}
		row := prefix + label + strings.Repeat(" ", pad)
		if item.Description != "" {
			remain := width - ansitext.VisibleWidth(row) - 1
			if remain > 4 {
				desc := ansitext.TruncateToWidth(item.Description, remain, "…")
				if selected {
					row += " " + "\x1b[2m" + desc + "\x1b[0m"
				} else {
					row += " " + "\x1b[2m" + desc + "\x1b[0m"
				}
			}
		}
		if selected {
			row = "\x1b[7m" + row + "\x1b[0m"
		}
		// Ensure width
		rw := ansitext.VisibleWidth(row)
		if rw < width {
			row += strings.Repeat(" ", width-rw)
		}
		lines = append(lines, row)
	}
	return lines
}

// RankItems is a helper for providers: fuzzy-rank items by label/value.
func RankItems(items []AutocompleteItem, query string) []AutocompleteItem {
	if strings.TrimSpace(query) == "" {
		out := make([]AutocompleteItem, len(items))
		copy(out, items)
		return out
	}
	ranked := fuzzy.Rank(items, query, func(it AutocompleteItem) string {
		if it.Label != "" {
			return it.Label + " " + it.Value
		}
		return it.Value
	})
	out := make([]AutocompleteItem, len(ranked))
	for i, r := range ranked {
		out[i] = r.Item
	}
	return out
}

// StaticProvider is a simple injected provider for tests and slash lists.
type StaticProvider struct {
	Items []AutocompleteItem
	// MatchPrefix when non-empty requires text before cursor to start with it.
	MatchPrefix string
}

// GetSuggestions filters Items by the token before cursor.
func (p *StaticProvider) GetSuggestions(lines []string, cursorLine, cursorCol int) *Suggestions {
	if cursorLine < 0 || cursorLine >= len(lines) {
		return nil
	}
	line := lines[cursorLine]
	if cursorCol > len(line) {
		cursorCol = len(line)
	}
	before := line[:cursorCol]
	prefix := before
	// token = last whitespace-separated piece
	if i := strings.LastIndexAny(before, " \t"); i >= 0 {
		prefix = before[i+1:]
	}
	if p.MatchPrefix != "" && !strings.HasPrefix(prefix, p.MatchPrefix) {
		return nil
	}
	q := prefix
	if p.MatchPrefix != "" {
		q = strings.TrimPrefix(prefix, p.MatchPrefix)
	}
	items := RankItems(p.Items, q)
	if len(items) == 0 {
		return nil
	}
	return &Suggestions{Items: items, Prefix: prefix}
}

// ApplyCompletion replaces the prefix token with item.Value.
func (p *StaticProvider) ApplyCompletion(lines []string, cursorLine, cursorCol int, item AutocompleteItem, prefix string) CompletionResult {
	out := cloneLines(lines)
	if cursorLine < 0 || cursorLine >= len(out) {
		return CompletionResult{Lines: out, CursorLine: 0, CursorCol: 0}
	}
	line := out[cursorLine]
	if cursorCol > len(line) {
		cursorCol = len(line)
	}
	start := cursorCol - len(prefix)
	if start < 0 || (start > 0 && line[start:cursorCol] != prefix) {
		// fallback: find prefix end at cursor
		start = cursorCol
		if len(prefix) > 0 && cursorCol >= len(prefix) && line[cursorCol-len(prefix):cursorCol] == prefix {
			start = cursorCol - len(prefix)
		}
	}
	insert := item.Value
	newLine := line[:start] + insert + line[cursorCol:]
	out[cursorLine] = newLine
	return CompletionResult{
		Lines:      out,
		CursorLine: cursorLine,
		CursorCol:  start + len(insert),
	}
}
