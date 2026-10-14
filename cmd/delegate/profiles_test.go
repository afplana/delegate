package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/afplana/delegate/internal/config"
	"github.com/afplana/delegate/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfilesListsBinaryAvailability(t *testing.T) {
	profiles := map[string]runner.Profile{
		config.DefaultProfile: {Name: config.DefaultProfile, Command: "sh"},
		"absent":              {Name: "absent", Command: "delegate-no-such-binary"},
	}
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"profiles"}, &stdout, &stderr, profiles)

	assert.Equal(t, 0, code)
	assert.Empty(t, stderr.String())
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, []string{"PROFILE", "COMMAND", "BINARY"}, strings.Fields(lines[0]))

	// "absent" sorts before the default profile.
	absent := strings.Fields(lines[1])
	require.Len(t, absent, 3)
	assert.Equal(t, "absent", absent[0])
	assert.Equal(t, "missing", absent[2])

	assert.Contains(t, lines[2], "(default)")
	def := strings.Fields(lines[2])
	require.Len(t, def, 4)
	assert.Equal(t, config.DefaultProfile, def[0])
	assert.Equal(t, "sh", def[2])
	assert.True(t, strings.HasSuffix(def[3], "/sh"), "binary path %q should end in /sh", def[3])
}
