// Package gittest builds throwaway git repos for tests.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// InitRepo creates a repo on branch main with one commit holding README.md
// and returns its path with symlinks resolved (macOS temp dirs are symlinks).
// The user's global git config is ignored so hooks or signing cannot interfere.
func InitRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	Run(t, dir, "init", "--initial-branch", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o600))
	Run(t, dir, "add", "README.md")
	Run(t, dir, "commit", "--message", "initial")
	return dir
}

// Run runs git in dir and fails the test if it fails.
func Run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput() //nolint:gosec // G204: fixed binary, args from the test itself
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}
