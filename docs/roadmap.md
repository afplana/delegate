# delegate — roadmap

What the tool does, what is built, and what is left. Each remaining item is a
self-contained card, so you can pick one up at any point and code it by hand.

Design and contracts: [design.md](design.md). Planner-side skill:
[skills/delegate/SKILL.md](../skills/delegate/SKILL.md).

## How to jump in

1. Pick a card below (⬜ = not started).
2. Tell Claude the card number and a pairing mode:
   - **mode 1**: you drive, Claude reviews and hints
   - **mode 2**: Claude writes one failing test, you make it pass
   - **mode 3**: Claude writes it, you explain every line back
3. Each card lists the files, the test seam, and the Go idioms you will practise.

Every card ends the same way: `make check` green, then a commit.

## Features

| Feature | Status |
|---|---|
| Run any agent CLI from a profile, with `{prompt}` substitution | ✅ v1 |
| Non-zero exit is a result, not an error; missing binary is an error | ✅ v1 |
| Per-profile timeout; `--timeout` override | ✅ v1 |
| stdout and stderr captured separately and saved to a log dir | ✅ v1 |
| Worker summary from `<delegate-summary>`, fallback to the last 20 lines | ✅ v1 |
| Built-in profiles: `cursor` (default), `claude-sonnet` | ✅ v1 |
| Isolated git worktree per run on branch `delegate/<run-id>` | ✅ v1 |
| `--in-place` mode, refused on `main`/`master` | ✅ v1 |
| Files changed and diffstat, including new and committed files | ✅ v1 |
| One-line JSON result; `status` ok, failed, timeout or error | ✅ v1 |
| Claude Code skill: plan → delegate → review → report, one round | ✅ v1 (in repo, not installed) |
| `delegate profiles` | ✅ |
| `delegate doctor`, `skill install` | ⬜ cards 9b–9c |
| YAML config merged over the built-ins | ⬜ card 11 |
| Large prompts via stdin | ⬜ card 12 |
| Clean up old worktrees and runs | ⬜ card 13 |
| Codex and Gemini profiles, verified | ⬜ card 14 |
| Dirty-repo warning in the result | ⬜ card 15 |
| Public release | ⬜ card 16 |

## v1 slices (done)

| # | Slice | Code | Written by |
|---|---|---|---|
| 1 | Run a command with `{prompt}`, capture stdout | `internal/runner/runner.go` | Alain (mode 2) |
| 2 | Exit code as a result; missing binary as an error | `internal/runner/runner.go` | Alain (mode 2) |
| 3 | Timeout via `context` | `internal/runner/runner.go` | Alain (mode 2) |
| 4 | stderr captured separately | `internal/runner/runner.go` | Alain (mode 2) |
| 5 | Summary extraction plus fallback | `internal/runner/summary.go` | Alain (block), Claude (fallback) |
| 6 | Built-in profiles and `Lookup` | `internal/config/config.go` | Claude (mode 3) |
| 7 | Worktree, files changed, diffstat | `internal/workspace/workspace.go` | Claude (mode 3) |
| 8 | `delegate run` end to end, JSON, run ID, log dir | `cmd/delegate/run.go` | Claude (mode 3) |
| 10 | Skill | `skills/delegate/SKILL.md` | Claude (mode 3) |

### Explain-back checklist for Claude-written code (mode 3)

Answer each in one or two sentences, out loud or in chat:

1. `config.Builtin()` returns a new map on every call. What would go wrong with a package-level map var?
2. `config.Lookup` sorts the known names before printing them. Why does an error message need a stable order?
3. `workspace.Changes` runs `git add --intent-to-add --all` first. What would `files_changed` miss without it?
4. Why does `Changes` diff against `Base` and not `HEAD`? (Hint: the worker may commit.)
5. `Prepare` uses `rev-parse --show-toplevel`. What breaks if you pass `--repo ./internal`?
6. `execute` returns status `error` as JSON instead of returning a Go `error`. Who benefits?
7. `runTask` returns 2 for flag mistakes and 1 for run failures. Why keep them different?
8. Why is the worktree **not** deleted after a run?
9. `main` uses `signal.NotifyContext`. Follow Ctrl-C: how does it reach the worker process?
10. `gittest.InitRepo` sets `GIT_CONFIG_GLOBAL=/dev/null`. What could your own git config break in a test?

## Next: first real use (about 15 minutes)

1. `make install`, then check `delegate version` works from any directory.
2. Install the skill as a **copy** (never a symlink into this repo): `make install-skill`.
3. In a real repo on a feature branch with a clean tree, ask Claude Code: `delegate <small task>`.
4. Note what felt wrong; each issue becomes a card below.

## Backlog cards

### 9a ✅ `delegate profiles` (about 20 minutes, good first card)

List the profiles and whether each binary is on `PATH`.

- Files: `cmd/delegate/profiles.go`, `cmd/delegate/profiles_test.go`
- Seam: `runCommands(ctx, []string{"profiles"}, &out, &errOut, profiles)`
- Test idea: a fake profile with `Command: "sh"` shows `found`; one with `Command: "no-such-bin"` shows `missing`.
- Idioms: `exec.LookPath`, `text/tabwriter`, `slices.Sorted(maps.Keys(m))`.

### 9b ⬜ `delegate doctor` (about 30 minutes)

Check that `git` is on `PATH`, the state dir is writable, and the default
profile's binary exists. Print one line per check; exit 1 if any check fails.

- Files: `cmd/delegate/doctor.go`
- Idioms: a slice of `struct{ name string; check func() error }`, `errors.Join`.

### 9c ⬜ `delegate skill install` (about 30 minutes)

Embed `skills/delegate/SKILL.md` into the binary and copy it to
`~/.claude/skills/delegate/SKILL.md`. Copy, never symlink. Refuse to overwrite a
changed file unless `--force` is passed.

- Files: `skills/embed.go` (`//go:embed delegate/SKILL.md`), `cmd/delegate/skill.go`
- Seam: pass the target dir in, so tests use `t.TempDir()`.
- Idioms: `embed`, `os.MkdirAll`, `bytes.Equal` to detect changes.
- Then replace the `install-skill` Makefile target with this command.

### 11 ⬜ YAML config (about 1 hour)

Load `~/.config/delegate/config.yaml` and `<repo>/.delegate.yaml` and merge them
over `config.Builtin()`. Format: see design.md § Config.

- Files: `internal/config/load.go`, `internal/config/load_test.go`
- Seam: `config.Load(paths ...string) (Config, error)` with temp files.
- Cases: no file means the built-ins; a file adds a profile; a file overrides
  one field of a built-in; `default_profile`; a bad duration is an error naming
  the file and key.
- Idioms: `gopkg.in/yaml.v3` struct tags, `errors.Is(err, fs.ErrNotExist)`,
  `time.ParseDuration`.

### 12 ⬜ Large prompts via stdin (about 30 minutes)

macOS caps argv at about 1 MB. Add `PromptVia string` ("arg" or "stdin") to
`runner.Profile`; with "stdin", set `cmd.Stdin = strings.NewReader(task.Prompt)`.

- Files: `internal/runner/runner.go`, `runner_test.go`
- Test: profile `cat` with `PromptVia: "stdin"` returns the prompt as `Output`.

### 13 ⬜ `delegate clean` (about 45 minutes)

Remove worktrees, `delegate/*` branches and run logs older than N days. This is
destructive, so it shows what it would remove and needs `--yes` to act.

- Idioms: `os.ReadDir`, `fs.FileInfo.ModTime`, `git worktree remove`, `git branch -D`.

### 14 ⬜ Codex and Gemini profiles (about 20 minutes plus real runs)

Read `codex --help` and `gemini --help`, add the profiles, and smoke-test each
with a one-line task. Record the verified flags in design.md.

### 15 ⬜ Dirty-repo warning (about 20 minutes)

A new worktree starts from HEAD, so uncommitted work is invisible to the
worker. Add `"dirty": true` to the result when `git status --porcelain` is
non-empty in the source repo.

- Files: `internal/workspace/workspace.go` (`Workspace.Dirty`), `cmd/delegate/run.go`

### 16 ⬜ Public release (about 1 hour)

- `AGENTS.md`, plus a `CLAUDE.md` containing `@AGENTS.md`.
- README: install, usage, JSON example, skill install.
- `gitleaks git` over the full history; grep for company names, internal hosts and ticket keys.
- Tag `v0.1.0`; optionally add goreleaser for binaries.
- Flip the repo to public.

## Known limitations (v1)

- The worker runs with the permissions its CLI grants in print mode. Cursor
  runs without `--force` (org-managed accounts can disable it): file edits
  work, shell commands may not (a smoke test's `ls` returned no output), so
  the worker may be unable to run tests. Claude's `acceptEdits` allows edits,
  but shell commands outside your Claude allow-list are denied. Either way the
  planner runs the tests during review.
- No progress output while the worker runs; `delegate` is silent until it ends.
- macOS and Linux only: on timeout or Ctrl-C the worker's whole process group
  is killed (`syscall.Kill`), which does not compile on Windows.
- `--in-place` marks new files intent-to-add in your repo's index (that is how
  they show up in the diff). `git reset <file>` undoes it.
- Worktrees and logs accumulate under `~/.local/state/delegate/` until card 13.
