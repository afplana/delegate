package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/afplana/delegate/skills"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall(t *testing.T) {
	t.Run("fresh install copies the embedded skill", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "skills", "delegate")

		got, err := skills.Install(dir, false)

		require.NoError(t, err)
		assert.Equal(t, skills.Installed, got)
		assert.Equal(t, skills.SkillMD, readFile(t, filepath.Join(dir, "SKILL.md")))
		info, err := os.Lstat(filepath.Join(dir, "SKILL.md"))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular(), "a copy, never a symlink")
	})

	t.Run("same content is up to date", func(t *testing.T) {
		dir := t.TempDir()
		_, err := skills.Install(dir, false)
		require.NoError(t, err)

		got, err := skills.Install(dir, false)

		require.NoError(t, err)
		assert.Equal(t, skills.UpToDate, got)
	})

	t.Run("an untouched older install is upgraded", func(t *testing.T) {
		dir := t.TempDir()
		installOld(t, dir, "old skill\n")

		got, err := skills.Install(dir, false)

		require.NoError(t, err)
		assert.Equal(t, skills.Upgraded, got)
		assert.Equal(t, skills.SkillMD, readFile(t, filepath.Join(dir, "SKILL.md")))
	})

	t.Run("a locally edited skill is not overwritten", func(t *testing.T) {
		dir := t.TempDir()
		installOld(t, dir, "old skill\n")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("my edits\n"), 0o600))

		_, err := skills.Install(dir, false)

		require.ErrorIs(t, err, skills.ErrLocalChanges)
		assert.Equal(t, []byte("my edits\n"), readFile(t, filepath.Join(dir, "SKILL.md")))
	})

	t.Run("a copy delegate did not install needs --force", func(t *testing.T) {
		// make install-skill and the README's curl command wrote no hash file.
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("curl copy\n"), 0o600))

		_, err := skills.Install(dir, false)
		require.ErrorIs(t, err, skills.ErrLocalChanges)

		got, err := skills.Install(dir, true)
		require.NoError(t, err)
		assert.Equal(t, skills.Upgraded, got)
		assert.Equal(t, skills.SkillMD, readFile(t, filepath.Join(dir, "SKILL.md")))
	})
}

func TestInstall_UpToDateCopyGetsAHashSoLaterUpgradesWork(t *testing.T) {
	// A current copy from make or curl has no hash file; record one now so
	// the next version can upgrade it without --force.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), skills.SkillMD, 0o600))

	got, err := skills.Install(dir, false)

	require.NoError(t, err)
	assert.Equal(t, skills.UpToDate, got)
	assert.Equal(t, skills.Hash(skills.SkillMD)+"\n", string(readFile(t, filepath.Join(dir, skills.HashFile))))
}

func TestInstalledMatches(t *testing.T) {
	dir := t.TempDir()

	assert.False(t, skills.InstalledMatches(dir), "nothing installed")
	_, err := skills.Install(dir, false)
	require.NoError(t, err)
	assert.True(t, skills.InstalledMatches(dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("drifted\n"), 0o600))
	assert.False(t, skills.InstalledMatches(dir))
}

// installOld fakes an earlier `delegate skill install` of content.
func installOld(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, skills.HashFile), []byte(skills.Hash([]byte(content))+"\n"), 0o600))
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
