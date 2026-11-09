package runner

import (
	"context"
	"time"
)

// verifyTailLines is how much of each check's output goes into the result.
// Test runners print their failure summary last.
const verifyTailLines = 30

// VerifyResult is the outcome of one acceptance command run after the worker.
type VerifyResult struct {
	Cmd  string
	Exit int
	// TimedOut is true when the command was stopped by the timeout.
	TimedOut bool
	// Tail is the last verifyTailLines lines of stdout and stderr combined.
	Tail string
	// Output is the full combined output, for the run logs.
	Output string
}

// Verify runs each command with `sh -c` in dir, in order, and reports every
// one of them: a red check does not stop the next. It reuses Run, so a hung
// check is killed with its whole process group, just like a worker.
func Verify(ctx context.Context, dir string, cmds []string, timeout time.Duration) []VerifyResult {
	if len(cmds) == 0 {
		return nil
	}
	results := make([]VerifyResult, 0, len(cmds))
	for _, cmd := range cmds {
		// "exec 2>&1" merges stderr into stdout for the whole script, keeping
		// the order the planner would see in a terminal.
		profile := Profile{Name: "verify", Command: "sh", Args: []string{"-c", "exec 2>&1\n" + cmd}, Timeout: timeout}
		out, err := Run(ctx, profile, Task{Dir: dir})
		if err != nil {
			// sh itself could not start; report it like a failed check.
			results = append(results, VerifyResult{Cmd: cmd, Exit: -1, Tail: err.Error(), Output: err.Error()})
			continue
		}
		results = append(results, VerifyResult{
			Cmd:      cmd,
			Exit:     out.ExitCode,
			TimedOut: out.TimedOut,
			Tail:     lastLines(out.Output, verifyTailLines),
			Output:   out.Output,
		})
	}
	return results
}
