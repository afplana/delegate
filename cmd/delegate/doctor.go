package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/skills"
)

// warning is a check result worth showing that does not make doctor fail.
type warning string

func (w warning) Error() string { return string(w) }

// runDoctor checks git, the state dir, the default profile binary and the
// installed skill; it prints one line per check and returns 1 if any check
// fails. Warnings are printed but do not fail.
func runDoctor(stdout io.Writer, cfg config.Config) int {
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
				profile, err := config.Lookup(cfg.Profiles, cfg.Default)
				if err != nil {
					return "", err
				}
				path, err := findCommand(cfg, profile, true)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s: %s", profile.Name, path), nil
			},
		},
		{
			// A stale copy still works, just with older instructions: warn.
			name: "skill",
			check: func() (string, error) {
				dir := skillDir()
				if !skills.InstalledMatches(dir) {
					return "", warning(fmt.Sprintf("%s is missing or differs from this binary's; run delegate skill install", filepath.Join(dir, "SKILL.md")))
				}
				return filepath.Join(dir, "SKILL.md"), nil
			},
		},
	}

	failed := false
	for _, c := range checks {
		detail, err := c.check()
		if w, ok := errors.AsType[warning](err); ok {
			fmt.Fprintf(stdout, "%-6s%s: %s\n", "WARN", c.name, w)
			continue
		}
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
