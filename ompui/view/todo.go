package view

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// maxTodoRows bounds the rendered task list. A plan longer than this is a plan
// you scan, not read, so the overflow collapses into a count.
const maxTodoRows = 10

// TodoStatus is a task's lifecycle state.
type TodoStatus uint8

const (
	// TodoPending is not started.
	TodoPending TodoStatus = iota
	// TodoActive is in progress.
	TodoActive
	// TodoDone is completed.
	TodoDone
	// TodoAbandoned was dropped without completing.
	TodoAbandoned
)

func parseTodoStatus(value string) TodoStatus {
	switch value {
	case "in_progress":
		return TodoActive
	case "completed":
		return TodoDone
	case "abandoned":
		return TodoAbandoned
	default:
		return TodoPending
	}
}

// TodoItem is one task.
type TodoItem struct {
	Phase   string
	Content string
	Status  TodoStatus
}

// wireTodo covers both shapes the core sends: a phase list
// (`[{name, tasks:[…]}]`) and a bare task list (`[{content, status}]`).
type wireTodo struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Status  string `json:"status"`
	Tasks   []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	} `json:"tasks"`
}

// ParseTodos flattens the raw todo payload into an ordered task list. Both the
// phase-grouped and flat encodings are accepted; anything else yields nil
// rather than a guess.
func ParseTodos(raw json.RawMessage) []TodoItem {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var entries []wireTodo
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	out := make([]TodoItem, 0, len(entries))
	for _, entry := range entries {
		if len(entry.Tasks) > 0 {
			for _, task := range entry.Tasks {
				out = append(out, TodoItem{
					Phase:   entry.Name,
					Content: task.Content,
					Status:  parseTodoStatus(task.Status),
				})
			}
			continue
		}
		if entry.Content == "" {
			continue
		}
		out = append(out, TodoItem{
			Phase:   entry.Name,
			Content: entry.Content,
			Status:  parseTodoStatus(entry.Status),
		})
	}
	return out
}

// countPendingTodos counts tasks still open: not completed and not abandoned.
func countPendingTodos(items []TodoItem) int {
	n := 0
	for _, item := range items {
		if item.Status != TodoDone && item.Status != TodoAbandoned {
			n++
		}
	}
	return n
}

// TodoBlockRows renders a transcript plan block as Hermes' TodoPanel: a chevron
// header carrying the progress count, then the tasks indented under it. Returns
// nil when the message is not a plan block, so callers can chain it.
//
// A plan survives every detail mode. It is the operator's own checklist, not the
// model's private reasoning, so /details never hides it.
func (r Renderer) TodoBlockRows(msg model.Message, width int) []string {
	extras := readExtras(msg)
	items := ParseTodos(extras.Todos)
	if len(items) == 0 {
		return nil
	}
	return r.TodoPanelRows(items, width, extras.TodoCollapsedByDefault, extras.TodoIncomplete)
}

// TodoPanelRows renders a plan under a chevron header. A collapsed panel shows
// only the header — the count is the whole summary a settled plan needs.
func (r Renderer) TodoPanelRows(items []TodoItem, width int, collapsed, incomplete bool) []string {
	if len(items) == 0 {
		return nil
	}
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)

	done := 0
	for _, item := range items {
		if item.Status == TodoDone {
			done++
		}
	}
	pending := countPendingTodos(items)

	header := apply(r.theme.Accent, detailChevron(r.theme.Symbols, !collapsed)+" ") +
		apply(r.theme.Bold, apply(r.theme.Text, "Todo")) +
		apply(r.theme.Dim, " ("+strconv.Itoa(done)+"/"+strconv.Itoa(len(items))+")")
	if incomplete && pending > 0 {
		state := "pending/in_progress"
		if pending == 1 {
			state = "pending"
		}
		header += apply(r.theme.Dim, r.theme.Symbols.Sep+"incomplete"+r.theme.Symbols.Sep+
			strconv.Itoa(pending)+" still "+state)
	}
	out := []string{inset + fit(header, layout.Body, r.theme.Symbols.Ellipsis)}
	if collapsed {
		return out
	}

	body := inset + "  "
	for _, item := range items {
		mark, style := r.todoGlyph(item.Status)
		text := flattenLine(item.Content)
		if item.Status == TodoDone || item.Status == TodoAbandoned {
			text = apply(r.theme.Dim, text)
		} else {
			text = apply(r.theme.Text, text)
		}
		out = append(out, fit(body+apply(style, mark)+" "+text, layout.Width, r.theme.Symbols.Ellipsis))
	}
	return out
}

func (r Renderer) todoGlyph(status TodoStatus) (string, StyleFunc) {
	sym := r.theme.Symbols
	switch status {
	case TodoActive:
		return sym.CheckActive, r.theme.Accent
	case TodoDone:
		return sym.CheckDone, r.theme.Success
	case TodoAbandoned:
		return sym.CheckAbandoned, r.theme.Dim
	default:
		return sym.CheckPending, r.theme.Muted
	}
}

// TodoRows renders the task list: a progress header, then the tasks in plan
// order behind status marks.
//
// When the list outgrows [maxTodoRows] the completed tasks are dropped first —
// they are already summarized in the header count, and what the reader needs on
// screen is the work that is still open.
func (r Renderer) TodoRows(items []TodoItem, width int) []string {
	if len(items) == 0 {
		return nil
	}
	layout := r.opts.layout(width)
	inset := padding(layout.Inset)

	done := 0
	for _, item := range items {
		if item.Status == TodoDone {
			done++
		}
	}

	visible := items
	if len(visible) > maxTodoRows {
		open := make([]TodoItem, 0, len(items))
		for _, item := range items {
			if item.Status != TodoDone {
				open = append(open, item)
			}
		}
		visible = open
	}
	hidden := 0
	if len(visible) > maxTodoRows {
		hidden = len(visible) - maxTodoRows
		visible = visible[:maxTodoRows]
	}

	header := apply(r.theme.Muted, apply(r.theme.Bold, "Todo")) +
		apply(r.theme.Dim, "  "+strconv.Itoa(done)+"/"+strconv.Itoa(len(items))+" done")
	out := make([]string, 0, len(visible)+3)
	out = append(out, inset+fit(header, layout.Body, r.theme.Symbols.Ellipsis))

	phase := ""
	multiPhase := false
	for _, item := range items {
		if item.Phase != "" && item.Phase != items[0].Phase {
			multiPhase = true
			break
		}
	}

	indent := inset + " "
	for _, item := range visible {
		if multiPhase && item.Phase != phase {
			phase = item.Phase
			out = append(out, indent+apply(r.theme.Dim, phase))
		}
		mark, style := r.todoGlyph(item.Status)
		body := indent
		if multiPhase {
			body += " "
		}
		text := flattenLine(item.Content)
		if item.Status == TodoDone {
			text = apply(r.theme.Dim, text)
		} else {
			text = apply(r.theme.Text, text)
		}
		out = append(out, fit(body+apply(style, mark)+" "+text, layout.Width, r.theme.Symbols.Ellipsis))
	}
	if hidden > 0 {
		out = append(out, indent+apply(r.theme.Dim,
			r.theme.Symbols.Ellipsis+" "+strconv.Itoa(hidden)+" more open "+pluralize("task", hidden)))
	}
	return out
}

// TodoSummary renders the session's task list.
type TodoSummary struct {
	widget
	items []TodoItem
}

// NewTodoSummary constructs a todo summary.
func NewTodoSummary(theme Theme, opts Options) *TodoSummary {
	return &TodoSummary{widget: newWidget(theme, opts)}
}

// SetItems installs an explicit task list.
func (t *TodoSummary) SetItems(items []TodoItem) { t.items = items }

// SetSnapshot parses the task list out of a snapshot, preferring the live
// session state and falling back to the last todo event.
func (t *TodoSummary) SetSnapshot(snap model.Snapshot) {
	if items := ParseTodos(snap.Session.TodoPhases); len(items) > 0 {
		t.items = items
		return
	}
	t.items = ParseTodos(snap.TodoPhases)
}

// Items returns the current task list.
func (t *TodoSummary) Items() []TodoItem { return t.items }

// Render implements component.Component.
func (t *TodoSummary) Render(width int) component.Frame {
	return t.frame(width, t.r.TodoRows(t.items, width))
}
