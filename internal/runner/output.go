package runner

import (
	"encoding/json"
	"fmt"
)

// Usage is what one worker run consumed, as the worker CLI reports it.
type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	// CostUSD is nil when the CLI reports no cost (Cursor).
	CostUSD *float64
}

// ParseFunc reads a worker's stdout and returns its final reply text and
// usage. It returns an error when stdout is not in the expected shape, e.g. a
// worker that crashed before printing JSON; callers then use stdout as is.
type ParseFunc func(stdout string) (reply string, usage *Usage, err error)

// ParseClaudeJSON reads `claude -p --output-format json` output.
func ParseClaudeJSON(stdout string) (string, *Usage, error) {
	var out struct {
		Result       string   `json:"result"`
		TotalCostUSD *float64 `json:"total_cost_usd"`
		Usage        struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return "", nil, fmt.Errorf("parse claude json: %w", err)
	}
	return out.Result, &Usage{
		InputTokens:      out.Usage.InputTokens,
		OutputTokens:     out.Usage.OutputTokens,
		CacheReadTokens:  out.Usage.CacheReadInputTokens,
		CacheWriteTokens: out.Usage.CacheCreationInputTokens,
		CostUSD:          out.TotalCostUSD,
	}, nil
}

// ParseCursorJSON reads `cursor-agent -p --output-format json` output.
// Cursor reports tokens but no cost.
func ParseCursorJSON(stdout string) (string, *Usage, error) {
	var out struct {
		Result string `json:"result"`
		Usage  struct {
			InputTokens      int64 `json:"inputTokens"`
			OutputTokens     int64 `json:"outputTokens"`
			CacheReadTokens  int64 `json:"cacheReadTokens"`
			CacheWriteTokens int64 `json:"cacheWriteTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return "", nil, fmt.Errorf("parse cursor json: %w", err)
	}
	return out.Result, &Usage{
		InputTokens:      out.Usage.InputTokens,
		OutputTokens:     out.Usage.OutputTokens,
		CacheReadTokens:  out.Usage.CacheReadTokens,
		CacheWriteTokens: out.Usage.CacheWriteTokens,
	}, nil
}
