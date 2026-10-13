// Package runner executes one task with one agent CLI profile.
package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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

	cmd := exec.CommandContext(ctx, profile.Command, args...)
	cmd.Dir = task.Dir
	stdout, err := cmd.Output()
	output := string(stdout)

	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Result{ExitCode: -1, Output: output, TimedOut: true}, nil
	}

	var exitErr *exec.ExitError

	if errors.As(err, &exitErr) {
		return Result{ExitCode: exitErr.ExitCode(), Output: output}, nil
	}

	if err != nil {
		return Result{}, fmt.Errorf("run %s: %w", profile.Command, err)
	}

	return Result{ExitCode: 0, Output: output}, nil
}
