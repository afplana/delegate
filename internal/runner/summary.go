package runner

import "strings"

const (
	summaryOpenTag  = "<delegate-summary>"
	summaryCloseTag = "</delegate-summary>"

	// summaryTailLines is how much of the transcript the planner sees when
	// the worker skips the summary block.
	summaryTailLines = 20
)

// WorkerContract is prepended to every task prompt, whatever the CLI: the
// worker runs unattended, so nobody can answer a question, and its work is
// reviewed by the planner before anything leaves the worktree.
const WorkerContract = "You are a delegate worker: an agent running unattended on the plan below, " +
	"in a git worktree made for this task. Nobody will answer questions, so never ask one or wait for input. " +
	"Do what the plan makes unambiguous; when a choice that matters is unclear, do not guess: " +
	"leave it undone and name it in your report. Stay inside the current directory. " +
	"Do not push, and do not start another delegate run.\n\n"

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
		return lastLines(output, summaryTailLines)
	}

	inside, _, _ := strings.Cut(rest, summaryCloseTag)
	return strings.TrimSpace(inside)
}

// lastLines returns the last n lines of s, trimmed of surrounding whitespace.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
