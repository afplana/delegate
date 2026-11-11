// Command delegate hands a written plan to a worker agent CLI and reports a compact result.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"

	"github.com/afplana/delegate/internal/config"
)

// version is set with -ldflags "-X main.version=..." by `make build`.
var version = "dev"

func main() {
	// Ctrl-C cancels the context, which stops the worker process too.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	cfg, err := config.Defaults().WithUserFile(userConfigFile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "delegate: %v\n", err)
		os.Exit(1)
	}
	code := runCommands(ctx, os.Args[1:], os.Stdout, os.Stderr, cfg)
	stop()
	os.Exit(code)
}

// runCommands is the testable entry point: no os.Exit, no globals for I/O,
// and the config is passed in so tests can use a fake worker.
func runCommands(ctx context.Context, args []string, stdout, stderr io.Writer, cfg config.Config) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: delegate <run|review|stats|clean|profiles|doctor|skill|version>")
		return 2
	}
	switch args[0] {
	case "run":
		return runTask(ctx, args[1:], stdout, stderr, cfg)
	case "review":
		return runReview(args[1:], stdout, stderr)
	case "stats":
		return runStats(stdout, stderr)
	case "clean":
		return runClean(ctx, args[1:], stdout, stderr)
	case "profiles":
		return listProfiles(stdout, cfg)
	case "doctor":
		return runDoctor(stdout, cfg)
	case "skill":
		return runSkill(args[1:], stdout, stderr)
	case "version":
		var module string
		if info, ok := debug.ReadBuildInfo(); ok {
			module = info.Main.Version
		}
		fmt.Fprintln(stdout, resolveVersion(version, module))
		return 0
	default:
		fmt.Fprintf(stderr, "delegate: unknown command %q\n", args[0])
		return 2
	}
}

// userConfigFile is $XDG_CONFIG_HOME/delegate/config.yaml, falling back to
// ~/.config/delegate/config.yaml.
func userConfigFile() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "delegate", "config.yaml")
}

// resolveVersion prefers the -ldflags version, then the module version Go
// records for `go install ...@<tag>`. "(devel)" means a local build.
func resolveVersion(ldflag, module string) string {
	if ldflag != "dev" || module == "" || module == "(devel)" {
		return ldflag
	}
	return module
}
