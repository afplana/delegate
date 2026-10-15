package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"

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
				require.Len(t, lines, 3)
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
				require.Len(t, lines, 3)
				assert.True(t, strings.HasPrefix(lineFor(lines, "git"), "ok"))
				assert.True(t, strings.HasPrefix(lineFor(lines, "state dir"), "ok"))
				def := lineFor(lines, "default profile")
				assert.True(t, strings.HasPrefix(def, "FAIL"))
				assert.Contains(t, def, "delegate-no-such-binary")
			},
		},
		{
			name:     "default profile not configured",
			profiles: map[string]runner.Profile{},
			wantCode: 1,
			assertOut: func(t *testing.T, lines []string) {
				require.Len(t, lines, 3)
				assert.True(t, strings.HasPrefix(lineFor(lines, "git"), "ok"))
				assert.True(t, strings.HasPrefix(lineFor(lines, "state dir"), "ok"))
				def := lineFor(lines, "default profile")
				assert.True(t, strings.HasPrefix(def, "FAIL"))
				assert.Contains(t, def, "unknown profile")
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
				require.Len(t, lines, 3)
				assert.True(t, strings.HasPrefix(lineFor(lines, "state dir"), "FAIL"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			} else {
				t.Setenv("XDG_STATE_HOME", t.TempDir())
			}

			var stdout, stderr bytes.Buffer
			code := runCommands(t.Context(), []string{"doctor"}, &stdout, &stderr, tt.profiles)

			assert.Equal(t, tt.wantCode, code)
			assert.Empty(t, stderr.String())
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			tt.assertOut(t, lines)
		})
	}
}
