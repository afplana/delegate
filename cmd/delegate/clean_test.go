package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/afplana/delegate/internal/gittest"
	"github.com/afplana/delegate/internal/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const oldRunID = "20200101-000000-aaaa"

// oldRunState makes a 2020 run with a worktree and a log dir in a fresh state dir.
func oldRunState(t *testing.T) (worktree, logDir string) {
	t.Helper()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	root := filepath.Join(state, "delegate")
	ws, err := workspace.Prepare(t.Context(), workspace.Options{
		Repo: gittest.InitRepo(t), RunID: oldRunID, WorktreeRoot: filepath.Join(root, "worktrees"),
	})
	require.NoError(t, err)
	logDir = filepath.Join(root, "runs", oldRunID)
	require.NoError(t, os.MkdirAll(logDir, 0o700))
	return ws.Dir, logDir
}

func TestClean_PreviewsWithoutYes(t *testing.T) {
	worktree, logDir := oldRunState(t)
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"clean", "--logs"}, &stdout, &stderr, nil)

	require.Equal(t, 0, code, "stderr: %s", stderr.String())
	out := stdout.String()
	assert.Contains(t, out, "would remove worktree "+oldRunID)
	assert.Contains(t, out, "would remove 1 run log dir(s)")
	assert.Contains(t, out, "--yes")
	assert.DirExists(t, worktree, "a preview deletes nothing")
	assert.DirExists(t, logDir)
}

func TestClean_YesRemovesWorktreeButKeepsLogsUnlessAsked(t *testing.T) {
	worktree, logDir := oldRunState(t)
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"clean", "--yes"}, &stdout, &stderr, nil)

	require.Equal(t, 0, code, "stderr: %s", stderr.String())
	assert.Contains(t, stdout.String(), "removed worktree "+oldRunID)
	assert.NoDirExists(t, worktree)
	assert.DirExists(t, logDir, "logs feed delegate stats; only --logs removes them")
}

func TestClean_YesWithLogsRemovesLogsToo(t *testing.T) {
	_, logDir := oldRunState(t)
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"clean", "--yes", "--logs"}, &stdout, &stderr, nil)

	require.Equal(t, 0, code, "stderr: %s", stderr.String())
	assert.NoDirExists(t, logDir)
}

func TestClean_RecentRunsAreLeftAlone(t *testing.T) {
	worktree, _ := oldRunState(t)
	var stdout, stderr bytes.Buffer

	// 2020 is younger than 100 years.
	code := runCommands(t.Context(), []string{"clean", "--yes", "--days", "36500"}, &stdout, &stderr, nil)

	require.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "nothing to clean")
	assert.DirExists(t, worktree)
}
