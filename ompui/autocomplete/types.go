package autocomplete

import (
	"context"

	"github.com/lyc-aon/ratatui-go/ompui/editor"
	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// Item is one dropdown candidate (OMP AutocompleteItem).
type Item = editor.AutocompleteItem

// Suggestions is a provider response.
type Suggestions = editor.Suggestions

// Subcommand is one declarative slash subcommand (OMP SubcommandDef / RPC shape).
type Subcommand struct {
	Name        string
	Description string
	Usage       string
}

// SlashCommand is a provider-side command entry (OMP SlashCommand).
//
// ArgumentCompletions and InlineHint are optional hooks. When building from
// [model.AvailableCommand], subcommands and input.hint are materialized into
// these hooks automatically.
type SlashCommand struct {
	Name        string
	Aliases     []string
	Description string
	// ArgumentHint is static hint text shown in the description column
	// (OMP argumentHint / input.hint).
	ArgumentHint string
	// GetAutocompleteDescription, when set, supplies a live description for
	// the dropdown. Static Description remains the search corpus.
	GetAutocompleteDescription func() string
	// ArgumentCompletions returns argument-phase items for text after the
	// command name and first space. nil / empty means no argument UI.
	ArgumentCompletions func(argumentPrefix string) []Item
	// InlineHint returns dim ghost text for the current argument state.
	InlineHint func(argumentText string) string
}

// FileEntry is one filesystem candidate.
type FileEntry struct {
	// RelPath uses forward slashes relative to the walk root (or absolute when
	// listing outside the configured base). Directories end with "/".
	RelPath string
	// Name is the final path segment (no trailing slash).
	Name string
	// IsDir reports whether the entry is a directory (or symlink-to-dir).
	IsDir bool
}

// FileSource supplies path candidates. Implementations MUST honor ctx
// cancellation, MUST NOT shell out, and MUST NOT mutate the process cwd.
type FileSource interface {
	// ListDir returns immediate children of absDir for prefix completion.
	// Names are base names; IsDir is set; RelPath may be empty (caller builds
	// the display path from the user's typed prefix).
	ListDir(ctx context.Context, absDir string) ([]FileEntry, error)
	// Discover walks root for fuzzy @ completion. query may be empty (list
	// shallow). Results are already ranked best-first and capped.
	Discover(ctx context.Context, root, query string) ([]FileEntry, error)
}

// Options configures a [Provider].
type Options struct {
	// Commands are slash entries. Prefer SetCommands / CommandsFromModel for
	// live model.AvailableCommand updates.
	Commands []SlashCommand
	// BasePath is the project root used for relative path completion.
	// Empty defaults to the process working directory at construction time
	// (captured once; never mutated later).
	BasePath string
	// HomeDir overrides the user home used for ~ expansion. Empty uses os.UserHomeDir.
	HomeDir string
	// Files injects a custom file source. nil uses the production bounded FS source.
	Files FileSource
	// AllowOutsideRoot permits absolute, ~/ and ../ prefix listing outside BasePath.
	// Recursive fuzzy discovery still refuses to walk outside BasePath.
	// Default true (OMP behavior).
	AllowOutsideRoot *bool
	// MaxResults caps fuzzy discovery results (default 100).
	MaxResults int
	// MaxScanEntries caps total filesystem entries visited per discover (default 5000).
	MaxScanEntries int
	// MaxDepth caps directory recursion depth from the walk root (default 12).
	MaxDepth int
}

// Default scan bounds (OMP fuzzyFind maxResults=100 plus Go safety caps).
const (
	DefaultMaxResults     = 100
	DefaultMaxScanEntries = 5000
	DefaultMaxDepth       = 12
)

// CommandsFromModel maps core-advertised commands into provider slash entries.
// Subcommands JSON and input.hint are materialized into argument completions
// and inline hints. Order is preserved (registry order).
func CommandsFromModel(cmds []model.AvailableCommand) []SlashCommand {
	out := make([]SlashCommand, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, slashFromModel(c))
	}
	return out
}
