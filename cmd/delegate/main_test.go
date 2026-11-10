package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// validPlan has every section delegate requires before starting a worker.
const validPlan = "# Goal\nCreate new.txt\n\n# Steps\n1. Create it.\n\n# Acceptance criteria\n- new.txt exists.\n"

// runDelegate runs `delegate run` in a fresh repo with a valid plan and decodes the JSON it prints.
func runDelegate(t *testing.T, profiles map[string]runner.Profile, extraArgs ...string) (int, result) {
	t.Helper()
	return runDelegateWithPlan(t, profiles, validPlan, extraArgs...)
}

func runDelegateWithPlan(t *testing.T, profiles map[string]runner.Profile, planText string, extraArgs ...string) (int, result) {
	t.Helper()
	// These tests may themselves run inside a delegate worker; an inherited
	// run id would make every run refuse as nested.
	t.Setenv("DELEGATE_RUN_ID", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := gittest.InitRepo(t)
	task := filepath.Join(t.TempDir(), "plan.md")
	require.NoError(t, os.WriteFile(task, []byte(planText), 0o600))

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
	assert.True(t, strings.HasPrefix(string(prompt), runner.WorkerContract), "the worker contract comes first")

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
		{
			// A question after a clean exit means the work is not done yet.
			name:     "question after a clean exit is needs_input",
			script:   `printf '<delegate-question>\nWhich name?\n</delegate-question>\n'`,
			status:   "needs_input",
			exitCode: 0,
		},
		{
			name:     "a failed exit stays failed even with a question",
			script:   `printf '<delegate-question>\nWhich name?\n</delegate-question>\n'; exit 2`,
			status:   "failed",
			exitCode: 2,
		},
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

func TestRun_VerifyRunsAcceptanceCommandsAfterTheWorker(t *testing.T) {
	// The worker creates new.txt; the checks see its result in the workdir.
	code, res := runDelegate(t, fakeWorker("touch new.txt"),
		"--verify", "test -f new.txt && echo found", "--verify", "echo red; exit 4")

	assert.Equal(t, 0, code, "a red check does not change the worker's exit")
	assert.Equal(t, "ok", res.Status, "status stays the worker's")
	require.Len(t, res.Verify, 2)
	assert.Equal(t, verifyResult{Cmd: "test -f new.txt && echo found", Exit: 0, Tail: "found"}, res.Verify[0])
	assert.Equal(t, verifyResult{Cmd: "echo red; exit 4", Exit: 4, Tail: "red"}, res.Verify[1])
	for _, name := range []string{"verify-1.log", "verify-2.log"} {
		assert.FileExists(t, filepath.Join(res.LogDir, name))
	}
}

func TestRun_WithoutVerifyLeavesTheFieldOut(t *testing.T) {
	// Every byte of the result costs the planner tokens.
	code, res := runDelegate(t, fakeWorker("true"))

	assert.Equal(t, 0, code)
	assert.Nil(t, res.Verify)
	line, err := os.ReadFile(filepath.Join(res.LogDir, "result.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(line), `"verify"`)
}

func TestRun_ListsChangedFilesOutsideThePlansFiles(t *testing.T) {
	// Drift shows up in one JSON field instead of a full diff review.
	planText := validPlan + "\n# Files\n- `new.txt`\n"

	code, res := runDelegateWithPlan(t, fakeWorker("touch new.txt extra.txt"), planText)

	assert.Equal(t, 0, code, "out-of-scope files do not change the worker's status")
	assert.Equal(t, []string{"extra.txt"}, res.OutOfScope)
}

func TestRun_WithoutFilesSectionSkipsTheScopeCheck(t *testing.T) {
	code, res := runDelegate(t, fakeWorker("touch new.txt extra.txt"))

	assert.Equal(t, 0, code)
	assert.Nil(t, res.OutOfScope)
}

func TestRun_RecordsRepoAndStartTime(t *testing.T) {
	// A past result.json must say where and when the run happened.
	before := time.Now().UTC().Truncate(time.Second)

	_, res := runDelegate(t, fakeWorker("true"))

	assert.True(t, filepath.IsAbs(res.Repo), "repo is absolute, got %q", res.Repo)
	assert.DirExists(t, filepath.Join(res.Repo, ".git"))
	started, err := time.Parse(time.RFC3339, res.StartedAt)
	require.NoError(t, err)
	assert.False(t, started.Before(before), "started_at %s is before the test began", res.StartedAt)
	assert.Equal(t, time.UTC, started.Location())
}

func TestRun_RecordsRepoAndStartTimeOnError(t *testing.T) {
	_, res := runDelegateWithPlan(t, fakeWorker("true"), "# Goal\nonly\n")

	assert.Equal(t, "error", res.Status)
	assert.NotEmpty(t, res.Repo)
	assert.NotEmpty(t, res.StartedAt)
}

func TestRun_FlagsADirtySourceRepo(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DELEGATE_RUN_ID", "")
	repo := gittest.InitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "README.md"), []byte("uncommitted\n"), 0o600))
	task := filepath.Join(t.TempDir(), "plan.md")
	require.NoError(t, os.WriteFile(task, []byte(validPlan), 0o600))
	var stdout, stderr bytes.Buffer

	runCommands(t.Context(), []string{"run", "--task", task, "--repo", repo, "--profile", "fake"}, &stdout, &stderr, fakeWorker("true"))

	var res result
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res), "stdout: %s", stdout.String())
	assert.True(t, res.Dirty)
}

func TestRun_CleanRepoLeavesDirtyOut(t *testing.T) {
	_, res := runDelegate(t, fakeWorker("true"))

	line, err := os.ReadFile(filepath.Join(res.LogDir, "result.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(line), `"dirty"`)
}

func TestRun_RefusesAPlanWithoutRequiredSections(t *testing.T) {
	// The worker must never start on a plan with no way to tell when it is done.
	code, res := runDelegateWithPlan(t, fakeWorker("touch started.txt"), "# Goal\nCreate new.txt\n")

	assert.Equal(t, 1, code)
	assert.Equal(t, "error", res.Status)
	assert.Equal(t, "plan missing sections: Steps, Acceptance criteria", res.Error)
	assert.Empty(t, res.Workdir, "no worktree is created for a refused plan")
}

func TestRun_ReportsTheWorkersQuestion(t *testing.T) {
	script := `printf '<delegate-summary>\nDid step 1.\n</delegate-summary>\n<delegate-question>\nStep 2: keep the alias?\n</delegate-question>\n'`

	_, res := runDelegate(t, fakeWorker(script))

	assert.Equal(t, "needs_input", res.Status)
	assert.Equal(t, "Step 2: keep the alias?", res.Question)
	assert.Equal(t, "Did step 1.", res.Summary)
}

func TestRun_MarksTheWorkerWithItsRunID(t *testing.T) {
	_, res := runDelegate(t, fakeWorker(`echo "$DELEGATE_RUN_ID" > run-id.txt`))

	got, err := os.ReadFile(filepath.Join(res.Workdir, "run-id.txt"))
	require.NoError(t, err)
	assert.Equal(t, res.RunID+"\n", string(got))
}

func TestRun_RefusesToStartInsideAnotherRun(t *testing.T) {
	// A worker is a full agent session with the delegate skill loaded; a plan
	// saying "delegate X" must not make it start a worker of its own.
	t.Setenv("DELEGATE_RUN_ID", "outer-run")
	var stdout, stderr bytes.Buffer

	// The check comes first, so it needs no real repo or plan.
	code := runCommands(t.Context(), []string{"run", "--task", "plan.md", "--profile", "fake"}, &stdout, &stderr, fakeWorker("true"))

	var res result
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &res), "stdout: %s", stdout.String())
	assert.Equal(t, 1, code)
	assert.Equal(t, "error", res.Status)
	assert.Equal(t, "nested delegate run inside run outer-run", res.Error)
	assert.Empty(t, res.Workdir, "no worktree for a refused run")
}

func TestRun_ReadsReplyAndUsageThroughTheProfileParser(t *testing.T) {
	profiles := fakeWorker(`printf '{"result":"<delegate-summary>\\nparsed\\n</delegate-summary>","usage":{"inputTokens":7,"outputTokens":3}}'`)
	p := profiles["fake"]
	p.Parse = runner.ParseCursorJSON
	profiles["fake"] = p

	_, res := runDelegate(t, profiles)

	assert.Equal(t, "parsed", res.Summary, "summary comes from the parsed reply, not raw JSON")
	require.NotNil(t, res.Usage)
	assert.Equal(t, int64(7), res.Usage.InputTokens)
	assert.Equal(t, int64(3), res.Usage.OutputTokens)
}

func TestRun_FallsBackToRawOutputWhenParsingFails(t *testing.T) {
	profiles := fakeWorker(`echo "plain crash text"; exit 1`)
	p := profiles["fake"]
	p.Parse = runner.ParseCursorJSON
	profiles["fake"] = p

	_, res := runDelegate(t, profiles)

	assert.Equal(t, "failed", res.Status)
	assert.Equal(t, "plain crash text", res.Summary)
	assert.Nil(t, res.Usage)
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
