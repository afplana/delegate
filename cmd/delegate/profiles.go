package main

import (
	"fmt"
	"io"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"
)

// listProfiles prints each profile with the path of its binary, or "missing"
// when the binary is not on PATH. A missing binary is not a failure.
func listProfiles(stdout io.Writer, cfg config.Config) int {
	profiles := cfg.Profiles
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tCOMMAND\tBINARY")
	for _, name := range slices.Sorted(maps.Keys(profiles)) {
		profile := profiles[name]
		binary, err := exec.LookPath(profile.Command)
		if err != nil {
			binary = "missing"
		}
		label := name
		if name == cfg.Default {
			label += " (default)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", label, profile.Command, binary)
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}

// findCommand returns the path of profile's binary. When it is not on PATH the
// error names the profiles whose binaries are, so the planner (or user) can
// pick one; delegate never switches profile on its own, because cost, model
// and permissions differ between them. isDefault says the profile was not
// chosen with --profile.
func findCommand(cfg config.Config, profile runner.Profile, isDefault bool) (string, error) {
	path, err := exec.LookPath(profile.Command)
	if err == nil {
		return path, nil
	}
	which := "profile"
	if isDefault {
		which = "default profile"
	}
	var installed []string
	for _, name := range slices.Sorted(maps.Keys(cfg.Profiles)) {
		if _, err := exec.LookPath(cfg.Profiles[name].Command); err == nil {
			installed = append(installed, name)
		}
	}
	var hint string
	switch {
	case len(installed) == 0:
		hint = "no profile's command is on PATH: install an agent CLI or add a profile in " + userConfigFile()
	case isDefault:
		hint = fmt.Sprintf("installed profiles: %s; pass --profile <name> or set default_profile in %s", strings.Join(installed, ", "), userConfigFile())
	default:
		hint = fmt.Sprintf("installed profiles: %s; pass --profile <name>", strings.Join(installed, ", "))
	}
	return "", fmt.Errorf("%s %q: command %q not on PATH; %s", which, profile.Name, profile.Command, hint)
}
