// Package skills embeds the Claude Code skill that drives delegate and
// installs it as a plain copy, so the skill always matches the binary.
package skills

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SkillMD is the skill shipped with this binary.
//
//go:embed delegate/SKILL.md
var SkillMD []byte

// HashFile sits next to the installed SKILL.md and records the hash of what
// delegate wrote, so a later install can tell an untouched copy (safe to
// upgrade) from local edits (refused without force).
const HashFile = ".installed.sha256"

// ErrLocalChanges means the installed skill differs from what delegate last
// installed there, or delegate never installed it.
var ErrLocalChanges = errors.New("installed skill has changes delegate did not make")

// Outcome says what Install did.
type Outcome string

const (
	Installed Outcome = "installed"
	Upgraded  Outcome = "upgraded"
	UpToDate  Outcome = "up to date"
)

// Hash returns the hex SHA-256 of content.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Install copies SkillMD to dir/SKILL.md. It overwrites an existing copy only
// when force is set or the copy is exactly what delegate last installed.
func Install(dir string, force bool) (Outcome, error) {
	target := filepath.Join(dir, "SKILL.md")
	current, err := os.ReadFile(target)
	outcome := Installed
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", fmt.Errorf("read installed skill: %w", err)
	case bytes.Equal(current, SkillMD):
		// Record the hash anyway: a copy installed by other means has none,
		// and without it the next upgrade would wrongly need --force.
		if err := writeHash(dir); err != nil {
			return "", err
		}
		return UpToDate, nil
	case !force && !untouched(dir, current):
		return "", fmt.Errorf("%w: %s (pass --force to overwrite)", ErrLocalChanges, target)
	default:
		outcome = Upgraded
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create skill dir: %w", err)
	}
	// Remove first: if target is a symlink, WriteFile would write through it.
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("replace installed skill: %w", err)
	}
	if err := os.WriteFile(target, SkillMD, 0o600); err != nil {
		return "", fmt.Errorf("write skill: %w", err)
	}
	if err := writeHash(dir); err != nil {
		return "", err
	}
	return outcome, nil
}

func writeHash(dir string) error {
	if err := os.WriteFile(filepath.Join(dir, HashFile), []byte(Hash(SkillMD)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write skill hash: %w", err)
	}
	return nil
}

// InstalledMatches reports whether dir/SKILL.md is exactly this binary's skill.
func InstalledMatches(dir string) bool {
	current, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	return err == nil && bytes.Equal(current, SkillMD)
}

// untouched reports whether current is what delegate last wrote to dir.
func untouched(dir string, current []byte) bool {
	recorded, err := os.ReadFile(filepath.Join(dir, HashFile))
	return err == nil && strings.TrimSpace(string(recorded)) == Hash(current)
}
