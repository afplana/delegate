package runner_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
)

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			// The worker is asked to end with this block; everything around it
			// is transcript noise the planner should not pay tokens for.
			name:   "returns the text inside the summary block",
			output: "reading files...\n<delegate-summary>\nAdded a timeout to runner.Run.\n</delegate-summary>\nbye\n",
			want:   "Added a timeout to runner.Run.",
		},
		{
			// A worker that ignores the instruction still gets reported: its
			// final lines usually say what it did, and 20 lines stay cheap.
			name:   "falls back to the last 20 lines when there is no summary block",
			output: numberedLines(1, 25) + "\n",
			want:   numberedLines(6, 25),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, runner.ExtractSummary(tt.output))
		})
	}
}

// numberedLines returns "line from\n...\nline to" with no trailing newline.
func numberedLines(from, to int) string {
	lines := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	return strings.Join(lines, "\n")
}
