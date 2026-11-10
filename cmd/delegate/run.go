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
	"strings"
	"time"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/plan"
	"github.com/afplana/delegate/internal/runner"
	"github.com/afplana/delegate/internal/workspace"
)

// result is the JSON contract printed on stdout. Keep it small: the caller is
// usually a model paying for every token. Full output lives in LogDir.
type result struct {
	RunID        string         `json:"run_id"`
	Profile      string         `json:"profile"`
	Repo         string         `json:"repo"`       // absolute path of --repo
	StartedAt    string         `json:"started_at"` // RFC 3339, UTC
	Status       string         `json:"status"`     // ok | needs_input | failed | timeout | error
	ExitCode     int            `json:"exit_code"`
	DurationMS   int64          `json:"duration_ms"`
	Workdir      string         `json:"workdir,omitempty"`
	Branch       string         `json:"branch,omitempty"`
	Base         string         `json:"base,omitempty"`
	Dirty        bool           `json:"dirty,omitempty"` // repo had uncommitted or untracked files at start
	FilesChanged []string       `json:"files_changed"`
	OutOfScope   []string       `json:"out_of_scope,omitempty"` // changed files the plan's # Files does not allow
	Diffstat     string         `json:"diffstat"`
	Summary      string         `json:"summary"`
	Question     string         `json:"question,omitempty"` // what the worker needs answered
	Usage        *usage         `json:"usage,omitempty"`    // absent when the CLI's output could not be parsed
	Verify       []verifyResult `json:"verify,omitempty"`
	LogDir       string         `json:"log_dir,omitempty"`
	Error        string         `json:"error,omitempty"`
}

// usage is what the worker consumed. CostUSD is absent for CLIs that do not
// report it (Cursor).
type usage struct {
	InputTokens      int64    `json:"input_tokens"`
	OutputTokens     int64    `json:"output_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
	CacheWriteTokens int64    `json:"cache_write_tokens"`
	CostUSD          *float64 `json:"cost_usd,omitempty"`
}

// verifyResult is one --verify command's outcome. Full output is in
// verify-<n>.log next to the other logs.
type verifyResult struct {
	Cmd      string `json:"cmd"`
	Exit     int    `json:"exit"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Tail     string `json:"tail"`
}

// runIDEnv marks a worker's environment with its run id, so a delegate
// started inside a worker can tell it is nested.
const runIDEnv = "DELEGATE_RUN_ID"

type runFlags struct {
	task, profile, repo string
	inPlace             bool
	timeout             time.Duration
	verify              stringList
}

// stringList is a flag that can be passed more than once.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ", ") }

func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
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
	fs.Var(&f.verify, "verify", "shell command to run in the workdir after the worker, e.g. \"go test ./...\" (repeatable; same timeout as the worker)")
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
	res := result{
		RunID:        newRunID(start),
		Profile:      f.profile,
		Repo:         absPath(f.repo),
		StartedAt:    start.UTC().Format(time.RFC3339),
		FilesChanged: []string{},
	}
	fail := func(err error) result {
		res.Status = "error"
		res.Error = err.Error()
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}

	// A worker is a full agent session that may have the delegate skill; it
	// must never start a worker of its own. Empty counts as unset.
	if outer := os.Getenv(runIDEnv); outer != "" {
		return fail(fmt.Errorf("nested delegate run inside run %s", outer))
	}
	profile, err := config.Lookup(profiles, f.profile)
	if err != nil {
		return fail(err)
	}
	if f.timeout > 0 {
		profile.Timeout = f.timeout
	}
	planText, err := os.ReadFile(f.task)
	if err != nil {
		return fail(fmt.Errorf("read task: %w", err))
	}
	parsed := plan.Parse(string(planText))
	if missing := parsed.Missing(); len(missing) > 0 {
		return fail(fmt.Errorf("plan missing sections: %s", strings.Join(missing, ", ")))
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
	res.Workdir, res.Branch, res.Base, res.Dirty = ws.Dir, ws.Branch, ws.Base, ws.Dirty

	prompt := runner.WorkerContract + string(planText) + runner.SummaryInstruction
	if err := writeLogs(logDir, map[string]string{"prompt.md": prompt}); err != nil {
		return fail(err)
	}
	out, err := runner.Run(ctx, profile, runner.Task{Prompt: prompt, Dir: ws.Dir, Env: []string{runIDEnv + "=" + res.RunID}})
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
	res.OutOfScope = plan.OutOfScope(res.FilesChanged, parsed.Files())

	checks := runner.Verify(ctx, ws.Dir, f.verify, profile.Timeout)
	logs := make(map[string]string, len(checks))
	for i, c := range checks {
		res.Verify = append(res.Verify, verifyResult{Cmd: c.Cmd, Exit: c.Exit, TimedOut: c.TimedOut, Tail: c.Tail})
		logs[fmt.Sprintf("verify-%d.log", i+1)] = c.Output
	}
	if err := writeLogs(logDir, logs); err != nil {
		return fail(err)
	}

	res.ExitCode = out.ExitCode
	reply := out.Output
	if profile.Parse != nil {
		// A parse failure is not delegate's failure: a worker that crashed
		// early prints plain text, which is still the best reply there is.
		if text, u, err := profile.Parse(out.Output); err == nil {
			reply = text
			res.Usage = &usage{
				InputTokens:      u.InputTokens,
				OutputTokens:     u.OutputTokens,
				CacheReadTokens:  u.CacheReadTokens,
				CacheWriteTokens: u.CacheWriteTokens,
				CostUSD:          u.CostUSD,
			}
		}
	}
	res.Summary = runner.ExtractSummary(reply)
	question, asked := runner.ExtractQuestion(reply)
	res.Question = question
	res.Status = status(out, asked)
	res.DurationMS = time.Since(start).Milliseconds()
	return res
}

// status maps the worker's outcome. A crash or timeout wins over a question:
// the process outcome is what the planner must handle first.
func status(out runner.Result, asked bool) string {
	switch {
	case out.TimedOut:
		return "timeout"
	case out.ExitCode != 0:
		return "failed"
	case asked:
		return "needs_input"
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

// absPath makes a past result traceable to its repo. If the path cannot be
// resolved, the flag value as given still beats an empty field.
func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
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
