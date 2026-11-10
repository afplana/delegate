package workspace_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/gittest"
	"github.com/afplana/delegate/internal/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const oldRun = "20200101-000000-aaaa"

// prepareOld creates a worktree for a run that started in 2020.
func prepareOld(t *testing.T) (repo, root string, ws workspace.Workspace) {
	t.Helper()
	repo = gittest.InitRepo(t)
	root = t.TempDir()
	ws, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: oldRun, WorktreeRoot: root})
	require.NoError(t, err)
	return repo, root, ws
}

func TestFindStale_ListsOnlyWorktreesOlderThanTheCutoff(t *testing.T) {
	repo, root, ws := prepareOld(t)
	recent := time.Now().UTC().Format("20060102-150405") + "-bbbb"
	_, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: recent, WorktreeRoot: root})
	require.NoError(t, err)

	stale, err := workspace.FindStale(t.Context(), root, time.Now().Add(-7*24*time.Hour))

	require.NoError(t, err)
	require.Len(t, stale, 1)
	got := stale[0]
	assert.Equal(t, oldRun, got.RunID)
	assert.Equal(t, ws.Dir, got.Dir)
	assert.Equal(t, "delegate/"+oldRun, got.Branch)
	assertSamePath(t, repo, got.Repo)
	assert.Positive(t, got.Bytes)
	assert.Empty(t, got.Problem)
}

func TestFindStale_MissingRootIsEmpty(t *testing.T) {
	stale, err := workspace.FindStale(t.Context(), filepath.Join(t.TempDir(), "nope"), time.Now())

	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestFindStale_ReportsAnOrphanInsteadOfGuessing(t *testing.T) {
	// A worktree whose source repo is gone cannot be removed through git.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, oldRun), 0o700))

	stale, err := workspace.FindStale(t.Context(), root, time.Now())

	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Contains(t, stale[0].Problem, "not a git worktree")
}

func TestFindStale_ReportsAWorktreeWhoseRepoIsGone(t *testing.T) {
	// Seen for real: the .git file points at a gitdir that no longer exists.
	root := t.TempDir()
	dir := filepath.Join(root, oldRun)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /nowhere/.git/worktrees/x\n"), 0o600))

	stale, err := workspace.FindStale(t.Context(), root, time.Now())

	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, "its source repo is gone (/nowhere/.git/worktrees/x); remove it by hand if unwanted", stale[0].Problem)
}

func TestRemoveStale_RemovesWorktreeAndMergedBranch(t *testing.T) {
	repo, root, _ := prepareOld(t)
	stale, err := workspace.FindStale(t.Context(), root, time.Now())
	require.NoError(t, err)
	require.Len(t, stale, 1)

	require.NoError(t, workspace.RemoveStale(t.Context(), stale[0]))

	assert.NoDirExists(t, stale[0].Dir)
	assert.Empty(t, gittest.Run(t, repo, "branch", "--list", "delegate/"+oldRun))
}

func TestRemoveStale_KeepsUncommittedWork(t *testing.T) {
	_, root, ws := prepareOld(t)
	require.NoError(t, os.WriteFile(filepath.Join(ws.Dir, "README.md"), []byte("worker edit\n"), 0o600))
	stale, err := workspace.FindStale(t.Context(), root, time.Now())
	require.NoError(t, err)

	err = workspace.RemoveStale(t.Context(), stale[0])

	require.Error(t, err)
	assert.DirExists(t, ws.Dir, "never force-remove a worktree with changes")
}

func TestRemoveStale_KeepsAnUnmergedBranch(t *testing.T) {
	// Committed but never accepted work lives only on that branch.
	repo, root, ws := prepareOld(t)
	require.NoError(t, os.WriteFile(filepath.Join(ws.Dir, "new.txt"), []byte("x\n"), 0o600))
	gittest.Run(t, ws.Dir, "add", "new.txt")
	gittest.Run(t, ws.Dir, "commit", "--message", "worker commit")
	stale, err := workspace.FindStale(t.Context(), root, time.Now())
	require.NoError(t, err)

	err = workspace.RemoveStale(t.Context(), stale[0])

	require.ErrorIs(t, err, workspace.ErrBranchKept)
	assert.NotContains(t, err.Error(), "hint:", "only git's first line, not its advice")
	assert.NoDirExists(t, ws.Dir, "the worktree itself is clean, so it goes")
	assert.NotEmpty(t, gittest.Run(t, repo, "branch", "--list", "delegate/"+oldRun))
}

// assertSamePath compares paths after resolving symlinks (/var vs /private/var on macOS).
func assertSamePath(t *testing.T, want, got string) {
	t.Helper()
	w, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)
	g, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, w, g)
}
