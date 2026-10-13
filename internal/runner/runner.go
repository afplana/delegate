// Package runner executes one task with one agent CLI profile.
package runner

import (
	"context"
	"errors"
)

// Profile describes how to invoke one agent CLI.
type Profile struct {
	Name    string
	Command string
	// Args may contain the placeholder "{prompt}", replaced by Task.Prompt.
	Args []string
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
}

// Run executes task with profile and reports the result.
func Run(ctx context.Context, profile Profile, task Task) (Result, error) {
	return Result{}, errors.New("not implemented")
}
