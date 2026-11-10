package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/afplana/delegate/skills"
)

// runSkill implements `delegate skill install [--force]`.
func runSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "install" {
		fmt.Fprintln(stderr, "usage: delegate skill install [--force]")
		return 2
	}
	fs := flag.NewFlagSet("skill install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "overwrite an installed skill that has changes delegate did not make")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	dir := skillDir()
	outcome, err := skills.Install(dir, *force)
	if err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s %s\n", outcome, filepath.Join(dir, "SKILL.md"))
	return 0
}

// skillDir is where Claude Code looks for the skill: $CLAUDE_CONFIG_DIR, or
// ~/.claude when unset.
func skillDir() string {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "skills", "delegate")
}
