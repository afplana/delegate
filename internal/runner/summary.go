package runner

import "strings"

const (
	summaryOpenTag  = "<delegate-summary>"
	summaryCloseTag = "</delegate-summary>"

	// summaryTailLines is how much of the transcript the planner sees when
	// the worker skips the summary block.
	summaryTailLines = 20
)

// SummaryInstruction is appended to every task prompt so the worker ends its
// output with a block that ExtractSummary can find.
const SummaryInstruction = "\n\nWhen you are done, end your reply with a short report for a reviewer " +
	"(what you changed, what you did not do, anything uncertain), wrapped exactly like this:\n" +
	summaryOpenTag + "\n...your report...\n" + summaryCloseTag + "\n"

// ExtractSummary returns the short report the planner reads instead of the
// worker's full transcript: the text inside <delegate-summary>...</delegate-summary>,
// or the last summaryTailLines lines when the worker did not write that block.
func ExtractSummary(output string) string {
	_, rest, found := strings.Cut(output, summaryOpenTag)
	if !found {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) > summaryTailLines {
			lines = lines[len(lines)-summaryTailLines:]
		}
		return strings.Join(lines, "\n")
	}

	inside, _, _ := strings.Cut(rest, summaryCloseTag)
	return strings.TrimSpace(inside)
}
