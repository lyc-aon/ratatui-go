package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/client"
)

// Version is the omp-tui frontend version string.
const Version = "0.1.0"

// Env keys consumed by the frontend CLI / runtime.
const (
	EnvCoreCommandJSON = "OMP_CORE_COMMAND_JSON"
	EnvCoreCWD         = "OMP_CORE_CWD"
	EnvTrace           = "OMP_TUI_TRACE"
	EnvActive          = "OMP_TUI_ACTIVE" // set by Bun launcher; not re-checked here
	EnvBootstrapJSON   = "OMP_TUI_BOOTSTRAP_JSON"
)

// Config is the fully resolved launch configuration for [App.Run].
type Config struct {
	// Core is the Bun rpc-ui argv. Path is argv[0]; Args are argv[1:].
	Core client.Command

	// ProcessFactory overrides Core process creation. It permits an in-process
	// Go core while preserving the same protocol and lifecycle contract.
	// Nil uses client.DefaultProcessFactory.
	ProcessFactory client.ProcessFactory

	// Trace enables renderer/app diagnostic logging to stderr.
	Trace bool

	// ReadyTimeout bounds client.Start ready wait. Zero → client default.
	ReadyTimeout time.Duration

	// ShutdownTimeout bounds client.Shutdown. Zero → client default.
	ShutdownTimeout time.Duration

	// TickInterval drives working-indicator animation. Zero → 100ms.
	TickInterval time.Duration

	// Stdin / Stdout are preferred TTY files when they are terminals.
	// Nil → os.Stdin / os.Stdout. If not a TTY, runtime opens /dev/tty
	// (or Windows CONIN$/CONOUT$).
	Stdin  *os.File
	Stdout *os.File

	// Stderr is attached to the core child and used for local diagnostics.
	// Nil → os.Stderr.
	Stderr *os.File

	// Bootstrap carries the initial interactive prompt stripped from core argv.
	// Sent once via client.Prompt after ready + get_state/messages/commands.
	Bootstrap Bootstrap

	// ConfigDir is the directory containing user keybindings or configuration.
	ConfigDir string

	// UserKeyBindings overrides default keybindings when provided.
	UserKeyBindings map[string][]string
}

// Bootstrap is the launcher-provided initial interactive payload.
//
// Primary fields (BunGoLauncherPort):
//
//	{"initialMessage"?: string, "initialImages"?: [...], "queuedMessages"?: string[]}
//
// Also accepts legacy aliases: prompt, messages, images, fileArgs.
type Bootstrap struct {
	InitialMessage string   `json:"initialMessage,omitempty"`
	InitialImages  []any    `json:"initialImages,omitempty"`
	QueuedMessages []string `json:"queuedMessages,omitempty"`
	// Legacy aliases
	Prompt   string   `json:"prompt,omitempty"`
	Messages []string `json:"messages,omitempty"`
	Images   []any    `json:"images,omitempty"`
	FileArgs []string `json:"fileArgs,omitempty"`
}

// PrimaryMessage returns initialMessage or legacy prompt.
func (b Bootstrap) PrimaryMessage() string {
	if s := strings.TrimSpace(b.InitialMessage); s != "" {
		return s
	}
	return strings.TrimSpace(b.Prompt)
}

// AllImages merges initialImages and legacy images.
func (b Bootstrap) AllImages() []any {
	if len(b.InitialImages) > 0 {
		return b.InitialImages
	}
	return b.Images
}

// AllQueued returns queuedMessages or legacy messages (not including primary).
func (b Bootstrap) AllQueued() []string {
	if len(b.QueuedMessages) > 0 {
		return b.QueuedMessages
	}
	return b.Messages
}

// HasPrompt reports whether any initial text should be submitted after boot.
func (b Bootstrap) HasPrompt() bool {
	if b.PrimaryMessage() != "" {
		return true
	}
	for _, m := range b.AllQueued() {
		if strings.TrimSpace(m) != "" {
			return true
		}
	}
	return len(b.AllImages()) > 0
}

func (c Config) withDefaults() Config {
	out := c
	if out.Stdin == nil {
		out.Stdin = os.Stdin
	}
	if out.Stdout == nil {
		out.Stdout = os.Stdout
	}
	if out.Stderr == nil {
		out.Stderr = os.Stderr
	}
	if out.TickInterval <= 0 {
		out.TickInterval = 100 * time.Millisecond
	}
	if !out.Trace {
		if v := strings.TrimSpace(os.Getenv(EnvTrace)); v == "1" || strings.EqualFold(v, "true") {
			out.Trace = true
		}
	}
	return out
}

// ParseCoreCommandJSON decodes a JSON argv array into a client.Command.
func ParseCoreCommandJSON(raw string) (client.Command, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return client.Command{}, fmt.Errorf("empty core command JSON")
	}
	var argv []string
	if err := json.Unmarshal([]byte(raw), &argv); err != nil {
		return client.Command{}, fmt.Errorf("parse core command JSON: %w", err)
	}
	if len(argv) == 0 {
		return client.Command{}, fmt.Errorf("core command JSON array is empty")
	}
	for i, a := range argv {
		if strings.TrimSpace(a) == "" {
			return client.Command{}, fmt.Errorf("core command argv[%d] is empty", i)
		}
	}
	cmd := client.Command{Path: argv[0]}
	if len(argv) > 1 {
		cmd.Args = append([]string(nil), argv[1:]...)
	}
	return cmd, nil
}

// ParseBootstrapJSON decodes bootstrap payload from a JSON object string.
// If raw starts with '@', the remainder is a file path to read.
func ParseBootstrapJSON(raw string) (Bootstrap, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Bootstrap{}, nil
	}
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimSpace(strings.TrimPrefix(raw, "@"))
		if path == "" {
			return Bootstrap{}, fmt.Errorf("bootstrap @path is empty")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return Bootstrap{}, fmt.Errorf("read bootstrap file: %w", err)
		}
		raw = string(b)
	}
	var boot Bootstrap
	if err := json.Unmarshal([]byte(raw), &boot); err != nil {
		return Bootstrap{}, fmt.Errorf("parse bootstrap JSON: %w", err)
	}
	return boot, nil
}

// CLIFlags is the parsed argv surface for cmd/omp-tui.
type CLIFlags struct {
	ShowVersion     bool
	Trace           bool
	CoreCommandJSON string
	CoreCWD         string
	CoreEnv         []string
	NoCoreEnv       bool
	BootstrapJSON   string
	Help            bool
	Unknown         []string
	ParseErr        error
}

// ParseArgs parses omp-tui argv (excluding the program name).
func ParseArgs(args []string) CLIFlags {
	var f CLIFlags
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--version" || a == "-V":
			f.ShowVersion = true
			i++
		case a == "--help" || a == "-h":
			f.Help = true
			i++
		case a == "--trace":
			f.Trace = true
			i++
		case a == "--no-core-env":
			f.NoCoreEnv = true
			i++
		case a == "--core-command-json":
			if i+1 >= len(args) {
				f.ParseErr = fmt.Errorf("--core-command-json requires a value")
				return f
			}
			f.CoreCommandJSON = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--core-command-json="):
			f.CoreCommandJSON = strings.TrimPrefix(a, "--core-command-json=")
			i++
		case a == "--core-cwd":
			if i+1 >= len(args) {
				f.ParseErr = fmt.Errorf("--core-cwd requires a value")
				return f
			}
			f.CoreCWD = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--core-cwd="):
			f.CoreCWD = strings.TrimPrefix(a, "--core-cwd=")
			i++
		case a == "--core-env":
			if i+1 >= len(args) {
				f.ParseErr = fmt.Errorf("--core-env requires KEY=VAL")
				return f
			}
			kv := args[i+1]
			if !strings.Contains(kv, "=") {
				f.ParseErr = fmt.Errorf("--core-env value must be KEY=VAL, got %q", kv)
				return f
			}
			f.CoreEnv = append(f.CoreEnv, kv)
			i += 2
		case strings.HasPrefix(a, "--core-env="):
			kv := strings.TrimPrefix(a, "--core-env=")
			if !strings.Contains(kv, "=") {
				f.ParseErr = fmt.Errorf("--core-env value must be KEY=VAL, got %q", kv)
				return f
			}
			f.CoreEnv = append(f.CoreEnv, kv)
			i++
		case a == "--bootstrap-json":
			if i+1 >= len(args) {
				f.ParseErr = fmt.Errorf("--bootstrap-json requires a value")
				return f
			}
			f.BootstrapJSON = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--bootstrap-json="):
			f.BootstrapJSON = strings.TrimPrefix(a, "--bootstrap-json=")
			i++
		case a == "--":
			i++
			f.Unknown = append(f.Unknown, args[i:]...)
			return f
		default:
			f.Unknown = append(f.Unknown, a)
			i++
		}
	}
	return f
}

// ResolveConfig builds a Config from CLI flags + environment.
func ResolveConfig(f CLIFlags) (Config, error) {
	if f.ParseErr != nil {
		return Config{}, f.ParseErr
	}
	if len(f.Unknown) > 0 {
		return Config{}, fmt.Errorf("unknown arguments: %s", strings.Join(f.Unknown, " "))
	}

	raw := strings.TrimSpace(f.CoreCommandJSON)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(EnvCoreCommandJSON))
	}
	if raw == "" {
		return Config{}, fmt.Errorf("core command required: pass --core-command-json or set %s", EnvCoreCommandJSON)
	}
	core, err := ParseCoreCommandJSON(raw)
	if err != nil {
		return Config{}, err
	}

	cwd := strings.TrimSpace(f.CoreCWD)
	if cwd == "" {
		cwd = strings.TrimSpace(os.Getenv(EnvCoreCWD))
	}
	core.Dir = cwd

	if f.NoCoreEnv {
		core.Env = append([]string(nil), f.CoreEnv...)
	} else if len(f.CoreEnv) > 0 {
		core.Env = append(append([]string(nil), os.Environ()...), f.CoreEnv...)
	}

	bootRaw := strings.TrimSpace(f.BootstrapJSON)
	if bootRaw == "" {
		bootRaw = strings.TrimSpace(os.Getenv(EnvBootstrapJSON))
	}
	boot, err := ParseBootstrapJSON(bootRaw)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Core:      core,
		Trace:     f.Trace,
		Bootstrap: boot,
	}
	return cfg.withDefaults(), nil
}

// Usage returns the CLI help text.
func Usage() string {
	return `omp-tui — OMP Go frontend (TTY owner)

Usage:
  omp-tui [flags]

Flags:
  --core-command-json <json-array>   Bun core argv (required unless env set)
  --core-cwd <dir>                   Working directory for the core process
  --core-env KEY=VAL                 Extra env var for core (repeatable)
  --no-core-env                      Do not inherit process env; use only --core-env
  --bootstrap-json <json|@file>      Initial prompt payload
  --trace                            Diagnostic logging to stderr
  --version, -V                      Print version and exit
  --help, -h                         Show this help

Environment:
  OMP_CORE_COMMAND_JSON     Same as --core-command-json
  OMP_CORE_CWD              Same as --core-cwd
  OMP_TUI_TRACE=1           Same as --trace
  OMP_TUI_BOOTSTRAP_JSON    Same as --bootstrap-json
  OMP_TUI_ACTIVE=1          Set by Bun launcher (recursion guard; not read here)

Topology:
  Bun launcher → omp-tui (owns TTY) → spawns Bun core --mode rpc-ui on pipes.
  stdin/stdout should be the real controlling TTY when possible; if not a TTY,
  omp-tui opens /dev/tty (Windows CONIN$/CONOUT$). Core never shares the TTY.

Bootstrap JSON:
  {"initialMessage"?: string, "initialImages"?: [...], "queuedMessages"?: string[]}
`
}
