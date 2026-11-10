package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runIDLayout is the timestamp prefix of every run id (see newRunID).
const runIDLayout = "20060102-150405"

// Stale is a leftover worktree from an old run.
type Stale struct {
	RunID   string
	Dir     string
	Repo    string // source repository; empty when Problem is set
	Branch  string
	Started time.Time
	Bytes   int64
	// Problem says why git cannot remove it; such entries are only reported.
	Problem string
}

// FindStale lists worktrees under root whose run started before cutoff.
// The start time comes from the run id, falling back to the directory's
// modification time. A missing root means nothing to clean.
func FindStale(ctx context.Context, root string, cutoff time.Time) ([]Stale, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read worktrees: %w", err)
	}
	var stale []Stale
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		started, ok := startTime(entry)
		if !ok || !started.Before(cutoff) {
			continue
		}
		s := Stale{RunID: entry.Name(), Dir: dir, Started: started, Bytes: dirSize(dir)}
		if err := s.locate(ctx); err != nil {
			s.Problem = err.Error()
		}
		stale = append(stale, s)
	}
	return stale, nil
}

// locate fills in the source repo and branch from git's own records.
func (s *Stale) locate(ctx context.Context) error {
	// A worktree has a .git file; without one, git would walk up to some
	// unrelated parent repository.
	pointer, err := os.ReadFile(filepath.Join(s.Dir, ".git"))
	if err != nil {
		return errors.New("not a git worktree (no .git); remove it by hand if unwanted")
	}
	if gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(pointer)), "gitdir: "); ok {
		if _, err := os.Stat(gitdir); err != nil { //nolint:gosec // G703: existence check only; nothing is read or written there
			return fmt.Errorf("its source repo is gone (%s); remove it by hand if unwanted", gitdir)
		}
	}
	common, err := git(ctx, s.Dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("source repo not found: %w", err)
	}
	branch, err := git(ctx, s.Dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	s.Repo, s.Branch = filepath.Dir(common), branch
	return nil
}

// RemoveStale removes the worktree and then its branch, never forcing either:
// git refuses a worktree with uncommitted changes and an unmerged branch, and
// both are kept. The returned error says what was kept and why.
func RemoveStale(ctx context.Context, s Stale) error {
	if s.Problem != "" {
		return errors.New(s.Problem)
	}
	if _, err := git(ctx, s.Repo, "worktree", "remove", s.Dir); err != nil {
		return fmt.Errorf("kept worktree: %w", err)
	}
	if _, err := git(ctx, s.Repo, "branch", "-d", s.Branch); err != nil {
		return fmt.Errorf("removed worktree, kept branch %s: %w", s.Branch, err)
	}
	return nil
}

func startTime(entry fs.DirEntry) (time.Time, bool) {
	name := entry.Name()
	if len(name) >= len(runIDLayout) {
		if t, err := time.Parse(runIDLayout, name[:len(runIDLayout)]); err == nil {
			return t, true
		}
	}
	info, err := entry.Info()
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// dirSize sums file sizes under dir; unreadable entries count as zero.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best effort: a size estimate, not a check
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}
