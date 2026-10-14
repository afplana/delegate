package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"
	"github.com/afplana/delegate/internal/workspace"
)

// result is the JSON contract printed on stdout. Keep it small: the caller is
// usually a model paying for every token. Full output lives in LogDir.
type result struct {
	RunID        string   `json:"run_id"`
	Profile      string   `json:"profile"`
	Status       string   `json:"status"` // ok | failed | timeout | error
	ExitCode     int      `json:"exit_code"`
	DurationMS   int64    `json:"duration_ms"`
	Workdir      string   `json:"workdir,omitempty"`
	Branch       string   `json:"branch,omitempty"`
	Base         string   `json:"base,omitempty"`
	FilesChanged []string `json:"files_changed"`
	Diffstat     string   `json:"diffstat"`
	Summary      string   `json:"summary"`
	LogDir       string   `json:"log_dir,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type runFlags struct {
	task, profile, repo string
	inPlace             bool
	timeout             time.Duration
}

// runTask implements `delegate run`. Flag mistakes print usage and return 2.
// Anything after that prints a JSON result, so the caller always gets one.
func runTask(ctx context.Context, args []string, stdout, stderr io.Writer, profiles map[string]runner.Profile) int {
	var f runFlags
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.task, "task", "", "file with the plan for the worker (required)")
	fs.StringVar(&f.profile, "profile", config.DefaultProfile, "worker profile")
	fs.StringVar(&f.repo, "repo", ".", "git repository to work on")
	fs.BoolVar(&f.inPlace, "in-place", false, "work in --repo itself instead of a new worktree (refused on main/master)")
	fs.DurationVar(&f.timeout, "timeout", 0, "stop the worker after this long (default: the profile's)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if f.task == "" {
		fmt.Fprintln(stderr, "delegate run: --task is required")
		fs.Usage()
		return 2
	}

	res := execute(ctx, f, profiles)
	line, err := json.Marshal(res)
	if err != nil {
		fmt.Fprintf(stderr, "delegate: encode result: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(line))
	if res.LogDir != "" {
		if err := os.WriteFile(filepath.Join(res.LogDir, "result.json"), line, 0o600); err != nil {
			fmt.Fprintf(stderr, "delegate: save result: %v\n", err)
		}
	}
	if res.Status != "ok" {
		return 1
	}
	return 0
}

// execute does one run. Failures of delegate itself become status "error".
func execute(ctx context.Context, f runFlags, profiles map[string]runner.Profile) result {
	start := time.Now()
	res := result{RunID: newRunID(start), Profile: f.profile, FilesChanged: []string{}}
	fail := func(err error) result {
		res.Status = "error"
		res.Error = err.Error()
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}

	profile, err := config.Lookup(profiles, f.profile)
	if err != nil {
		return fail(err)
	}
	if f.timeout > 0 {
		profile.Timeout = f.timeout
	}
	plan, err := os.ReadFile(f.task)
	if err != nil {
		return fail(fmt.Errorf("read task: %w", err))
	}

	state, err := stateDir()
	if err != nil {
		return fail(err)
	}
	logDir := filepath.Join(state, "runs", res.RunID)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fail(fmt.Errorf("create log dir: %w", err))
	}
	res.LogDir = logDir

	ws, err := workspace.Prepare(ctx, workspace.Options{
		Repo:         f.repo,
		RunID:        res.RunID,
		InPlace:      f.inPlace,
		WorktreeRoot: filepath.Join(state, "worktrees"),
	})
	if err != nil {
		return fail(err)
	}
	res.Workdir, res.Branch, res.Base = ws.Dir, ws.Branch, ws.Base

	prompt := string(plan) + runner.SummaryInstruction
	if err := writeLogs(logDir, map[string]string{"prompt.md": prompt}); err != nil {
		return fail(err)
	}
	out, err := runner.Run(ctx, profile, runner.Task{Prompt: prompt, Dir: ws.Dir})
	if err != nil {
		return fail(err)
	}
	if err := writeLogs(logDir, map[string]string{"stdout.log": out.Output, "stderr.log": out.Stderr}); err != nil {
		return fail(err)
	}

	changes, err := ws.Changes(ctx)
	if err != nil {
		return fail(err)
	}
	if changes.Files != nil {
		res.FilesChanged = changes.Files
	}
	res.Diffstat = changes.Diffstat
	res.ExitCode = out.ExitCode
	res.Summary = runner.ExtractSummary(out.Output)
	res.Status = status(out)
	res.DurationMS = time.Since(start).Milliseconds()
	return res
}

func status(out runner.Result) string {
	switch {
	case out.TimedOut:
		return "timeout"
	case out.ExitCode != 0:
		return "failed"
	default:
		return "ok"
	}
}

// newRunID is sortable by time and unique enough for one user's runs.
func newRunID(now time.Time) string {
	suffix := make([]byte, 2)
	_, _ = rand.Read(suffix) // crypto/rand.Read never returns an error
	return now.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(suffix)
}

// stateDir follows the XDG base directory spec: $XDG_STATE_HOME/delegate,
// falling back to ~/.local/state/delegate.
func stateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "delegate"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find state dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "delegate"), nil
}

func writeLogs(dir string, files map[string]string) error {
	var errs []error
	for name, content := range files {
		errs = append(errs, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("write logs: %w", err)
	}
	return nil
}
