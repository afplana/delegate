package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/afplana/delegate/internal/history"
	"github.com/afplana/delegate/internal/workspace"
)

// runClean implements `delegate clean [--days N] [--logs] [--yes]`. Without
// --yes it only prints what it would remove.
func runClean(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(stderr)
	days := fs.Int("days", 7, "only runs that started more than this many days ago")
	logs := fs.Bool("logs", false, "also remove run log dirs (delegate stats reads them)")
	yes := fs.Bool("yes", false, "remove; without it, only show what would be removed")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	state, err := stateDir()
	if err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}
	cutoff := time.Now().Add(-time.Duration(*days) * 24 * time.Hour)
	stale, err := workspace.FindStale(ctx, filepath.Join(state, "worktrees"), cutoff)
	if err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}
	runsDir := filepath.Join(state, "runs")
	var oldLogs []string
	if *logs {
		if oldLogs, err = history.OlderThan(runsDir, cutoff); err != nil {
			fmt.Fprintf(stderr, "delegate: %v\n", err)
			return 1
		}
	}
	if len(stale) == 0 && len(oldLogs) == 0 {
		fmt.Fprintf(stdout, "nothing to clean older than %d days\n", *days)
		return 0
	}

	var total int64
	removable, skipped := 0, 0
	for _, s := range stale {
		if s.Problem != "" {
			skipped++
		} else {
			removable++
			total += s.Bytes
		}
		label := fmt.Sprintf("worktree %s (%s, %s, branch %s, repo %s)",
			s.RunID, s.Started.Format(time.DateOnly), humanBytes(s.Bytes), s.Branch, s.Repo)
		switch {
		case s.Problem != "":
			fmt.Fprintf(stdout, "skip   worktree %s: %s\n", s.RunID, s.Problem)
		case !*yes:
			fmt.Fprintf(stdout, "would remove %s\n", label)
		default:
			if err := workspace.RemoveStale(ctx, s); err != nil {
				fmt.Fprintf(stdout, "kept   worktree %s: %v\n", s.RunID, err)
				continue
			}
			fmt.Fprintf(stdout, "removed %s\n", label)
		}
	}

	if len(oldLogs) > 0 {
		if !*yes {
			fmt.Fprintf(stdout, "would remove %d run log dir(s) older than %d days\n", len(oldLogs), *days)
		} else {
			removed := 0
			for _, id := range oldLogs {
				if err := os.RemoveAll(filepath.Join(runsDir, id)); err != nil {
					fmt.Fprintf(stdout, "kept   run log %s: %v\n", id, err)
					continue
				}
				removed++
			}
			fmt.Fprintf(stdout, "removed %d run log dir(s)\n", removed)
		}
	}

	if !*yes {
		fmt.Fprintf(stdout, "%d worktree(s) to remove, %s; %d skipped. Run again with --yes to remove.\n", removable, humanBytes(total), skipped)
	}
	return 0
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, suffix := float64(n), "KMGT"
	i := -1
	for value >= unit && i < len(suffix)-1 {
		value /= unit
		i++
	}
	return fmt.Sprintf("%.1f %cB", value, suffix[i])
}
