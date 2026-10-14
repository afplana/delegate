package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/gittest"
	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWorker returns a profile that runs a shell script instead of an agent.
// The prompt is passed as $1 so scripts can check it arrived.
func fakeWorker(script string) map[string]runner.Profile {
	return map[string]runner.Profile{
		"fake": {Name: "fake", Command: "sh", Args: []string{"-c", script, "sh", "{prompt}"}, Timeout: 5 * time.Second},
	}
}

// runDelegate runs `delegate run` in a fresh repo with a task file and decodes the JSON it prints.
func runDelegate(t *testing.T, profiles map[string]runner.Profile, extraArgs ...string) (int, result) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := gittest.InitRepo(t)
	task := filepath.Join(t.TempDir(), "plan.md")
	require.NoError(t, os.WriteFile(task, []byte("Create new.txt"), 0o600))

	var stdout, stderr bytes.Buffer
	args := append([]string{"run", "--task", task, "--repo", repo, "--profile", "fake"}, extraArgs...)
	code := runCommands(t.Context(), args, &stdout, &stderr, profiles)

	var res result
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res), "stdout: %s\nstderr: %s", stdout.String(), stderr.String())
	return code, res
}

func TestRun_ReportsWorkerChangesAsCompactJSON(t *testing.T) {
	script := `echo "$1" > prompt-seen.txt; echo noise; printf '<delegate-summary>\nCreated new.txt\n</delegate-summary>\n'`

	code, res := runDelegate(t, fakeWorker(script))

	assert.Equal(t, 0, code)
	assert.Equal(t, "ok", res.Status)
	assert.Equal(t, "fake", res.Profile)
	assert.Equal(t, "Created new.txt", res.Summary)
	assert.Equal(t, []string{"prompt-seen.txt"}, res.FilesChanged)
	assert.Equal(t, "delegate/"+res.RunID, res.Branch)
	assert.NotEmpty(t, res.Base)

	prompt, err := os.ReadFile(filepath.Join(res.Workdir, "prompt-seen.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(prompt), "Create new.txt", "the task file reaches the worker")
	assert.Contains(t, string(prompt), "<delegate-summary>", "the worker is told how to report")

	for _, name := range []string{"prompt.md", "stdout.log", "stderr.log", "result.json"} {
		assert.FileExists(t, filepath.Join(res.LogDir, name))
	}
}

func TestRun_MapsWorkerOutcomeToStatus(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		args     []string
		status   string
		exitCode int
	}{
		{name: "non-zero exit is failed", script: "exit 3", status: "failed", exitCode: 3},
		{name: "timeout is timeout", script: "sleep 3", args: []string{"--timeout", "100ms"}, status: "timeout", exitCode: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, res := runDelegate(t, fakeWorker(tt.script), tt.args...)

			assert.Equal(t, 1, code)
			assert.Equal(t, tt.status, res.Status)
			assert.Equal(t, tt.exitCode, res.ExitCode)
		})
	}
}

func TestRun_ReportsDelegateFailuresAsJSON(t *testing.T) {
	code, res := runDelegate(t, map[string]runner.Profile{})

	assert.Equal(t, 1, code)
	assert.Equal(t, "error", res.Status)
	assert.Contains(t, res.Error, `unknown profile "fake"`)
}

func TestRun_RequiresTask(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"run"}, &stdout, &stderr, fakeWorker("true"))

	assert.Equal(t, 2, code)
	assert.Contains(t, stderr.String(), "--task is required")
	assert.Empty(t, stdout.String())
}
