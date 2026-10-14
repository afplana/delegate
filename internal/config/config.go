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

// DefaultProfile is used when the caller does not pass --profile. Not cursor:
// org-managed Cursor accounts can disable --force, which that profile needs.
const DefaultProfile = "claude-sonnet"

const defaultTimeout = 30 * time.Minute

// Builtin returns the profiles delegate knows without any config file.
// It returns a fresh map on every call so callers cannot change shared state.
func Builtin() map[string]runner.Profile {
	return map[string]runner.Profile{
		"cursor": {
			Name:    "cursor",
			Command: "cursor-agent",
			Args:    []string{"-p", "--force", "--trust", "--output-format", "text", "{prompt}"},
			Timeout: defaultTimeout,
		},
		"claude-sonnet": {
			Name:    "claude-sonnet",
			Command: "claude",
			Args:    []string{"-p", "--model", "sonnet", "--permission-mode", "acceptEdits", "{prompt}"},
			Timeout: defaultTimeout,
		},
	}
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
