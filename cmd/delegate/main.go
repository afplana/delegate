// Command delegate hands a written plan to a worker agent CLI and reports a compact result.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"
)

var version = "dev"

func main() {
	// Ctrl-C cancels the context, which stops the worker process too.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := runCommands(ctx, os.Args[1:], os.Stdout, os.Stderr, config.Builtin())
	stop()
	os.Exit(code)
}

// runCommands is the testable entry point: no os.Exit, no globals for I/O,
// and the profiles are passed in so tests can use a fake worker.
func runCommands(ctx context.Context, args []string, stdout, stderr io.Writer, profiles map[string]runner.Profile) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: delegate <run|profiles|version>")
		return 2
	}
	switch args[0] {
	case "run":
		return runTask(ctx, args[1:], stdout, stderr, profiles)
	case "profiles":
		return listProfiles(stdout, profiles)
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	default:
		fmt.Fprintf(stderr, "delegate: unknown command %q\n", args[0])
		return 2
	}
}
