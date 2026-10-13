package runner_test

import (
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/runner"
)

func TestRun_PassesPromptToCommandAndCapturesOutput(t *testing.T) {
	profile := runner.Profile{
		Name:    "echo",
		Command: "echo",
		Args:    []string{"{prompt}"},
	}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	result, err := runner.Run(t.Context(), profile, task)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if got, want := result.Output, "implement TEST-1\n"; got != want {
		t.Errorf("Output = %q, want %q", got, want)
	}
}

func TestRun_KeepsFixedArgsAroundThePrompt(t *testing.T) {
	// Real profiles look like: cursor-agent -p --force "{prompt}".
	// printf proves every arg arrives, in order, with only {prompt} replaced.
	profile := runner.Profile{
		Name:    "printf",
		Command: "printf",
		Args:    []string{"[%s]", "{prompt}"},
	}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	result, err := runner.Run(t.Context(), profile, task)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got, want := result.Output, "[implement TEST-1]"; got != want {
		t.Errorf("Output = %q, want %q", got, want)
	}
}

func TestRun_ReportsWorkerFailureAsResultNotError(t *testing.T) {
	// A worker that exits non-zero is a normal outcome the planner must see,
	// together with whatever it printed before failing.
	profile := runner.Profile{
		Name:    "sh",
		Command: "sh",
		Args:    []string{"-c", "echo partial work; exit 3"},
	}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	result, err := runner.Run(t.Context(), profile, task)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil: a failing worker is a Result, not an error", err)
	}

	if result.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", result.ExitCode)
	}
	if got, want := result.Output, "partial work\n"; got != want {
		t.Errorf("Output = %q, want %q", got, want)
	}
}

func TestRun_ReturnsErrorWhenCommandIsMissing(t *testing.T) {
	// delegate itself cannot run: this one IS an error, and callers can detect it.
	profile := runner.Profile{Name: "missing", Command: "delegate-no-such-binary"}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	_, err := runner.Run(t.Context(), profile, task)
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Run() error = %v, want it to wrap exec.ErrNotFound", err)
	}
}

func TestRun_CapturesStderrSeparatelyFromOutput(t *testing.T) {
	// Workers print progress and warnings on stderr; the planner needs both
	// streams, and the summary must only come from stdout.
	profile := runner.Profile{
		Name:    "sh",
		Command: "sh",
		Args:    []string{"-c", "echo done; echo warning >&2"},
	}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	result, err := runner.Run(t.Context(), profile, task)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got, want := result.Output, "done\n"; got != want {
		t.Errorf("Output = %q, want %q", got, want)
	}
	if got, want := result.Stderr, "warning\n"; got != want {
		t.Errorf("Stderr = %q, want %q", got, want)
	}
}

func TestRun_StopsWorkerAfterProfileTimeout(t *testing.T) {
	// A stuck worker must not block the planner forever.
	profile := runner.Profile{
		Name:    "sleep",
		Command: "sleep",
		Args:    []string{"3"},
		Timeout: 100 * time.Millisecond,
	}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	start := time.Now()
	result, err := runner.Run(t.Context(), profile, task)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil: a timeout is a Result, not an error", err)
	}
	if !result.TimedOut {
		t.Error("TimedOut = false, want true")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Run() took %v, want the worker stopped near its 100ms timeout", elapsed)
	}
}
