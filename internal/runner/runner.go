// Package runner executes one task with one agent CLI profile.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// waitDelay is how long Run waits for output pipes after the worker is killed.
const waitDelay = 2 * time.Second

// Profile describes how to invoke one agent CLI.
type Profile struct {
	Name    string
	Command string
	// Args may contain the placeholder "{prompt}", replaced by Task.Prompt.
	Args []string
	// Timeout stops the command after this long. Zero means no limit.
	Timeout time.Duration
}

// Task is the work handed to the worker.
type Task struct {
	Prompt string
	// Dir is the working directory the command runs in.
	Dir string
}

// Result is what happened when the task ran.
type Result struct {
	ExitCode int
	// Output is the command's stdout.
	Output string
	// Stderr is the command's stderr, kept separate so it never pollutes Output.
	Stderr string
	// TimedOut is true when the command was stopped by Profile.Timeout.
	TimedOut bool
}

// Run executes task with profile and reports the result.
func Run(ctx context.Context, profile Profile, task Task) (Result, error) {
	if profile.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, profile.Timeout)
		defer cancel()
	}

	args := make([]string, 0, len(profile.Args))
	for _, arg := range profile.Args {
		args = append(args, strings.ReplaceAll(arg, "{prompt}", task.Prompt))
	}

	cmd := exec.CommandContext(ctx, profile.Command, args...) //nolint:gosec // G204: command comes from the user's own config
	cmd.Dir = task.Dir
	// Run the worker in its own process group and kill the whole group on
	// timeout or cancel: agent CLIs spawn children that would otherwise keep
	// running and keep stdout open. WaitDelay caps the wait for those pipes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = waitDelay

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.String()
	errOutput := stderr.String()

	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Result{ExitCode: -1, Output: output, TimedOut: true, Stderr: errOutput}, nil
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return Result{ExitCode: exitErr.ExitCode(), Output: output, Stderr: errOutput}, nil
	}

	if err != nil {
		return Result{}, fmt.Errorf("run %s: %w", profile.Command, err)
	}

	return Result{ExitCode: 0, Output: output, Stderr: errOutput}, nil
}
