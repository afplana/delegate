// Package plan reads the markdown plan a planner hands to delegate.
package plan

import (
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
