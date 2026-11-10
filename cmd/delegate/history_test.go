package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewThenStats(t *testing.T) {
	// runDelegate sets XDG_STATE_HOME for the whole test, so review and
	// stats below read the run it just made.
	_, res := runDelegate(t, fakeWorker("echo done"))

	var stdout, stderr bytes.Buffer
	code := runCommands(t.Context(), []string{"review", res.RunID, "accepted", "clean", "diff"}, &stdout, &stderr, withProfiles(nil))
	require.Equal(t, 0, code, "stderr: %s", stderr.String())
	assert.Contains(t, stdout.String(), "accepted")
	assert.FileExists(t, filepath.Join(res.LogDir, "review.json"))

	stdout.Reset()
	code = runCommands(t.Context(), []string{"stats"}, &stdout, &stderr, withProfiles(nil))
	require.Equal(t, 0, code, "stderr: %s", stderr.String())
	out := stdout.String()
	assert.Contains(t, out, "PROFILE")
	assert.Regexp(t, `fake\s+1\s+1\s+0\s+0\s+0\s+0\s+1/1`, out, "runs, ok, needs_input, failed, timeout, error, accepted/reviewed")
}

func TestReview_RejectsBadInput(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer

	assert.Equal(t, 2, runCommands(t.Context(), []string{"review", "only-id"}, &stdout, &stderr, withProfiles(nil)), "missing verdict is a usage error")
	assert.Equal(t, 1, runCommands(t.Context(), []string{"review", "nope", "accepted"}, &stdout, &stderr, withProfiles(nil)))
	assert.Contains(t, stderr.String(), `no run "nope"`)
}

func TestStats_ReportsSkippedRunDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, "delegate", "runs", "broken"), 0o700))
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"stats"}, &stdout, &stderr, withProfiles(nil))

	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "skipped 1 run dir(s) without a readable result.json: broken")
}
