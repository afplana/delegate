package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/afplana/delegate/skills"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillInstall_UsesClaudeConfigDir(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"skill", "install"}, &stdout, &stderr, nil)

	require.Equal(t, 0, code, "stderr: %s", stderr.String())
	target := filepath.Join(config, "skills", "delegate", "SKILL.md")
	assert.Equal(t, "installed "+target+"\n", stdout.String())
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, skills.SkillMD, got)
}

func TestSkillInstall_RefusesLocalChangesWithoutForce(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	dir := filepath.Join(config, "skills", "delegate")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("curl copy\n"), 0o600))
	var stdout, stderr bytes.Buffer

	assert.Equal(t, 1, runCommands(t.Context(), []string{"skill", "install"}, &stdout, &stderr, nil))
	assert.Contains(t, stderr.String(), "--force")
	assert.Equal(t, 0, runCommands(t.Context(), []string{"skill", "install", "--force"}, &stdout, &stderr, nil))
	assert.Contains(t, stdout.String(), "upgraded")
}

func TestSkill_UnknownSubcommandIsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	assert.Equal(t, 2, runCommands(t.Context(), []string{"skill"}, &stdout, &stderr, nil))
	assert.Equal(t, 2, runCommands(t.Context(), []string{"skill", "remove"}, &stdout, &stderr, nil))
}
