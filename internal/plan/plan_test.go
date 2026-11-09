package plan_test

import (
	"testing"

	"github.com/afplana/delegate/internal/plan"

	"github.com/stretchr/testify/assert"
)

const complete = `# Goal
Rename the greeting.

# Steps
1. Edit a.txt.

# Acceptance criteria
- ` + "`grep -qx hi a.txt`" + ` exits 0.
`

func TestMissing(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want []string
	}{
		{name: "complete plan", md: complete, want: nil},
		{
			name: "missing sections are listed in template order",
			md:   "# Goal\nDo it.\n",
			want: []string{"Steps", "Acceptance criteria"},
		},
		{name: "no headings at all", md: "Create new.txt", want: []string{"Goal", "Steps", "Acceptance criteria"}},
		{
			// A heading with nothing under it gives the worker nothing to check against.
			name: "empty section counts as missing",
			md:   "# Goal\nDo it.\n# Steps\n1. Edit.\n# Acceptance criteria\n\n# Constraints\n- none\n",
			want: []string{"Acceptance criteria"},
		},
		{
			name: "heading level and case do not matter",
			md:   "## goal\nDo it.\n### STEPS\n1. Edit.\n# Acceptance Criteria\n- passes\n",
			want: nil,
		},
		{
			// Shell comments in a code sample look like headings.
			name: "headings inside code fences are ignored",
			md:   "# Goal\nDo it.\n```sh\n# Steps\necho\n# Acceptance criteria\n```\n",
			want: []string{"Steps", "Acceptance criteria"},
		},
		{name: "a hash without a space is not a heading", md: "#Goal\nDo it.\n", want: []string{"Goal", "Steps", "Acceptance criteria"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, plan.Parse(tt.md).Missing())
		})
	}
}

func TestSection_ReturnsBodyWithoutTheHeading(t *testing.T) {
	// Card 19 reads the # Files section through this.
	p := plan.Parse("# Goal\nDo it.\n\n# Files\ninternal/x.go\ncmd/**\n")

	body, ok := p.Section("files")

	assert.True(t, ok)
	assert.Equal(t, "internal/x.go\ncmd/**", body)
	_, ok = p.Section("Steps")
	assert.False(t, ok)
}

func TestFiles_AcceptsBareBulletedAndBacktickedPaths(t *testing.T) {
	p := plan.Parse("# Files\ninternal/x.go\n- `cmd/**`\n* docs/\n\n")

	assert.Equal(t, []string{"internal/x.go", "cmd/**", "docs/"}, p.Files())
}

func TestFiles_NoSectionIsNil(t *testing.T) {
	assert.Nil(t, plan.Parse(complete).Files())
}

func TestOutOfScope(t *testing.T) {
	allowed := []string{"internal/plan/plan.go", "cmd/**", "docs/", "internal/runner/*_test.go"}
	tests := []struct {
		name    string
		changed []string
		allowed []string
		want    []string
	}{
		{name: "exact path is in scope", changed: []string{"internal/plan/plan.go"}, allowed: allowed, want: nil},
		{name: "dir/** covers nested files", changed: []string{"cmd/delegate/run.go"}, allowed: allowed, want: nil},
		{name: "trailing slash covers the directory", changed: []string{"docs/design.md"}, allowed: allowed, want: nil},
		{name: "single-segment glob", changed: []string{"internal/runner/verify_test.go"}, allowed: allowed, want: nil},
		{
			name:    "files matching nothing are listed in order",
			changed: []string{"go.mod", "cmd/x.go", "internal/runner/verify.go"},
			allowed: allowed,
			want:    []string{"go.mod", "internal/runner/verify.go"},
		},
		{
			// "cmd/**" must not match "cmdx/a.go" just because the prefix lines up.
			name:    "directory prefix stops at the slash",
			changed: []string{"cmdx/a.go"},
			allowed: allowed,
			want:    []string{"cmdx/a.go"},
		},
		{name: "no allowlist means no check", changed: []string{"anything.go"}, allowed: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, plan.OutOfScope(tt.changed, tt.allowed))
		})
	}
}
