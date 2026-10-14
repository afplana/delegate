package config_test

import (
	"testing"

	"github.com/afplana/delegate/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup_FindsBuiltinProfiles(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{name: "cursor", command: "cursor-agent"},
		{name: "claude-sonnet", command: "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := config.Lookup(config.Builtin(), tt.name)

			require.NoError(t, err)
			assert.Equal(t, tt.name, profile.Name)
			assert.Equal(t, tt.command, profile.Command)
			assert.Contains(t, profile.Args, "{prompt}", "the task must reach the worker")
			assert.Positive(t, profile.Timeout, "a stuck worker must not hang the planner forever")
		})
	}
}

func TestLookup_DefaultProfileExists(t *testing.T) {
	_, err := config.Lookup(config.Builtin(), config.DefaultProfile)

	assert.NoError(t, err)
}

func TestLookup_UnknownProfileListsKnownNames(t *testing.T) {
	_, err := config.Lookup(config.Builtin(), "nope")

	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown profile "nope"`)
	assert.ErrorContains(t, err, "claude-sonnet, cursor")
}
