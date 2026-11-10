package config_test

import (
	"slices"
	"testing"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"

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
			assert.Equal(t, runner.PromptStdin, profile.PromptVia, "stdin has no argv size limit")
			assert.NotContains(t, profile.Args, "{prompt}", "the prompt goes on stdin, not twice")
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

func TestBuiltin_ClaudeProfilesIgnoreTheUsersPersonalSetup(t *testing.T) {
	// A worker that loads the user's output style or MCP servers may leave
	// TODO(human) gaps or stop to ask; probe on 2026-10-19 showed both load.
	for _, tt := range claudeProfiles {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := config.Lookup(config.Builtin(), tt.name)

			require.NoError(t, err)
			i := slices.Index(profile.Args, "--setting-sources")
			require.NotEqual(t, -1, i, "--setting-sources must be set")
			require.Less(t, i+1, len(profile.Args))
			assert.Equal(t, "project", profile.Args[i+1], "user settings (output style, hooks) stay out")
			assert.Contains(t, profile.Args, "--strict-mcp-config", "the user's MCP servers stay out")
			assert.NotContains(t, profile.Args, "--bare", "--bare skips OAuth and keychain auth")
		})
	}
}

func TestBuiltin_ClaudeProfilesEndWithTheAllowedTools(t *testing.T) {
	// --allowedTools is variadic and swallows any argument after it, so it
	// must stay last now that no prompt argument follows.
	for _, tt := range claudeProfiles {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := config.Lookup(config.Builtin(), tt.name)

			require.NoError(t, err)
			i := slices.Index(profile.Args, "--allowedTools")
			require.NotEqual(t, -1, i)
			for _, arg := range profile.Args[i+1:] {
				assert.NotContains(t, arg, "--", "only tool names follow --allowedTools")
			}
		})
	}
}

func TestBuiltin_JSONOutputAlwaysHasAParser(t *testing.T) {
	// Without a parser the planner would get raw JSON as the summary.
	for name, profile := range config.Builtin() {
		t.Run(name, func(t *testing.T) {
			i := slices.Index(profile.Args, "--output-format")
			require.NotEqual(t, -1, i)
			require.Less(t, i+1, len(profile.Args))
			assert.Equal(t, "json", profile.Args[i+1])
			assert.NotNil(t, profile.Parse)
		})
	}
}
