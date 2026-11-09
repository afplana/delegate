package config_test

import (
	"slices"
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
		{name: "claude-haiku", command: "claude"},
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

func TestBuiltin_CursorRunsWithoutForce(t *testing.T) {
	// Org-managed Cursor accounts can disable --force ("Run Everything");
	// with it every cursor run fails within seconds.
	profile, err := config.Lookup(config.Builtin(), "cursor")

	require.NoError(t, err)
	assert.NotContains(t, profile.Args, "--force")
}

func TestLookup_DefaultProfileExists(t *testing.T) {
	_, err := config.Lookup(config.Builtin(), config.DefaultProfile)

	assert.NoError(t, err)
}

func TestLookup_UnknownProfileListsKnownNames(t *testing.T) {
	_, err := config.Lookup(config.Builtin(), "nope")

	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown profile "nope"`)
	assert.ErrorContains(t, err, "claude-haiku, claude-sonnet, cursor")
}

// claudeProfiles share claudeArgs, so every rule below must hold for each.
var claudeProfiles = []struct {
	name  string
	model string
}{
	{name: "claude-sonnet", model: "sonnet"},
	{name: "claude-haiku", model: "haiku"},
}

func TestBuiltin_ClaudeProfilesPickTheirModel(t *testing.T) {
	for _, tt := range claudeProfiles {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := config.Lookup(config.Builtin(), tt.name)

			require.NoError(t, err)
			i := slices.Index(profile.Args, "--model")
			require.NotEqual(t, -1, i, "--model must be set")
			require.Less(t, i+1, len(profile.Args))
			assert.Equal(t, tt.model, profile.Args[i+1])
		})
	}
}

func TestBuiltin_ClaudeProfilesCanBuildAndCommitButNotPush(t *testing.T) {
	// Print mode denies every shell command not allow-listed, so without
	// these the worker cannot run go, make or git and stops in its first task.
	for _, tt := range claudeProfiles {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := config.Lookup(config.Builtin(), tt.name)

			require.NoError(t, err)
			assert.Contains(t, profile.Args, "acceptEdits")
			assert.Contains(t, profile.Args, "--allowedTools")
			for _, tool := range []string{"Bash(go *)", "Bash(make *)", "Bash(git add *)", "Bash(git commit *)"} {
				assert.Contains(t, profile.Args, tool)
			}
			for _, arg := range profile.Args {
				assert.NotContains(t, arg, "push", "pushing stays with the planner")
				assert.NotContains(t, arg, "bypassPermissions")
				assert.NotContains(t, arg, "dangerously")
			}
		})
	}
}

func TestBuiltin_ClaudeProfilesEndOptionsBeforePrompt(t *testing.T) {
	// --allowedTools is variadic; without "--" it takes the prompt as a tool
	// name and claude exits with "Input must be provided".
	for _, tt := range claudeProfiles {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := config.Lookup(config.Builtin(), tt.name)

			require.NoError(t, err)
			require.GreaterOrEqual(t, len(profile.Args), 2)
			assert.Equal(t, []string{"--", "{prompt}"}, profile.Args[len(profile.Args)-2:])
		})
	}
}
