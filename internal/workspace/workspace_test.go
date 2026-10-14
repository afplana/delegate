package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afplana/delegate/internal/gittest"
	"github.com/afplana/delegate/internal/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepare_CreatesWorktreeOnOwnBranch(t *testing.T) {
	repo := gittest.InitRepo(t)
	root := t.TempDir()

	ws, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: "run1", WorktreeRoot: root})

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "run1"), ws.Dir)
	assert.Equal(t, "delegate/run1", ws.Branch)
	assert.FileExists(t, filepath.Join(ws.Dir, "README.md"))
	head := strings.TrimSpace(gittest.Run(t, repo, "rev-parse", "HEAD"))
	assert.Equal(t, head, ws.Base)
}

func TestChanges_ReportsEditedAndNewFiles(t *testing.T) {
	repo := gittest.InitRepo(t)
	ws, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: "run1", WorktreeRoot: t.TempDir()})
	require.NoError(t, err)

	// What a worker does: edit a tracked file and create an untracked one.
	require.NoError(t, os.WriteFile(filepath.Join(ws.Dir, "README.md"), []byte("hello\nworld\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ws.Dir, "new.go"), []byte("package x\n"), 0o600))

	changes, err := ws.Changes(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"README.md", "new.go"}, changes.Files)
	assert.Equal(t, "2 files changed, 2 insertions(+)", changes.Diffstat)
}

func TestChanges_IncludesWorkTheWorkerCommitted(t *testing.T) {
	repo := gittest.InitRepo(t)
	ws, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: "run1", WorktreeRoot: t.TempDir()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(ws.Dir, "done.go"), []byte("package x\n"), 0o600))
	gittest.Run(t, ws.Dir, "add", "done.go")
	gittest.Run(t, ws.Dir, "commit", "--message", "worker commit")

	changes, err := ws.Changes(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"done.go"}, changes.Files)
}

func TestChanges_NothingChanged(t *testing.T) {
	repo := gittest.InitRepo(t)
	ws, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: "run1", WorktreeRoot: t.TempDir()})
	require.NoError(t, err)

	changes, err := ws.Changes(t.Context())

	require.NoError(t, err)
	assert.Empty(t, changes.Files)
	assert.Empty(t, changes.Diffstat)
}

func TestPrepare_InPlaceUsesRepoOnFeatureBranch(t *testing.T) {
	repo := gittest.InitRepo(t)
	gittest.Run(t, repo, "switch", "--create", "feature/x")

	ws, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: "run1", InPlace: true})

	require.NoError(t, err)
	assert.Equal(t, repo, ws.Dir)
	assert.Equal(t, "feature/x", ws.Branch)
}

func TestPrepare_InPlaceRefusesMainBranch(t *testing.T) {
	repo := gittest.InitRepo(t)

	_, err := workspace.Prepare(t.Context(), workspace.Options{Repo: repo, RunID: "run1", InPlace: true})

	assert.ErrorContains(t, err, `refusing to run in place on "main"`)
}

func TestPrepare_FailsOutsideGitRepo(t *testing.T) {
	_, err := workspace.Prepare(t.Context(), workspace.Options{Repo: t.TempDir(), RunID: "run1", WorktreeRoot: t.TempDir()})

	assert.ErrorContains(t, err, "not a git repository")
}
