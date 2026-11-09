package runner_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_ReportsEachCommandsExitAndOutput(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		exit int
		tail string
	}{
		{name: "passing command", cmd: "echo ok", exit: 0, tail: "ok"},
		{name: "failing command keeps its exit code", cmd: "echo boom; exit 3", exit: 3, tail: "boom"},
		{
			// The planner needs the error text, which tools usually print on stderr.
			name: "stderr is merged into the tail in order",
			cmd:  "echo out; echo err >&2; echo end",
			exit: 0,
			tail: "out\nerr\nend",
		},
		{
			// A failing test suite can print thousands of lines; the planner
			// only needs the end, where the failure summary is.
			name: "tail keeps the last 30 lines",
			cmd:  "seq 1 40",
			exit: 0,
			tail: numberedTail(11, 40),
		},
		{name: "unknown command is exit 127, not an error", cmd: "no-such-command-xyz", exit: 127},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.Verify(t.Context(), t.TempDir(), []string{tt.cmd}, 5*time.Second)

			require.Len(t, got, 1)
			assert.Equal(t, tt.cmd, got[0].Cmd)
			assert.Equal(t, tt.exit, got[0].Exit)
			assert.False(t, got[0].TimedOut)
			if tt.tail != "" {
				assert.Equal(t, tt.tail, got[0].Tail)
			}
		})
	}
}

func TestVerify_RunsEveryCommandInDirEvenAfterAFailure(t *testing.T) {
	// One red check must not hide the others: the planner wants the full picture.
	dir := t.TempDir()

	got := runner.Verify(t.Context(), dir, []string{"exit 1", "touch marker"}, 5*time.Second)

	require.Len(t, got, 2)
	assert.Equal(t, 1, got[0].Exit)
	assert.Equal(t, 0, got[1].Exit)
	assert.FileExists(t, filepath.Join(dir, "marker"), "commands run in the workdir")
}

func TestVerify_StopsAHungCommand(t *testing.T) {
	got := runner.Verify(t.Context(), t.TempDir(), []string{"sleep 3"}, 100*time.Millisecond)

	require.Len(t, got, 1)
	assert.True(t, got[0].TimedOut)
	assert.Equal(t, -1, got[0].Exit)
}

func TestVerify_NoCommandsReturnsNil(t *testing.T) {
	assert.Nil(t, runner.Verify(t.Context(), t.TempDir(), nil, time.Second))
}

// numberedTail returns "from\n...\nto", the shape `seq` prints.
func numberedTail(from, to int) string {
	lines := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		lines = append(lines, strconv.Itoa(i))
	}
	return strings.Join(lines, "\n")
}
