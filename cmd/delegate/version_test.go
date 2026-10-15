package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionPrintsBuildVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"version"}, &stdout, &stderr, nil)

	assert.Equal(t, 0, code)
	assert.Equal(t, "dev\n", stdout.String())
	assert.Empty(t, stderr.String())
}
