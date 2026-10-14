package runner_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_ReportsWhatTheWorkerDid(t *testing.T) {
	tests := []struct {
		name         string
		profile      runner.Profile
		wantExitCode int
		wantOutput   string
		wantStderr   string
	}{
		{
			name:       "passes the prompt and captures stdout",
			profile:    runner.Profile{Name: "echo", Command: "echo", Args: []string{"{prompt}"}},
			wantOutput: "implement TEST-1\n",
		},
		{
			// Real profiles look like: cursor-agent -p --force "{prompt}".
			// printf proves every arg arrives, in order, with only {prompt} replaced.
			name:       "keeps fixed args around the prompt",
			profile:    runner.Profile{Name: "printf", Command: "printf", Args: []string{"[%s]", "{prompt}"}},
			wantOutput: "[implement TEST-1]",
		},
		{
			// A worker that exits non-zero is a normal outcome the planner must see,
			// together with whatever it printed before failing.
			name:         "reports worker failure as a result, not an error",
			profile:      runner.Profile{Name: "sh", Command: "sh", Args: []string{"-c", "echo partial work; exit 3"}},
			wantExitCode: 3,
			wantOutput:   "partial work\n",
		},
		{
			// The summary must only come from stdout, so stderr stays separate.
			name:       "captures stderr separately from output",
			profile:    runner.Profile{Name: "sh", Command: "sh", Args: []string{"-c", "echo done; echo warning >&2"}},
			wantOutput: "done\n",
			wantStderr: "warning\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

			result, err := runner.Run(t.Context(), tt.profile, task)

			require.NoError(t, err)
			assert.Equal(t, tt.wantExitCode, result.ExitCode)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.Equal(t, tt.wantStderr, result.Stderr)
			assert.False(t, result.TimedOut)
		})
	}
}

func TestRun_ReturnsErrorWhenCommandIsMissing(t *testing.T) {
	// delegate itself cannot run: this one IS an error, and callers can detect it.
	profile := runner.Profile{Name: "missing", Command: "delegate-no-such-binary"}
	task := runner.Task{Prompt: "implement TEST-1", Dir: t.TempDir()}

	_, err := runner.Run(t.Context(), profile, task)

	assert.ErrorIs(t, err, exec.ErrNotFound)
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

	require.NoError(t, err, "a timeout is a Result, not an error")
	assert.True(t, result.TimedOut)
	assert.Less(t, elapsed, 2*time.Second, "worker should stop near its 100ms timeout")
}
