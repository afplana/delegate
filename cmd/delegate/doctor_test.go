package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"
	"github.com/afplana/delegate/skills"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctor(t *testing.T) {
	lineFor := func(lines []string, name string) string {
		prefix := name + ":"
		for _, line := range lines {
			if strings.Contains(line, prefix) {
				return line
			}
		}
		return ""
	}

	tests := []struct {
		name      string
		profiles  map[string]runner.Profile
		setup     func(t *testing.T)
		wantCode  int
		assertOut func(t *testing.T, lines []string)
	}{
		{
			name: "all checks pass",
			profiles: map[string]runner.Profile{
				config.DefaultProfile: {Name: config.DefaultProfile, Command: "sh"},
			},
			wantCode: 0,
			assertOut: func(t *testing.T, lines []string) {
				require.Len(t, lines, 4)
				for _, line := range lines {
					assert.True(t, strings.HasPrefix(line, "ok"), "line %q should start with ok", line)
				}
			},
		},
		{
			name: "default binary missing",
			profiles: map[string]runner.Profile{
				config.DefaultProfile: {Name: config.DefaultProfile, Command: "delegate-no-such-binary"},
			},
			wantCode: 1,
			assertOut: func(t *testing.T, lines []string) {
				require.Len(t, lines, 4)
				assert.True(t, strings.HasPrefix(lineFor(lines, "git"), "ok"))
				assert.True(t, strings.HasPrefix(lineFor(lines, "state dir"), "ok"))
				def := lineFor(lines, "default profile")
				assert.True(t, strings.HasPrefix(def, "FAIL"))
				assert.Contains(t, def, "delegate-no-such-binary")
			},
		},
		{
			name: "default binary missing names what is installed",
			profiles: map[string]runner.Profile{
				config.DefaultProfile: {Name: config.DefaultProfile, Command: "delegate-no-such-binary"},
				"other":               {Name: "other", Command: "sh"},
			},
			wantCode: 1,
			assertOut: func(t *testing.T, lines []string) {
				def := lineFor(lines, "default profile")
				assert.True(t, strings.HasPrefix(def, "FAIL"))
				assert.Contains(t, def, "installed profiles: other")
			},
		},
		{
			name:     "default profile not configured",
			profiles: map[string]runner.Profile{},
			wantCode: 1,
			assertOut: func(t *testing.T, lines []string) {
				require.Len(t, lines, 4)
				assert.True(t, strings.HasPrefix(lineFor(lines, "git"), "ok"))
				assert.True(t, strings.HasPrefix(lineFor(lines, "state dir"), "ok"))
				def := lineFor(lines, "default profile")
				assert.True(t, strings.HasPrefix(def, "FAIL"))
				assert.Contains(t, def, "unknown profile")
			},
		},
		{
			// A stale skill still works, so it warns without failing doctor.
			name: "installed skill differs from the binary's",
			profiles: map[string]runner.Profile{
				config.DefaultProfile: {Name: config.DefaultProfile, Command: "sh"},
			},
			setup: func(t *testing.T) {
				t.Setenv("XDG_STATE_HOME", t.TempDir())
				config := t.TempDir()
				t.Setenv("CLAUDE_CONFIG_DIR", config)
				dir := filepath.Join(config, "skills", "delegate")
				require.NoError(t, os.MkdirAll(dir, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("old\n"), 0o600))
			},
			wantCode: 0,
			assertOut: func(t *testing.T, lines []string) {
				require.Len(t, lines, 4)
				line := lineFor(lines, "skill")
				assert.True(t, strings.HasPrefix(line, "WARN"), "got %q", line)
				assert.Contains(t, line, "delegate skill install")
			},
		},
		{
			name: "state dir not writable",
			profiles: map[string]runner.Profile{
				config.DefaultProfile: {Name: config.DefaultProfile, Command: "sh"},
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.NoError(t, os.Chmod(dir, 0o500))
				t.Cleanup(func() {
					_ = os.Chmod(dir, 0o700)
				})
				t.Setenv("XDG_STATE_HOME", dir)
			},
			wantCode: 1,
			assertOut: func(t *testing.T, lines []string) {
				require.Len(t, lines, 4)
				assert.True(t, strings.HasPrefix(lineFor(lines, "state dir"), "FAIL"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Every case starts with this binary's skill installed.
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			_, err := skills.Install(skillDir(), false)
			require.NoError(t, err)
			if tt.setup != nil {
				tt.setup(t)
			} else {
				t.Setenv("XDG_STATE_HOME", t.TempDir())
			}

			var stdout, stderr bytes.Buffer
			code := runCommands(t.Context(), []string{"doctor"}, &stdout, &stderr, withProfiles(tt.profiles))

			assert.Equal(t, tt.wantCode, code)
			assert.Empty(t, stderr.String())
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			tt.assertOut(t, lines)
		})
	}
}
