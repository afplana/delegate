package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionPrintsBuildVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runCommands(t.Context(), []string{"version"}, &stdout, &stderr, withProfiles(nil))

	assert.Equal(t, 0, code)
	assert.Equal(t, "dev\n", stdout.String())
	assert.Empty(t, stderr.String())
}

func TestResolveVersion(t *testing.T) {
	// `go install ...@v0.1.0` passes no -ldflags, but Go records the module
	// version in the binary; a local `go build` records "(devel)".
	tests := []struct {
		name, ldflag, module, want string
	}{
		{name: "ldflags win", ldflag: "v0.1.0-3-gabc1234", module: "v0.1.0", want: "v0.1.0-3-gabc1234"},
		{name: "go install from a tag", ldflag: "dev", module: "v0.1.0", want: "v0.1.0"},
		{name: "local go build", ldflag: "dev", module: "(devel)", want: "dev"},
		{name: "no build info", ldflag: "dev", module: "", want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveVersion(tt.ldflag, tt.module))
		})
	}
}
