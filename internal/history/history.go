// Package history reads past runs from the state dir and records how their
// review went, so the choice of worker profile can rest on data.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Verdicts are the review outcomes `delegate review` accepts.
var Verdicts = []string{"accepted", "needs-fixes", "failed"}

const (
	resultFile = "result.json"
	reviewFile = "review.json"
)

// Usage is the part of a run's token usage that stats aggregate.
type Usage struct {
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	CostUSD      *float64 `json:"cost_usd"`
}

// Run is one past run: what result.json said, plus its review if any.
type Run struct {
	ID      string `json:"run_id"`
	Profile string `json:"profile"`
	Status  string `json:"status"`
	Usage   *Usage `json:"usage"`
	Verdict string `json:"-"`
	Note    string `json:"-"`
}

type review struct {
	Verdict    string `json:"verdict"`
	Note       string `json:"note,omitempty"`
	ReviewedAt string `json:"reviewed_at"`
}

// WriteReview records the verdict for run id, replacing any earlier one.
func WriteReview(runsDir, id, verdict, note string, now time.Time) error {
	if !slices.Contains(Verdicts, verdict) {
		return fmt.Errorf("unknown verdict %q (want %s)", verdict, strings.Join(Verdicts, ", "))
	}
	// The id names a directory: anything path-like could escape runsDir.
	if id == "" || id != filepath.Base(id) || strings.HasPrefix(id, ".") {
		return fmt.Errorf("no run %q", id)
	}
	dir := filepath.Join(runsDir, id)
	if _, err := os.Stat(filepath.Join(dir, resultFile)); err != nil {
		return fmt.Errorf("no run %q", id)
	}
	data, err := json.Marshal(review{Verdict: verdict, Note: note, ReviewedAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return fmt.Errorf("encode review: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, reviewFile), data, 0o600); err != nil {
		return fmt.Errorf("write review: %w", err)
	}
	return nil
}

// Load reads every run under runsDir. Run dirs without a readable
// result.json are returned in skipped, so callers can say they exist instead
// of silently leaving them out. A missing runsDir means no runs yet.
func Load(runsDir string) (runs []Run, skipped []string, err error) {
	entries, err := os.ReadDir(runsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read runs: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(runsDir, entry.Name())
		var run Run
		data, err := os.ReadFile(filepath.Join(dir, resultFile))
		if err != nil || json.Unmarshal(data, &run) != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		run.ID = entry.Name()
		var rev review
		if data, err := os.ReadFile(filepath.Join(dir, reviewFile)); err == nil && json.Unmarshal(data, &rev) == nil {
			run.Verdict, run.Note = rev.Verdict, rev.Note
		}
		runs = append(runs, run)
	}
	return runs, skipped, nil
}

// ProfileStats aggregates the runs of one profile. Medians cover the runs
// that recorded usage (WithUsage); CostUSD sums the runs that reported cost.
type ProfileStats struct {
	Profile                                      string
	Runs, OK, NeedsInput, Failed, Timeout, Error int
	Reviewed, Accepted                           int
	WithUsage                                    int
	MedianInput, MedianOutput                    int64
	CostUSD                                      float64
}

// Summarize groups runs by profile, sorted by profile name.
func Summarize(runs []Run) []ProfileStats {
	byProfile := map[string][]Run{}
	for _, r := range runs {
		byProfile[r.Profile] = append(byProfile[r.Profile], r)
	}
	stats := make([]ProfileStats, 0, len(byProfile))
	for _, name := range slices.Sorted(maps.Keys(byProfile)) {
		s := ProfileStats{Profile: name}
		var inputs, outputs []int64
		for _, r := range byProfile[name] {
			s.Runs++
			switch r.Status {
			case "ok":
				s.OK++
			case "needs_input":
				s.NeedsInput++
			case "failed":
				s.Failed++
			case "timeout":
				s.Timeout++
			case "error":
				s.Error++
			}
			if r.Verdict != "" {
				s.Reviewed++
				if r.Verdict == "accepted" {
					s.Accepted++
				}
			}
			if r.Usage != nil {
				inputs = append(inputs, r.Usage.InputTokens)
				outputs = append(outputs, r.Usage.OutputTokens)
				if r.Usage.CostUSD != nil {
					s.CostUSD += *r.Usage.CostUSD
				}
			}
		}
		s.WithUsage = len(inputs)
		s.MedianInput, s.MedianOutput = median(inputs), median(outputs)
		stats = append(stats, s)
	}
	return stats
}

// median returns the middle value, or the mean of the middle two; 0 for none.
func median(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(values))
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
