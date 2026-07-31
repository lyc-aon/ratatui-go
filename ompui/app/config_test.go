package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaelkelly/ratatui-go/ompui/app"
)

func TestParseArgsHelpVersionTrace(t *testing.T) {
	t.Parallel()
	f := app.ParseArgs([]string{"--help"})
	if !f.Help || f.ParseErr != nil {
		t.Fatalf("%+v", f)
	}
	f = app.ParseArgs([]string{"-V"})
	if !f.ShowVersion {
		t.Fatal("version")
	}
	f = app.ParseArgs([]string{"--trace", "--core-command-json", `["bun","x"]`})
	if !f.Trace || f.CoreCommandJSON == "" {
		t.Fatalf("%+v", f)
	}
	f = app.ParseArgs([]string{"--core-command-json"})
	if f.ParseErr == nil {
		t.Fatal("expected missing value error")
	}
	f = app.ParseArgs([]string{"--core-env", "NOPE"})
	if f.ParseErr == nil {
		t.Fatal("expected KEY=VAL error")
	}
	f = app.ParseArgs([]string{"--core-env", "A=1", "--core-env=B=2", "mystery"})
	if len(f.CoreEnv) != 2 || len(f.Unknown) != 1 {
		t.Fatalf("%+v", f)
	}
}

func TestParseCoreCommandJSON(t *testing.T) {
	t.Parallel()
	cmd, err := app.ParseCoreCommandJSON(`["/bin/bun","run","core","--mode","rpc-ui"]`)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != "/bin/bun" || len(cmd.Args) != 4 || cmd.Args[0] != "run" {
		t.Fatalf("%+v", cmd)
	}
	if _, err := app.ParseCoreCommandJSON(``); err == nil {
		t.Fatal("empty")
	}
	if _, err := app.ParseCoreCommandJSON(`[]`); err == nil {
		t.Fatal("empty array")
	}
	if _, err := app.ParseCoreCommandJSON(`["ok",""]`); err == nil {
		t.Fatal("empty argv elem")
	}
	if _, err := app.ParseCoreCommandJSON(`not-json`); err == nil {
		t.Fatal("bad json")
	}
}

func TestParseBootstrapJSONPrimaryQueuedLegacy(t *testing.T) {
	t.Parallel()
	b, err := app.ParseBootstrapJSON(`{"initialMessage":"hi","queuedMessages":["a","b"],"initialImages":[1]}`)
	if err != nil {
		t.Fatal(err)
	}
	if b.PrimaryMessage() != "hi" {
		t.Fatal(b.PrimaryMessage())
	}
	if len(b.AllQueued()) != 2 || len(b.AllImages()) != 1 {
		t.Fatalf("%+v", b)
	}
	if !b.HasPrompt() {
		t.Fatal("HasPrompt")
	}

	legacy, err := app.ParseBootstrapJSON(`{"prompt":"p","messages":["m"],"images":["i"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.PrimaryMessage() != "p" || legacy.AllQueued()[0] != "m" {
		t.Fatalf("%+v", legacy)
	}

	empty, err := app.ParseBootstrapJSON("")
	if err != nil || empty.HasPrompt() {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}

	// @file form
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.json")
	if err := os.WriteFile(path, []byte(`{"initialMessage":"from-file"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fromFile, err := app.ParseBootstrapJSON("@" + path)
	if err != nil {
		t.Fatal(err)
	}
	if fromFile.PrimaryMessage() != "from-file" {
		t.Fatal(fromFile.PrimaryMessage())
	}
	if _, err := app.ParseBootstrapJSON("@"); err == nil {
		t.Fatal("empty @path")
	}
	if _, err := app.ParseBootstrapJSON("@/no/such/file"); err == nil {
		t.Fatal("missing file")
	}
}

func TestResolveConfigFromFlagsAndEnv(t *testing.T) {
	// env mutations — not parallel
	t.Setenv(app.EnvCoreCommandJSON, "")
	t.Setenv(app.EnvCoreCWD, "")
	t.Setenv(app.EnvBootstrapJSON, "")
	t.Setenv(app.EnvTrace, "")

	_, err := app.ResolveConfig(app.CLIFlags{})
	if err == nil || !strings.Contains(err.Error(), "core command") {
		t.Fatalf("err=%v", err)
	}

	cfg, err := app.ResolveConfig(app.CLIFlags{
		CoreCommandJSON: `["bun","core"]`,
		CoreCWD:         "/tmp/proj",
		CoreEnv:         []string{"K=V"},
		NoCoreEnv:       true,
		BootstrapJSON:   `{"initialMessage":"boot"}`,
		Trace:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Core.Path != "bun" || cfg.Core.Dir != "/tmp/proj" {
		t.Fatalf("%+v", cfg.Core)
	}
	if len(cfg.Core.Env) != 1 || cfg.Core.Env[0] != "K=V" {
		t.Fatalf("env=%v", cfg.Core.Env)
	}
	if !cfg.Trace || cfg.Bootstrap.PrimaryMessage() != "boot" {
		t.Fatalf("%+v", cfg)
	}

	// Env fallbacks
	t.Setenv(app.EnvCoreCommandJSON, `["env-bun"]`)
	t.Setenv(app.EnvCoreCWD, "/env/cwd")
	t.Setenv(app.EnvBootstrapJSON, `{"prompt":"env-prompt"}`)
	t.Setenv(app.EnvTrace, "1")
	cfg, err = app.ResolveConfig(app.CLIFlags{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Core.Path != "env-bun" || cfg.Core.Dir != "/env/cwd" {
		t.Fatalf("%+v", cfg.Core)
	}
	if cfg.Bootstrap.PrimaryMessage() != "env-prompt" {
		t.Fatal(cfg.Bootstrap.PrimaryMessage())
	}
	if !cfg.Trace {
		t.Fatal("trace env")
	}

	// Unknown args fail resolve
	_, err = app.ResolveConfig(app.CLIFlags{Unknown: []string{"x"}, CoreCommandJSON: `["a"]`})
	if err == nil {
		t.Fatal("unknown args")
	}
}

func TestUsageAndVersionConstants(t *testing.T) {
	t.Parallel()
	u := app.Usage()
	for _, needle := range []string{"omp-tui", "--help", "--version", "OMP_CORE_COMMAND_JSON", "bootstrap"} {
		if !strings.Contains(u, needle) {
			t.Fatalf("usage missing %q", needle)
		}
	}
	if app.Version == "" {
		t.Fatal("empty version")
	}
	if app.ExitOK != 0 || app.ExitError != 1 || app.ExitInterrupted != 130 {
		t.Fatalf("exit codes %d %d %d", app.ExitOK, app.ExitError, app.ExitInterrupted)
	}
}

func TestEqualsFormFlags(t *testing.T) {
	t.Parallel()
	f := app.ParseArgs([]string{
		"--core-command-json=[\"c\"]",
		"--core-cwd=/w",
		"--bootstrap-json={\"initialMessage\":\"z\"}",
	})
	if f.CoreCommandJSON != `["c"]` || f.CoreCWD != "/w" {
		t.Fatalf("%+v", f)
	}
	if f.BootstrapJSON != `{"initialMessage":"z"}` {
		t.Fatalf("boot=%q", f.BootstrapJSON)
	}
}
