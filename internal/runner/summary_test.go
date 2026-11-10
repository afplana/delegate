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

func TestExtractQuestion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
		found  bool
	}{
		{name: "no block", output: "done\n<delegate-summary>\nok\n</delegate-summary>\n", found: false},
		{
			name:   "returns the text inside the question block",
			output: "<delegate-question>\nKeep the old flag as an alias?\n</delegate-question>\n",
			want:   "Keep the old flag as an alias?",
			found:  true,
		},
		{name: "empty block is no question", output: "<delegate-question>\n  \n</delegate-question>", found: false},
		{
			name: "question and summary side by side",
			output: "<delegate-summary>\nDid steps 1-2.\n</delegate-summary>\n" +
				"<delegate-question>\nStep 3: which timezone?\n</delegate-question>\n",
			want:  "Step 3: which timezone?",
			found: true,
		},
		{
			// A worker cut off mid-block still asked something worth reading.
			name:   "unclosed block takes the rest of the output",
			output: "<delegate-question>\nWhich DB?",
			want:   "Which DB?",
			found:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := runner.ExtractQuestion(tt.output)

			assert.Equal(t, tt.found, found)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptTellsTheWorkerHowToAsk(t *testing.T) {
	assert.Contains(t, runner.SummaryInstruction, "<delegate-question>")
}
