package runner_test

import (
	"testing"

	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cursorJSON is real `cursor-agent -p --output-format json` output (2026-11-10).
const cursorJSON = `{"type":"result","subtype":"success","is_error":false,"duration_ms":9827,` +
	`"duration_api_ms":9827,"result":"ok","session_id":"s","request_id":"r",` +
	`"usage":{"inputTokens":17070,"outputTokens":33,"cacheReadTokens":64,"cacheWriteTokens":0}}`

// claudeJSON follows `claude -p --output-format json` as recorded on 2026-10-23.
const claudeJSON = `{"type":"result","subtype":"success","is_error":false,"duration_ms":5000,` +
	`"num_turns":3,"result":"done\n<delegate-summary>\nok\n</delegate-summary>","total_cost_usd":0.0123,` +
	`"usage":{"input_tokens":120,"output_tokens":45,"cache_read_input_tokens":9000,"cache_creation_input_tokens":300}}`

func TestParseCursorJSON(t *testing.T) {
	reply, usage, err := runner.ParseCursorJSON(cursorJSON)

	require.NoError(t, err)
	assert.Equal(t, "ok", reply)
	assert.Equal(t, &runner.Usage{InputTokens: 17070, OutputTokens: 33, CacheReadTokens: 64}, usage)
	assert.Nil(t, usage.CostUSD, "cursor reports no cost")
}

func TestParseClaudeJSON(t *testing.T) {
	reply, usage, err := runner.ParseClaudeJSON(claudeJSON)

	require.NoError(t, err)
	assert.Equal(t, "done\n<delegate-summary>\nok\n</delegate-summary>", reply)
	require.NotNil(t, usage.CostUSD)
	assert.InDelta(t, 0.0123, *usage.CostUSD, 1e-9)
	assert.Equal(t, int64(120), usage.InputTokens)
	assert.Equal(t, int64(45), usage.OutputTokens)
	assert.Equal(t, int64(9000), usage.CacheReadTokens)
	assert.Equal(t, int64(300), usage.CacheWriteTokens)
}

func TestParsers_RejectNonJSON(t *testing.T) {
	// A worker that dies early prints plain text; the caller falls back to it.
	for name, parse := range map[string]runner.ParseFunc{"claude": runner.ParseClaudeJSON, "cursor": runner.ParseCursorJSON} {
		t.Run(name, func(t *testing.T) {
			_, _, err := parse("Failed to authenticate: OAuth session expired\n")

			assert.Error(t, err)
		})
	}
}
