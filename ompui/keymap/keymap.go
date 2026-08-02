package keymap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/event"
)

// Definition defines one configurable keybinding.
type Definition struct {
	DefaultKeys []string
	Description string
}

// Registry manages keybinding definitions, user overrides, and matching.
type Registry struct {
	definitions map[string]Definition
	userKeys    map[string][]string
	resolved    map[string][]string
	matchKeys   map[string]map[string]struct{}
}

// DefaultDefinitions returns all OMP default keybindings (app.* and tui.*).
func DefaultDefinitions() map[string]Definition {
	pasteImage := []string{"ctrl+v"}
	if runtime.GOOS == "windows" {
		pasteImage = []string{"ctrl+v", "alt+v"}
	}

	return map[string]Definition{
		// TUI editor navigation & editing
		"tui.editor.cursorUp":           {DefaultKeys: []string{"up"}, Description: "Move cursor up"},
		"tui.editor.cursorDown":         {DefaultKeys: []string{"down"}, Description: "Move cursor down"},
		"tui.editor.cursorLeft":         {DefaultKeys: []string{"left", "ctrl+b"}, Description: "Move cursor left"},
		"tui.editor.cursorRight":        {DefaultKeys: []string{"right", "ctrl+f"}, Description: "Move cursor right"},
		"tui.editor.cursorWordLeft":     {DefaultKeys: []string{"alt+left", "ctrl+left", "alt+b"}, Description: "Move cursor word left"},
		"tui.editor.cursorWordRight":    {DefaultKeys: []string{"alt+right", "ctrl+right", "alt+f"}, Description: "Move cursor word right"},
		"tui.editor.cursorLineStart":    {DefaultKeys: []string{"home", "ctrl+a"}, Description: "Move to line start"},
		"tui.editor.cursorLineEnd":      {DefaultKeys: []string{"end", "ctrl+e"}, Description: "Move to line end"},
		"tui.editor.jumpForward":        {DefaultKeys: []string{"ctrl+]"}, Description: "Jump forward to character"},
		"tui.editor.jumpBackward":       {DefaultKeys: []string{"ctrl+alt+]"}, Description: "Jump backward to character"},
		"tui.editor.pageUp":             {DefaultKeys: []string{"pageup"}, Description: "Page up"},
		"tui.editor.pageDown":           {DefaultKeys: []string{"pagedown"}, Description: "Page down"},
		"tui.editor.deleteCharBackward": {DefaultKeys: []string{"backspace"}, Description: "Delete character backward"},
		"tui.editor.deleteCharForward":  {DefaultKeys: []string{"delete", "ctrl+d"}, Description: "Delete character forward"},
		"tui.editor.deleteWordBackward": {DefaultKeys: []string{"ctrl+w", "alt+backspace", "ctrl+backspace", "super+alt+backspace"}, Description: "Delete word backward"},
		"tui.editor.deleteWordForward":  {DefaultKeys: []string{"alt+delete", "alt+d", "super+alt+delete", "super+alt+d"}, Description: "Delete word forward"},
		"tui.editor.deleteToLineStart":  {DefaultKeys: []string{"ctrl+u"}, Description: "Delete to line start"},
		"tui.editor.deleteToLineEnd":    {DefaultKeys: []string{"ctrl+k"}, Description: "Delete to line end"},
		"tui.editor.yank":               {DefaultKeys: []string{"ctrl+y"}, Description: "Yank"},
		"tui.editor.yankPop":            {DefaultKeys: []string{"alt+y"}, Description: "Yank pop"},
		"tui.editor.undo":               {DefaultKeys: []string{"ctrl+-", "ctrl+_"}, Description: "Undo"},

		// TUI generic input & selection
		"tui.input.newLine":   {DefaultKeys: []string{"shift+enter", "ctrl+j"}, Description: "Insert newline"},
		"tui.input.submit":    {DefaultKeys: []string{"enter"}, Description: "Submit input"},
		"tui.input.tab":       {DefaultKeys: []string{"tab"}, Description: "Tab / autocomplete"},
		"tui.input.copy":      {DefaultKeys: []string{"ctrl+c"}, Description: "Copy selection"},
		"tui.select.up":       {DefaultKeys: []string{"up"}, Description: "Move selection up"},
		"tui.select.down":     {DefaultKeys: []string{"down"}, Description: "Move selection down"},
		"tui.select.pageUp":   {DefaultKeys: []string{"pageup"}, Description: "Selection page up"},
		"tui.select.pageDown": {DefaultKeys: []string{"pagedown"}, Description: "Selection page down"},
		"tui.select.confirm":  {DefaultKeys: []string{"enter"}, Description: "Confirm selection"},
		"tui.select.cancel":   {DefaultKeys: []string{"escape", "ctrl+c"}, Description: "Cancel selection"},

		// App actions
		"app.interrupt":              {DefaultKeys: []string{"escape"}, Description: "Interrupt current operation"},
		"app.clear":                  {DefaultKeys: []string{"ctrl+c"}, Description: "Clear screen or cancel"},
		"app.exit":                   {DefaultKeys: []string{"ctrl+d"}, Description: "Exit application"},
		"app.suspend":                {DefaultKeys: []string{"ctrl+z"}, Description: "Suspend application"},
		"app.display.reset":          {DefaultKeys: []string{"ctrl+l"}, Description: "Reset terminal display"},
		"app.thinking.cycle":         {DefaultKeys: []string{"shift+tab"}, Description: "Cycle thinking level"},
		"app.thinking.toggle":        {DefaultKeys: []string{"ctrl+t"}, Description: "Toggle thinking mode"},
		"app.model.cycleForward":     {DefaultKeys: []string{"ctrl+p"}, Description: "Cycle to next model"},
		"app.model.cycleBackward":    {DefaultKeys: []string{"shift+ctrl+p"}, Description: "Cycle to previous model"},
		"app.model.select":           {DefaultKeys: []string{"alt+m"}, Description: "Select model"},
		"app.model.selectTemporary":  {DefaultKeys: []string{"alt+p"}, Description: "Select temporary model for current session"},
		"app.tools.expand":           {DefaultKeys: []string{"ctrl+o"}, Description: "Expand tools"},
		"app.editor.external":        {DefaultKeys: []string{"ctrl+g"}, Description: "Open external editor"},
		"app.message.followUp":       {DefaultKeys: []string{"ctrl+q", "ctrl+enter"}, Description: "Send follow-up message"},
		"app.retry":                  {DefaultKeys: []string{"alt+r"}, Description: "Retry last failed assistant turn"},
		"app.message.dequeue":        {DefaultKeys: []string{"alt+up"}, Description: "Dequeue message"},
		"app.clipboard.pasteImage":   {DefaultKeys: pasteImage, Description: "Paste image or text from clipboard"},
		"app.clipboard.pasteTextRaw": {DefaultKeys: []string{"ctrl+shift+v", "alt+shift+v"}, Description: "Paste text from clipboard as raw text"},
		"app.clipboard.copyLine":     {DefaultKeys: []string{"alt+shift+l"}, Description: "Copy current line"},
		"app.clipboard.copyPrompt":   {DefaultKeys: []string{"alt+shift+c"}, Description: "Copy prompt"},
		"app.agents.hub":             {DefaultKeys: []string{"alt+a"}, Description: "Open the agent hub"},
		"app.session.observe":        {DefaultKeys: []string{"ctrl+s"}, Description: "Observe/switch active session"},
		"app.plan.toggle":            {DefaultKeys: []string{"alt+shift+p"}, Description: "Toggle plan mode"},
		"app.history.search":         {DefaultKeys: []string{"ctrl+r"}, Description: "Search history"},
	}
}

// NewRegistry constructs a keybinding registry with default OMP definitions.
func NewRegistry() *Registry {
	r := &Registry{
		definitions: DefaultDefinitions(),
		userKeys:    make(map[string][]string),
		resolved:    make(map[string][]string),
		matchKeys:   make(map[string]map[string]struct{}),
	}
	r.rebuild()
	return r
}

// Rebuild updates resolved bindings and lookup structures.
func (r *Registry) rebuild() {
	r.resolved = make(map[string][]string, len(r.definitions))
	r.matchKeys = make(map[string]map[string]struct{}, len(r.definitions))

	for id, def := range r.definitions {
		keys, ok := r.userKeys[id]
		if !ok || len(keys) == 0 {
			keys = def.DefaultKeys
		}
		r.resolved[id] = append([]string(nil), keys...)

		m := make(map[string]struct{})
		for _, k := range keys {
			event.AddKeyAliases(m, k)
		}
		r.matchKeys[id] = m
	}
}

// SetUserBindings sets custom key mappings (e.g. from keybindings.yml/json).
func (r *Registry) SetUserBindings(userBindings map[string][]string) {
	r.userKeys = make(map[string][]string, len(userBindings))
	for k, v := range userBindings {
		r.userKeys[k] = append([]string(nil), v...)
	}
	r.rebuild()
}

// Matches reports whether an event.Key matches the given keybinding action id.
func (r *Registry) Matches(k event.Key, action string) bool {
	if k.Action == event.ActionRelease {
		return false
	}
	m, ok := r.matchKeys[action]
	if !ok {
		return false
	}
	actual := event.CanonicalKeyID(k.ID)
	if actual == "" {
		actual = event.FormatKeyID(k.Mods, string(k.Code))
	}
	if _, found := m[actual]; found {
		return true
	}
	// Check shifted symbol alias
	if event.IsShiftedSymbol(actual) {
		if _, found := m["shift+"+actual]; found {
			return true
		}
	}
	return false
}

// GetKeys returns the resolved key list for an action.
func (r *Registry) GetKeys(action string) []string {
	if keys, ok := r.resolved[action]; ok {
		return append([]string(nil), keys...)
	}
	return nil
}

// GetDisplayString returns formatted keys like "Ctrl+C/Esc" for hints.
func (r *Registry) GetDisplayString(action string) string {
	keys := r.GetKeys(action)
	if len(keys) == 0 {
		return ""
	}
	formatted := make([]string, len(keys))
	for i, k := range keys {
		formatted[i] = FormatKeyHint(k)
	}
	return strings.Join(formatted, "/")
}

// FormatKeyHint formats a key ID for display (e.g. "ctrl+shift+p" -> "Ctrl+Shift+P").
func FormatKeyHint(key string) string {
	parts := strings.Split(key, "+")
	for i, p := range parts {
		lower := strings.ToLower(p)
		switch lower {
		case "ctrl":
			parts[i] = "Ctrl"
		case "shift":
			parts[i] = "Shift"
		case "alt":
			parts[i] = "Alt"
		case "super":
			parts[i] = "Super"
		case "escape", "esc":
			parts[i] = "Esc"
		case "enter", "return":
			parts[i] = "Enter"
		case "backspace":
			parts[i] = "Backspace"
		case "delete":
			parts[i] = "Delete"
		case "space":
			parts[i] = "Space"
		case "tab":
			parts[i] = "Tab"
		case "up":
			parts[i] = "Up"
		case "down":
			parts[i] = "Down"
		case "left":
			parts[i] = "Left"
		case "right":
			parts[i] = "Right"
		case "pageup":
			parts[i] = "PgUp"
		case "pagedown":
			parts[i] = "PgDn"
		default:
			if len(p) == 1 {
				parts[i] = strings.ToUpper(p)
			} else if len(p) > 1 {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
	}
	return strings.Join(parts, "+")
}

// LoadConfig loads user keybindings from YAML, JSON, or line-based config files.
func LoadConfig(dir string) (map[string][]string, error) {
	paths := []string{
		filepath.Join(dir, "keybindings.yml"),
		filepath.Join(dir, "keybindings.yaml"),
		filepath.Join(dir, "keybindings.json"),
	}
	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, nil // No config file is okay
	}

	result := make(map[string][]string)

	// Try JSON first
	var jsonMap map[string]any
	if json.Unmarshal(data, &jsonMap) == nil {
		for k, v := range jsonMap {
			switch val := v.(type) {
			case string:
				result[k] = []string{val}
			case []any:
				var strList []string
				for _, item := range val {
					if s, ok := item.(string); ok {
						strList = append(strList, s)
					}
				}
				if len(strList) > 0 {
					result[k] = strList
				}
			}
		}
		return result, nil
	}

	// Simple YAML / KV fallback line parser
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			val = strings.Trim(val, "[]")
			items := strings.Split(val, ",")
			var strList []string
			for _, item := range items {
				item = strings.Trim(strings.TrimSpace(item), `"'`)
				if item != "" {
					strList = append(strList, item)
				}
			}
			if len(strList) > 0 {
				result[key] = strList
			}
		} else {
			val = strings.Trim(val, `"'`)
			if val != "" {
				result[key] = []string{val}
			}
		}
	}

	return result, nil
}
