package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/afplana/delegate/internal/history"
)

// runReview implements `delegate review <run-id> <verdict> [note...]`.
func runReview(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintf(stderr, "usage: delegate review <run-id> <%s> [note...]\n", strings.Join(history.Verdicts, "|"))
		return 2
	}
	runs, err := runsDir()
	if err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}
	id, verdict, note := args[0], args[1], strings.Join(args[2:], " ")
	if err := history.WriteReview(runs, id, verdict, note, time.Now()); err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "recorded %s for run %s\n", verdict, id)
	return 0
}

// runStats implements `delegate stats`: one row per profile.
func runStats(stdout, stderr io.Writer) int {
	runs, err := runsDir()
	if err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}
	loaded, skipped, err := history.Load(runs)
	if err != nil {
		fmt.Fprintf(stderr, "delegate: %v\n", err)
		return 1
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tRUNS\tOK\tNEEDS_INPUT\tFAILED\tTIMEOUT\tERROR\tACCEPTED/REVIEWED\tMEDIAN_IN\tMEDIAN_OUT\tCOST_USD")
	for _, s := range history.Summarize(loaded) {
		medianIn, medianOut := "-", "-"
		if s.WithUsage > 0 {
			medianIn, medianOut = fmt.Sprint(s.MedianInput), fmt.Sprint(s.MedianOutput)
		}
		cost := "-"
		if s.CostUSD > 0 {
			cost = fmt.Sprintf("%.4f", s.CostUSD)
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d/%d\t%s\t%s\t%s\n",
			s.Profile, s.Runs, s.OK, s.NeedsInput, s.Failed, s.Timeout, s.Error,
			s.Accepted, s.Reviewed, medianIn, medianOut, cost)
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	if len(skipped) > 0 {
		fmt.Fprintf(stdout, "skipped %d run dir(s) without a readable result.json: %s\n", len(skipped), strings.Join(skipped, ", "))
	}
	return 0
}

func runsDir() (string, error) {
	state, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(state, "runs"), nil
}
