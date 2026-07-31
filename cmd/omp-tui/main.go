// Command omp-tui is the OMP Go frontend process.
//
// Topology: Bun launcher → omp-tui (TTY owner) → spawns Bun core --mode rpc-ui
// on pipes. See ompui/app for the event loop.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/michaelkelly/ratatui-go/ompui/app"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := app.ParseArgs(args)
	if flags.ParseErr != nil {
		fmt.Fprintf(os.Stderr, "omp-tui: %v\n\n%s", flags.ParseErr, app.Usage())
		return app.ExitError
	}
	if flags.Help {
		fmt.Fprint(os.Stdout, app.Usage())
		return app.ExitOK
	}
	if flags.ShowVersion {
		fmt.Fprintf(os.Stdout, "omp-tui %s\n", app.Version)
		return app.ExitOK
	}

	cfg, err := app.ResolveConfig(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "omp-tui: %v\n\n%s", err, app.Usage())
		return app.ExitError
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg)
	code := application.Run(ctx)
	if err := application.Err(); err != nil && cfg.Trace {
		fmt.Fprintf(os.Stderr, "omp-tui: exit error: %v\n", err)
	}
	return code
}
