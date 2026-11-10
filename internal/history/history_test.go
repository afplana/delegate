package history_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/history"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRun creates runs/<id>/result.json with the given JSON.
func writeRun(t *testing.T, runsDir, id, resultJSON string) {
	t.Helper()
	dir := filepath.Join(runsDir, id)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "result.json"), []byte(resultJSON), 0o600))
}

func TestWriteReview_RecordsTheVerdict(t *testing.T) {
	runs := t.TempDir()
	writeRun(t, runs, "r1", `{"run_id":"r1","profile":"cursor","status":"ok"}`)
	now := time.Date(2026, 11, 10, 11, 0, 0, 0, time.UTC)

	require.NoError(t, history.WriteReview(runs, "r1", "accepted", "clean diff", now))

	loaded, skipped, err := history.Load(runs)
	require.NoError(t, err)
	assert.Empty(t, skipped)
	require.Len(t, loaded, 1)
	assert.Equal(t, "accepted", loaded[0].Verdict)
	assert.Equal(t, "clean diff", loaded[0].Note)
}

func TestWriteReview_LaterVerdictReplacesEarlier(t *testing.T) {
	runs := t.TempDir()
	writeRun(t, runs, "r1", `{"run_id":"r1","profile":"cursor","status":"ok"}`)

	require.NoError(t, history.WriteReview(runs, "r1", "needs-fixes", "", time.Now()))
	require.NoError(t, history.WriteReview(runs, "r1", "accepted", "", time.Now()))

	loaded, _, err := history.Load(runs)
	require.NoError(t, err)
	assert.Equal(t, "accepted", loaded[0].Verdict)
}

func TestWriteReview_Refuses(t *testing.T) {
	runs := t.TempDir()
	writeRun(t, runs, "r1", `{"run_id":"r1","profile":"cursor","status":"ok"}`)

	tests := []struct {
		name, id, verdict, wantErr string
	}{
		{name: "unknown verdict", id: "r1", verdict: "great", wantErr: `unknown verdict "great" (want accepted, needs-fixes, failed)`},
		{name: "unknown run", id: "nope", verdict: "accepted", wantErr: `no run "nope"`},
		{name: "path in the id", id: "../r1", verdict: "accepted", wantErr: `no run "../r1"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := history.WriteReview(runs, tt.id, tt.verdict, "", time.Now())

			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestLoad_ReportsRunDirsItCannotRead(t *testing.T) {
	// A broken run must show up as skipped, not vanish from the stats.
	runs := t.TempDir()
	writeRun(t, runs, "good", `{"run_id":"good","profile":"cursor","status":"ok"}`)
	writeRun(t, runs, "corrupt", `{not json`)
	require.NoError(t, os.MkdirAll(filepath.Join(runs, "no-result"), 0o700))

	loaded, skipped, err := history.Load(runs)

	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "good", loaded[0].ID)
	assert.ElementsMatch(t, []string{"corrupt", "no-result"}, skipped)
}

func TestLoad_MissingRunsDirIsEmpty(t *testing.T) {
	loaded, skipped, err := history.Load(filepath.Join(t.TempDir(), "runs"))

	require.NoError(t, err)
	assert.Empty(t, loaded)
	assert.Empty(t, skipped)
}

func TestSummarize(t *testing.T) {
	runs := []history.Run{
		{Profile: "cursor", Status: "ok", Verdict: "accepted", Usage: &history.Usage{InputTokens: 100, OutputTokens: 10}},
		{Profile: "cursor", Status: "ok", Verdict: "needs-fixes", Usage: &history.Usage{InputTokens: 300, OutputTokens: 30}},
		{Profile: "cursor", Status: "failed"},
		{Profile: "claude-sonnet", Status: "needs_input", Usage: &history.Usage{InputTokens: 50, OutputTokens: 5, CostUSD: new(0.25)}},
		{Profile: "claude-sonnet", Status: "ok", Verdict: "accepted", Usage: &history.Usage{InputTokens: 70, OutputTokens: 7, CostUSD: new(0.5)}},
		{Profile: "claude-sonnet", Status: "timeout"},
		{Profile: "claude-sonnet", Status: "error"},
	}

	got := history.Summarize(runs)

	require.Len(t, got, 2)
	assert.Equal(t, history.ProfileStats{
		Profile: "claude-sonnet", Runs: 4, OK: 1, NeedsInput: 1, Timeout: 1, Error: 1,
		Reviewed: 1, Accepted: 1, WithUsage: 2, MedianInput: 60, MedianOutput: 6, CostUSD: 0.75,
	}, got[0], "sorted by profile; median of an even count averages the middle two")
	assert.Equal(t, history.ProfileStats{
		Profile: "cursor", Runs: 3, OK: 2, Failed: 1,
		Reviewed: 2, Accepted: 1, WithUsage: 2, MedianInput: 200, MedianOutput: 20,
	}, got[1])
}

func TestOlderThan_ListsRunDirsStartedBeforeTheCutoff(t *testing.T) {
	runs := t.TempDir()
	for _, id := range []string{"20200101-000000-aaaa", "20991231-000000-bbbb", "not-a-run-id"} {
		require.NoError(t, os.MkdirAll(filepath.Join(runs, id), 0o700))
	}

	got, err := history.OlderThan(runs, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Equal(t, []string{"20200101-000000-aaaa"}, got, "names that are not run ids are never touched")
}
