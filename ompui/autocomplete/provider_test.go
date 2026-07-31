package autocomplete_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/autocomplete"
	"github.com/lyc-aon/ratatui-go/ompui/editor"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

func boolVal(v bool) *bool { return &v }

func TestSlashCommandCompletion(t *testing.T) {
	t.Parallel()
	p := autocomplete.New(autocomplete.Options{
		BasePath: t.TempDir(),
		Commands: []autocomplete.SlashCommand{
			{Name: "help", Description: "show help", Aliases: []string{"h"}},
			{Name: "model", Description: "set model", ArgumentHint: "<name>"},
			{Name: "clear", Description: "clear session"},
		},
		Files: &autocomplete.StaticFileSource{},
	})
	sugs := p.GetSuggestions([]string{"/he"}, 0, 3)
	if sugs == nil || len(sugs.Items) == 0 {
		t.Fatal("expected slash suggestions")
	}
	found := false
	for _, it := range sugs.Items {
		if strings.Contains(it.Value, "help") || strings.Contains(it.Label, "help") {
			found = true
		}
	}
	if !found {
		t.Fatalf("help not in %+v", sugs.Items)
	}

	item := sugs.Items[0]
	res := p.ApplyCompletion([]string{"/he"}, 0, 3, item, sugs.Prefix)
	if len(res.Lines) == 0 {
		t.Fatal("empty apply")
	}
}

func TestCommandsFromModel(t *testing.T) {
	t.Parallel()
	cmds := autocomplete.CommandsFromModel([]model.AvailableCommand{
		{Name: "foo", Description: "Foo", Aliases: []string{"f"}},
		{Name: "bar", Input: []byte(`{"hint":"<x>"}`), Subcommands: []byte(`[{"name":"sub","description":"S"}]`)},
	})
	if len(cmds) != 2 || cmds[0].Name != "foo" {
		t.Fatalf("%+v", cmds)
	}
	p := autocomplete.New(autocomplete.Options{BasePath: t.TempDir(), Files: &autocomplete.StaticFileSource{}})
	p.SetModelCommands([]model.AvailableCommand{{Name: "zzz"}})
	sugs := p.GetSuggestions([]string{"/z"}, 0, 2)
	if sugs == nil || len(sugs.Items) == 0 {
		t.Fatal("model commands not installed")
	}
}

func TestPathCompletionRespectsRootBounds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("src/main.go", "package main")
	mustWrite("src/util.go", "package util")
	mustWrite("README.md", "hi")
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := &autocomplete.StaticFileSource{
		DiscoverRoot: root,
		Dirs: map[string][]autocomplete.FileEntry{
			root: {
				{Name: "src", RelPath: "src/", IsDir: true},
				{Name: "README.md", RelPath: "README.md"},
			},
			filepath.Join(root, "src"): {
				{Name: "main.go", RelPath: "src/main.go"},
				{Name: "util.go", RelPath: "src/util.go"},
			},
		},
		Tree: []autocomplete.FileEntry{
			{Name: "README.md", RelPath: "README.md"},
			{Name: "main.go", RelPath: "src/main.go"},
			{Name: "util.go", RelPath: "src/util.go"},
			{Name: "src", RelPath: "src/", IsDir: true},
		},
	}

	p := autocomplete.New(autocomplete.Options{
		BasePath:         root,
		Files:            src,
		AllowOutsideRoot: boolVal(false),
	})

	sugs := p.GetSuggestions([]string{"@REA"}, 0, 4)
	if sugs != nil {
		for _, it := range sugs.Items {
			if strings.Contains(it.Value, outside) || strings.Contains(it.Label, "secret") {
				t.Fatalf("leaked outside root: %+v", it)
			}
		}
	}

	sugs = p.GetSuggestions([]string{"@src/"}, 0, 5)
	if sugs != nil {
		for _, it := range sugs.Items {
			val := it.Value + " " + it.Label
			if strings.Contains(val, "secret") {
				t.Fatalf("path escape: %+v", it)
			}
		}
	}

	// Real FS source: discover must not surface outside temp root.
	p2 := autocomplete.New(autocomplete.Options{
		BasePath:         root,
		Files:            autocomplete.NewFSSource(),
		AllowOutsideRoot: boolVal(false),
		MaxDepth:         4,
		MaxResults:       20,
		MaxScanEntries:   200,
	})
	sugs = p2.GetSuggestions([]string{"@"}, 0, 1)
	if sugs != nil {
		for _, it := range sugs.Items {
			if strings.Contains(it.Value, "secret.txt") || strings.Contains(it.Label, "secret.txt") {
				t.Fatalf("outside file leaked: %+v", it)
			}
		}
	}
}

func TestFSSourceListDirAndDiscover(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "root.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := autocomplete.NewFSSource()
	ctx := context.Background()
	ents, err := fs.ListDir(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) == 0 {
		t.Fatal("ListDir empty")
	}

	found, err := fs.Discover(ctx, root, "a.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range found {
		if strings.Contains(e.RelPath, ".git") {
			t.Fatalf("discover entered .git: %+v", e)
		}
	}

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = fs.Discover(cctx, root, "")
}

func TestStaleRequestIDCancel(t *testing.T) {
	t.Parallel()
	p := autocomplete.New(autocomplete.Options{
		BasePath: t.TempDir(),
		Commands: []autocomplete.SlashCommand{{Name: "x"}},
		Files:    &autocomplete.StaticFileSource{},
	})
	before := p.SnapshotRequestID()
	_ = p.GetSuggestions([]string{"/x"}, 0, 2)
	after := p.SnapshotRequestID()
	if after < before {
		t.Fatalf("req id went backwards %d -> %d", before, after)
	}
	p.Cancel()
	_ = p.GetSuggestions([]string{"/x"}, 0, 2)
}

func TestForceFileAndInlineHint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "file.txt"), []byte("1"), 0o644)
	p := autocomplete.New(autocomplete.Options{
		BasePath: root,
		Commands: []autocomplete.SlashCommand{
			{
				Name:         "cmd",
				ArgumentHint: "<path>",
				InlineHint:   func(arg string) string { return "hint:" + arg },
			},
		},
		Files: autocomplete.NewFSSource(),
	})
	_ = p.ShouldTriggerFileCompletion([]string{"hello @"}, 0, 7)
	_ = p.GetForceFileSuggestions([]string{"@"}, 0, 1)
	_ = p.GetInlineHint([]string{"/cmd "}, 0, 5)
}

func TestFindLeadingSlash(t *testing.T) {
	t.Parallel()
	if autocomplete.FindLeadingSlashCommandStart("/help") != 0 {
		t.Fatal("expected 0")
	}
	if autocomplete.FindLeadingSlashCommandStart("  /x") < 0 {
		t.Fatal("whitespace slash")
	}
	if autocomplete.FindLeadingSlashCommandStart("nope") != -1 {
		t.Fatal("non-slash")
	}
}

func TestStaticFileSource(t *testing.T) {
	t.Parallel()
	s := &autocomplete.StaticFileSource{
		Dirs: map[string][]autocomplete.FileEntry{
			"/proj": {{Name: "a", RelPath: "a", IsDir: false}},
		},
		Tree: []autocomplete.FileEntry{
			{Name: "a", RelPath: "a"},
		},
		DiscoverRoot: "/proj",
	}
	ents, err := s.ListDir(context.Background(), "/proj")
	if err != nil || len(ents) != 1 {
		t.Fatalf("list=%v err=%v", ents, err)
	}
	out, err := s.Discover(context.Background(), "/proj", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("discover=%v", out)
	}
}

var (
	_ editor.AutocompleteProvider = (*autocomplete.Provider)(nil)
	_ editor.InlineHintProvider   = (*autocomplete.Provider)(nil)
	_ editor.SyncSlashProvider    = (*autocomplete.Provider)(nil)
	_ editor.ForceFileProvider    = (*autocomplete.Provider)(nil)
)
