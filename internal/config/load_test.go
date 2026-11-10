package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestWithUserFile_MissingFileKeepsTheBuiltins(t *testing.T) {
	cfg, err := config.Defaults().WithUserFile(filepath.Join(t.TempDir(), "none.yaml"))

	require.NoError(t, err)
	assert.Equal(t, config.DefaultProfile, cfg.Default)
	assert.Len(t, cfg.Profiles, len(config.Builtin()))
}

func TestWithUserFile_AddsAProfile(t *testing.T) {
	path := writeYAML(t, `
profiles:
  codex:
    command: codex
    args: ["exec", "{prompt}"]
    timeout: 10m
`)

	cfg, err := config.Defaults().WithUserFile(path)

	require.NoError(t, err)
	p, err := config.Lookup(cfg.Profiles, "codex")
	require.NoError(t, err)
	assert.Equal(t, "codex", p.Name)
	assert.Equal(t, "codex", p.Command)
	assert.Equal(t, []string{"exec", "{prompt}"}, p.Args)
	assert.Equal(t, 10*time.Minute, p.Timeout)
	assert.Nil(t, p.Parse, "no output key means plain text")
}

func TestWithUserFile_OverridesOneFieldOfABuiltin(t *testing.T) {
	path := writeYAML(t, "profiles:\n  cursor:\n    timeout: 45m\n")

	cfg, err := config.Defaults().WithUserFile(path)

	require.NoError(t, err)
	got := cfg.Profiles["cursor"]
	want := config.Builtin()["cursor"]
	assert.Equal(t, 45*time.Minute, got.Timeout)
	assert.Equal(t, want.Command, got.Command, "untouched fields stay")
	assert.Equal(t, want.Args, got.Args)
	assert.NotNil(t, got.Parse, "the built-in parser stays")
	assert.Equal(t, 30*time.Minute, config.Builtin()["cursor"].Timeout, "built-ins are never mutated")
}

func TestWithUserFile_SetsDefaultProfileAndOutput(t *testing.T) {
	path := writeYAML(t, `
default_profile: mine
profiles:
  mine:
    command: claude
    args: ["-p", "--output-format", "json", "{prompt}"]
    output: claude-json
`)

	cfg, err := config.Defaults().WithUserFile(path)

	require.NoError(t, err)
	assert.Equal(t, "mine", cfg.Default)
	assert.NotNil(t, cfg.Profiles["mine"].Parse)
}

func TestWithUserFile_PromptVia(t *testing.T) {
	path := writeYAML(t, `
profiles:
  piped:
    command: my-agent
    args: ["--headless"]
    prompt_via: stdin
  classic:
    command: my-agent
    args: ["{prompt}"]
    prompt_via: arg
  plain:
    command: my-agent
    args: ["{prompt}"]
`)

	cfg, err := config.Defaults().WithUserFile(path)

	require.NoError(t, err)
	assert.Equal(t, runner.PromptStdin, cfg.Profiles["piped"].PromptVia)
	assert.Equal(t, runner.PromptArg, cfg.Profiles["classic"].PromptVia)
	assert.Equal(t, runner.PromptArg, cfg.Profiles["plain"].PromptVia, "arg is the default for a new profile")
}

func TestWithUserFile_SwitchingABuiltinToArgsNeedsThePlaceholder(t *testing.T) {
	// The built-ins read stdin and have no {prompt}; flipping only prompt_via
	// would start a worker that never sees the task.
	path := writeYAML(t, "profiles:\n  cursor:\n    prompt_via: arg\n")

	_, err := config.Defaults().WithUserFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "profiles.cursor.args: must contain {prompt}")
}

func TestWithUserFile_Errors(t *testing.T) {
	tests := []struct {
		name, yaml, want string
	}{
		{name: "bad duration", yaml: "profiles:\n  cursor:\n    timeout: soon\n", want: "profiles.cursor.timeout"},
		{name: "unknown key", yaml: "profiles:\n  cursor:\n    comand: x\n", want: "comand"},
		{name: "unknown output", yaml: "profiles:\n  cursor:\n    output: xml\n", want: `profiles.cursor.output: unknown "xml" (want text, claude-json, cursor-json)`},
		{name: "new profile without command", yaml: "profiles:\n  x:\n    args: [\"{prompt}\"]\n", want: "profiles.x.command: required for a new profile"},
		{name: "args without the prompt", yaml: "profiles:\n  x:\n    command: x\n    args: [\"-p\"]\n", want: "profiles.x.args: must contain {prompt}"},
		{name: "unknown default", yaml: "default_profile: nope\n", want: `default_profile: unknown profile "nope"`},
		{name: "unknown prompt_via", yaml: "profiles:\n  x:\n    command: x\n    args: [\"{prompt}\"]\n    prompt_via: file\n", want: `profiles.x.prompt_via: unknown "file" (want arg, stdin)`},
		{name: "stdin plus placeholder", yaml: "profiles:\n  x:\n    command: x\n    args: [\"{prompt}\"]\n    prompt_via: stdin\n", want: "profiles.x.args: {prompt} is not replaced when prompt_via is stdin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, tt.yaml)

			_, err := config.Defaults().WithUserFile(path)

			require.Error(t, err)
			assert.Contains(t, err.Error(), path, "errors name the file")
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestWithRepoFile_AllowsOnlyDefaultProfileAndTimeout(t *testing.T) {
	path := writeYAML(t, "default_profile: claude-sonnet\nprofiles:\n  cursor:\n    timeout: 5m\n")

	cfg, err := config.Defaults().WithRepoFile(path)

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet", cfg.Default)
	assert.Equal(t, 5*time.Minute, cfg.Profiles["cursor"].Timeout)
}

func TestWithRepoFile_RefusesToChooseWhatRuns(t *testing.T) {
	// A cloned repo must not be able to pick the program delegate executes.
	tests := []struct {
		name, yaml, want string
	}{
		{name: "command", yaml: "profiles:\n  cursor:\n    command: sh\n", want: "profiles.cursor.command: not allowed in a repo file"},
		{name: "args", yaml: "profiles:\n  cursor:\n    args: [\"--force\", \"{prompt}\"]\n", want: "profiles.cursor.args: not allowed in a repo file"},
		{name: "output", yaml: "profiles:\n  cursor:\n    output: text\n", want: "profiles.cursor.output: not allowed in a repo file"},
		{name: "prompt_via", yaml: "profiles:\n  cursor:\n    prompt_via: arg\n", want: "profiles.cursor.prompt_via: not allowed in a repo file"},
		{name: "new profile", yaml: "profiles:\n  evil:\n    timeout: 1m\n", want: `profiles.evil: a repo file cannot add profiles`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAML(t, tt.yaml)

			_, err := config.Defaults().WithRepoFile(path)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
