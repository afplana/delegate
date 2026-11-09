// Package plan reads the markdown plan a planner hands to delegate.
package plan

import (
	"path"
	"strings"
)

// Required are the sections every plan needs, in the skill template's order.
// Without acceptance criteria neither the worker nor the reviewer knows when
// the task is done; ambiguity is the main reason workers drift.
var Required = []string{"Goal", "Steps", "Acceptance criteria"}

// Plan is a parsed plan: section bodies keyed by lower-cased heading.
type Plan struct {
	sections map[string]string
}

// Parse splits md into sections at markdown headings of any level. Lines
// inside ``` or ~~~ fences are body text, never headings: a shell comment in
// a code sample must not open a section.
func Parse(md string) Plan {
	p := Plan{sections: map[string]string{}}
	var current string
	var body []string
	inSection, inFence := false, false
	flush := func() {
		if inSection {
			p.sections[current] = strings.TrimSpace(strings.Join(body, "\n"))
		}
	}

	for line := range strings.Lines(md) {
		line = strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if name, ok := heading(trimmed); ok && !inFence {
			flush()
			current, body, inSection = strings.ToLower(name), nil, true
			continue
		}
		body = append(body, line)
	}
	flush()
	return p
}

// heading returns the text of a markdown ATX heading ("## Steps" -> "Steps").
func heading(line string) (string, bool) {
	text := strings.TrimLeft(line, "#")
	if text == line || !strings.HasPrefix(text, " ") {
		return "", false
	}
	return strings.TrimSpace(text), true
}

// Section returns the body of the section called name, ignoring case.
func (p Plan) Section(name string) (string, bool) {
	body, ok := p.sections[strings.ToLower(name)]
	return body, ok
}

// Missing lists the Required sections that are absent or empty, in order.
func (p Plan) Missing() []string {
	var missing []string
	for _, name := range Required {
		if body, _ := p.Section(name); body == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// Files returns the entries of the optional "Files" section: the paths the
// worker may change. List markers and backticks are stripped, so "- `cmd/**`"
// reads as "cmd/**". No section means nil: no scope check.
func (p Plan) Files() []string {
	body, ok := p.Section("Files")
	if !ok {
		return nil
	}
	var files []string
	for line := range strings.Lines(body) {
		entry := strings.TrimSpace(line)
		entry = strings.TrimPrefix(entry, "- ")
		entry = strings.TrimPrefix(entry, "* ")
		entry = strings.Trim(strings.TrimSpace(entry), "`")
		if entry != "" {
			files = append(files, entry)
		}
	}
	return files
}

// OutOfScope returns the changed files that match none of the allowed
// entries, in order. An empty allowlist means the plan set no scope, so
// nothing is out of it.
func OutOfScope(changed, allowed []string) []string {
	if len(allowed) == 0 {
		return nil
	}
	var out []string
	for _, file := range changed {
		if !matchesAny(file, allowed) {
			out = append(out, file)
		}
	}
	return out
}

// matchesAny reports whether file is an allowed entry, sits under an allowed
// "dir/" or "dir/**", or matches an allowed path.Match glob (one segment per *).
func matchesAny(file string, allowed []string) bool {
	for _, entry := range allowed {
		if dir, ok := strings.CutSuffix(entry, "**"); ok && strings.HasSuffix(dir, "/") {
			entry = dir
		}
		if strings.HasSuffix(entry, "/") && strings.HasPrefix(file, entry) {
			return true
		}
		// A malformed pattern only fails to match; the exact check still applies.
		if ok, _ := path.Match(entry, file); ok || entry == file {
			return true
		}
	}
	return false
}
