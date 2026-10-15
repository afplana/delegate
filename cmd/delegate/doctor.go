package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"
)

// runDoctor checks git, the state dir, and the default profile binary; it prints
// one line per check and returns 1 if any check fails.
func runDoctor(stdout io.Writer, profiles map[string]runner.Profile) int {
	checks := []struct {
		name  string
		check func() (string, error)
	}{
		{
			name: "git",
			check: func() (string, error) {
				return exec.LookPath("git")
			},
		},
		{
			name: "state dir",
			check: func() (string, error) {
				dir, err := stateDir()
				if err != nil {
					return "", err
				}
				if err := checkWritable(dir); err != nil {
					return "", fmt.Errorf("%s not writable: %w", dir, err)
				}
				return dir, nil
			},
		},
		{
			name: "default profile",
			check: func() (string, error) {
				profile, err := config.Lookup(profiles, config.DefaultProfile)
				if err != nil {
					return "", err
				}
				path, err := exec.LookPath(profile.Command)
				if err != nil {
					return "", fmt.Errorf("profile %q: command %q not on PATH: %w", profile.Name, profile.Command, err)
				}
				return fmt.Sprintf("%s: %s", profile.Name, path), nil
			},
		},
	}

	failed := false
	for _, c := range checks {
		detail, err := c.check()
		if err != nil {
			fmt.Fprintf(stdout, "%-6s%s: %s\n", "FAIL", c.name, err)
			failed = true
			continue
		}
		fmt.Fprintf(stdout, "%-6s%s: %s\n", "ok", c.name, detail)
	}
	if failed {
		return 1
	}
	return 0
}

// checkWritable creates dir if needed and proves files can be written there
// by creating and removing a temporary file.
func checkWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(f.Name())
}
