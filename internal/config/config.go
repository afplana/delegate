// Package config decides which worker profiles exist. v1 ships them built in;
// a YAML file will be merged over Builtin later without callers changing.
package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/afplana/delegate/internal/runner"
)

// DefaultProfile is used when the caller does not pass --profile.
const DefaultProfile = "cursor"

const defaultTimeout = 30 * time.Minute

// Builtin returns the profiles delegate knows without any config file.
// It returns a fresh map on every call so callers cannot change shared state.
func Builtin() map[string]runner.Profile {
	return map[string]runner.Profile{
		"cursor": {
			Name:    "cursor",
			Command: "cursor-agent",
			// No --force: org-managed Cursor accounts can disable it ("Run Everything"),
			// and print mode already allows file edits without it.
			Args:    []string{"-p", "--trust", "--output-format", "text", "{prompt}"},
			Timeout: defaultTimeout,
		},
		"claude-sonnet": {
			Name:    "claude-sonnet",
			Command: "claude",
			Args:    claudeArgs("sonnet"),
			Timeout: defaultTimeout,
		},
		// For fully specified, mechanical work: renames, sweeps, exact edits.
		"claude-haiku": {
			Name:    "claude-haiku",
			Command: "claude",
			Args:    claudeArgs("haiku"),
			Timeout: defaultTimeout,
		},
	}
}

// claudeAllowedTools are the shell commands a headless Claude worker may run.
// acceptEdits only approves file edits, and print mode denies anything that
// would prompt, so without this list the worker cannot build or test. Pushing
// stays with the planner. Allow rules cannot lift Claude's "sensitive file"
// denial (e.g. .pre-commit-config.yaml): the planner writes those files itself.
var claudeAllowedTools = []string{
	"Bash(go *)",
	"Bash(gofmt *)",
	"Bash(make *)",
	"Bash(golangci-lint *)",
	"Bash(gotestsum *)",
	"Bash(git add *)",
	"Bash(git commit *)",
	"Bash(git status *)",
	"Bash(git diff *)",
	"Bash(which *)",
}

// claudeArgs builds the args for a headless Claude worker on model.
// --setting-sources project and --strict-mcp-config keep the user's personal
// setup (output style, hooks, MCP servers) out of the worker; the repo's own
// settings still apply. Not --bare: it also skips OAuth and keychain auth.
func claudeArgs(model string) []string {
	args := []string{
		"-p", "--model", model, "--permission-mode", "acceptEdits",
		"--setting-sources", "project", "--strict-mcp-config",
		"--allowedTools",
	}
	args = append(args, claudeAllowedTools...)
	// --allowedTools is variadic: without "--" it would swallow the prompt.
	return append(args, "--", "{prompt}")
}

// Lookup returns the profile called name, or an error naming the ones that exist.
func Lookup(profiles map[string]runner.Profile, name string) (runner.Profile, error) {
	profile, ok := profiles[name]
	if !ok {
		known := slices.Sorted(maps.Keys(profiles))
		return runner.Profile{}, fmt.Errorf("unknown profile %q (known: %s)", name, strings.Join(known, ", "))
	}
	return profile, nil
}
