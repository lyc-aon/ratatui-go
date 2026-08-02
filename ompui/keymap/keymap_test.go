package keymap_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/event"
	"github.com/lyc-aon/ratatui-go/ompui/keymap"
)

func TestKeymapDefaultsAndMatching(t *testing.T) {
	reg := keymap.NewRegistry()

	// Verify default bindings
	tests := []struct {
		action   string
		keyID    string
		expected bool
	}{
		{"app.suspend", "ctrl+z", true},
		{"app.model.select", "alt+m", true},
		{"app.model.selectTemporary", "alt+p", true},
		{"app.thinking.toggle", "ctrl+t", true},
		{"app.history.search", "ctrl+r", true},
		{"app.retry", "alt+r", true},
		{"app.message.dequeue", "alt+up", true},
		{"app.editor.external", "ctrl+g", true},
		{"app.agents.hub", "alt+a", true},
		{"app.session.observe", "ctrl+s", true},
		{"app.plan.toggle", "alt+shift+p", true},
		{"app.model.cycleBackward", "shift+ctrl+p", true},
		{"app.model.cycleForward", "ctrl+p", true},
		{"app.suspend", "ctrl+x", false},
	}

	for _, tt := range tests {
		k := event.Key{ID: tt.keyID, Action: event.ActionPress}
		matched := reg.Matches(k, tt.action)
		if matched != tt.expected {
			t.Errorf("Matches(%s, %s) = %v; want %v", tt.keyID, tt.action, matched, tt.expected)
		}
	}
}

func TestUserOverrides(t *testing.T) {
	reg := keymap.NewRegistry()
	reg.SetUserBindings(map[string][]string{
		"app.suspend": {"ctrl+x"},
	})

	// Original binding should no longer match
	kOld := event.Key{ID: "ctrl+z", Action: event.ActionPress}
	if reg.Matches(kOld, "app.suspend") {
		t.Errorf("Matches(ctrl+z, app.suspend) after override = true; want false")
	}

	// New binding should match
	kNew := event.Key{ID: "ctrl+x", Action: event.ActionPress}
	if !reg.Matches(kNew, "app.suspend") {
		t.Errorf("Matches(ctrl+x, app.suspend) after override = false; want true")
	}
}

func TestFormatDisplayString(t *testing.T) {
	reg := keymap.NewRegistry()
	str := reg.GetDisplayString("app.suspend")
	if str != "Ctrl+Z" {
		t.Errorf("GetDisplayString(app.suspend) = %q; want \"Ctrl+Z\"", str)
	}

	strMulti := reg.GetDisplayString("app.message.followUp")
	if strMulti != "Ctrl+Q/Ctrl+Enter" {
		t.Errorf("GetDisplayString(app.message.followUp) = %q; want \"Ctrl+Q/Ctrl+Enter\"", strMulti)
	}
}

func TestLoadConfigJSON(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `{"app.suspend": "ctrl+x", "app.retry": ["alt+r", "ctrl+r"]}`
	if err := os.WriteFile(filepath.Join(dir, "keybindings.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	userMap, err := keymap.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	reg := keymap.NewRegistry()
	reg.SetUserBindings(userMap)

	if !reg.Matches(event.Key{ID: "ctrl+x", Action: event.ActionPress}, "app.suspend") {
		t.Errorf("User config json app.suspend failed")
	}
	if !reg.Matches(event.Key{ID: "ctrl+r", Action: event.ActionPress}, "app.retry") {
		t.Errorf("User config json app.retry multi failed")
	}
}

func TestLoadConfigYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := "app.suspend: ctrl+y\napp.plan.toggle: [alt+p, ctrl+alt+p]\n"
	if err := os.WriteFile(filepath.Join(dir, "keybindings.yml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	userMap, err := keymap.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	reg := keymap.NewRegistry()
	reg.SetUserBindings(userMap)

	if !reg.Matches(event.Key{ID: "ctrl+y", Action: event.ActionPress}, "app.suspend") {
		t.Errorf("User config yaml app.suspend failed")
	}
	if !reg.Matches(event.Key{ID: "ctrl+alt+p", Action: event.ActionPress}, "app.plan.toggle") {
		t.Errorf("User config yaml app.plan.toggle list failed")
	}
}
