// Package workspace prepares the directory a worker runs in and reports
// what the worker changed there, using plain git commands.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options says where and how to prepare a workspace.
type Options struct {
	// Repo is any path inside the git repository to work on.
	Repo  string
	RunID string
	// InPlace runs in Repo itself instead of a new worktree. Refused on main/master.
	InPlace bool
	// WorktreeRoot is the parent directory for new worktrees. Unused when InPlace.
	WorktreeRoot string
}

// Workspace is a prepared directory plus what it started from.
type Workspace struct {
	Dir    string
	Branch string
	// Base is the commit the worker started from; Changes diffs against it.
	Base string
}

// Changes is what a worker changed relative to Workspace.Base.
type Changes struct {
	Files    []string
	Diffstat string
}

// Prepare creates a worktree on branch delegate/<RunID> from the repo's HEAD,
// or with InPlace returns the repo itself after checking its branch.
// Uncommitted changes in the repo are not copied into a new worktree.
func Prepare(ctx context.Context, opts Options) (Workspace, error) {
	root, err := git(ctx, opts.Repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return Workspace{}, err
	}
	base, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Workspace{}, err
	}

	if opts.InPlace {
		branch, err := git(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return Workspace{}, err
		}
		if branch == "main" || branch == "master" {
			return Workspace{}, fmt.Errorf("refusing to run in place on %q: switch to a feature branch or drop --in-place", branch)
		}
		return Workspace{Dir: root, Branch: branch, Base: base}, nil
	}

	branch := "delegate/" + opts.RunID
	dir := filepath.Join(opts.WorktreeRoot, opts.RunID)
	if _, err := git(ctx, root, "worktree", "add", "-b", branch, dir, base); err != nil {
		return Workspace{}, err
	}
	return Workspace{Dir: dir, Branch: branch, Base: base}, nil
}

// Changes lists files that differ from Base, committed or not, including new
// untracked files, plus git's one-line diffstat. Empty when nothing changed.
func (w Workspace) Changes(ctx context.Context) (Changes, error) {
	// Intent-to-add makes untracked files visible to git diff without staging their content.
	if _, err := git(ctx, w.Dir, "add", "--intent-to-add", "--all"); err != nil {
		return Changes{}, err
	}
	names, err := git(ctx, w.Dir, "diff", "--name-only", w.Base)
	if err != nil {
		return Changes{}, err
	}
	stat, err := git(ctx, w.Dir, "diff", "--shortstat", w.Base)
	if err != nil {
		return Changes{}, err
	}

	var files []string
	if names != "" {
		files = strings.Split(names, "\n")
	}
	return Changes{Files: files, Diffstat: stat}, nil
}

// git runs git in dir and returns its trimmed stdout. On failure the error
// carries git's stderr, which is where git explains what went wrong.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // G204: fixed binary, exec passes args without a shell
	out, err := cmd.Output()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
