package main

import (
	"fmt"
	"io"
	"maps"
	"os/exec"
	"slices"
	"text/tabwriter"

	"github.com/afplana/delegate/internal/config"
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
