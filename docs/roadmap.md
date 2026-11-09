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
| `delegate doctor` | ✅ |
| `delegate skill install` | ⬜ card 9c |
| YAML config merged over the built-ins | ⬜ card 11 |
| Large prompts via stdin | ⬜ card 12 |
| Clean up old worktrees and runs | ⬜ card 13 |
| Codex and Gemini profiles, verified | ⬜ card 14 |
| Dirty-repo warning in the result | ⬜ card 15 |
| Public release | ⬜ card 16 |
| Recursion guard: a worker cannot start its own delegate run | ⬜ card 17 (Alain codes) |
| `--verify`: delegate runs the acceptance commands itself | ⬜ card 18 |
| File allowlist in the plan; `out_of_scope` in the result | ⬜ card 19 |
| Plan check: refuse a plan without Goal, Steps, Acceptance criteria | ⬜ card 20 |
| Cost, tokens and review verdict recorded per run | ⬜ card 21 |
| `claude-haiku` profile | ⬜ card 22 |
| Skill: plans give paths and line ranges, not pasted code | ⬜ card 23 |
| `repo` and `started_at` in `result.json` | ⬜ card 24 |

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

### 9b ✅ `delegate doctor` (about 30 minutes)

Check that `git` is on `PATH`, the state dir is writable, and the default
profile's binary exists. Print one line per check; exit 1 if any check fails.

- Files: `cmd/delegate/doctor.go`, `cmd/delegate/doctor_test.go`
- Idioms: a slice of `struct{ name string; check func() error }`, `errors.Join`.

### 9c ⬜ `delegate skill install` (about 30 minutes)

Embed `skills/delegate/SKILL.md` into the binary and copy it to
`~/.claude/skills/delegate/SKILL.md`. Copy, never symlink. Refuse to overwrite a
changed file unless `--force` is passed.

- Files: `skills/embed.go` (`//go:embed delegate/SKILL.md`), `cmd/delegate/skill.go`
- Seam: pass the target dir in, so tests use `t.TempDir()`.
- Idioms: `embed`, `os.MkdirAll`, `bytes.Equal` to detect changes.
- Then replace the `install-skill` Makefile target with this command.
- Also add a `doctor` check: `installed skill matches binary` (warn, not fail,
  when they differ). On 2026-10-23 the installed copy had silently fallen
  behind the repo (it still used `git add -A` and `branch -D`); this check
  would have caught it.

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
- State on 2026-10-23: 3 leftover worktrees, 21 MB.

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

### 17 ⬜ Recursion guard (about 30 minutes) — Alain codes, Claude helps (mode 2)

A worker is a full agent session (`claude -p` loads your user skills, including
`delegate`), so a plan saying "delegate X" could make the worker start its own
worker. Not seen so far (checked 2026-10-14), but cheap to prevent.

Guard inside delegate itself, so it works for every CLI and stays role-agnostic:

1. `runner.Run` passes `DELEGATE_RUN_ID=<run id>` to the worker
   (`cmd.Env = append(os.Environ(), ...)`; add a `Task.Env` or `RunID` field).
2. `delegate run` refuses to start when `DELEGATE_RUN_ID` is already set:
   `status: error`, `error: "nested delegate run inside run <id>"`.

- Files: `internal/runner/runner.go`, `cmd/delegate/run.go`, their tests.
- Test seams: a profile `sh -c 'echo $DELEGATE_RUN_ID'` returns the id in `Output`;
  `t.Setenv("DELEGATE_RUN_ID", "outer")` then `runCommands(... "run" ...)` returns status `error`.
- Idioms: `os.Environ`, `cmd.Env`, `os.LookupEnv`, `t.Setenv`.
- Optional second layer: block the skill in the `claude-sonnet` profile args.
  Check `claude --help` for the exact disallowed-tools syntax first (UNVERIFIED).

## Accuracy and efficiency cards (from the 2026-10-23 run review)

Evidence: the 10 runs in `~/.local/state/delegate/runs` up to 2026-10-23. Nine
ended `ok`, one `failed` (Cursor `--force` disabled by an admin, fixed in
`2f2542b`). Every successful run wrote the `<delegate-summary>` block. Weak
spots: in 2 runs the worker's output never mentions a test command, one run
touched 18 files, and nothing records cost or how the review went. Highest
payoff first.

### 18 ⬜ `--verify "<cmd>"` (about 45 minutes)

Run the acceptance commands **after** the worker exits, in the workdir, and put
the outcome in the result: `"verify": {"cmd": "...", "exit": 0, "tail": "..."}`
(last 30 lines of combined output). The result no longer depends on the worker
saying it ran the tests, and a red run costs the planner one JSON line instead
of a full diff review. Repeatable flag: `--verify "go test ./..." --verify "make lint"`.
`status` stays the worker's status; verify is a separate field.

- Files: `cmd/delegate/run.go`, `internal/runner/verify.go`, tests.
- Seam: `runVerify(ctx, dir string, cmds []string) []VerifyResult`; test with
  `sh -c 'exit 3'` and `sh -c 'echo ok'` in `t.TempDir()`.
- Idioms: `exec.CommandContext` with `cmd.Dir`, `CombinedOutput`,
  `errors.As(err, &exitErr)` for the exit code, a flag type implementing
  `flag.Value` for the repeatable flag.
- Skill follow-up: pass the plan's acceptance commands as `--verify`; the
  planner still re-runs them during review (evidence policy), but stops early
  on a red verify.

### 19 ⬜ File allowlist and `out_of_scope` (about 45 minutes)

The plan gets a `# Files` section: one path or glob per line. After the run,
every entry of `files_changed` that matches none of them goes into
`"out_of_scope": [...]`. Catches the most common drift (the 18-file run) without
reading code. No `# Files` section means no check, so old plans still work.

- Files: `internal/plan/plan.go` (parse sections), `cmd/delegate/run.go`, tests.
- Seam: `plan.Parse(md string) (Plan, error)` and `OutOfScope(changed, allowed []string) []string`.
- Cases: exact path, `internal/**` glob, a file outside both, empty section.
- Idioms: `bufio.Scanner`, `strings.HasPrefix` for headings,
  `path.Match` (note: no `**`; handle a trailing `/**` as a prefix match yourself).

### 20 ⬜ Plan check before the worker starts (about 30 minutes)

`delegate run` refuses a plan missing a `# Goal`, `# Steps` or
`# Acceptance criteria` heading: `status: error`,
`error: "plan missing sections: Steps, Acceptance criteria"`. The skill names
ambiguity as the main cause of drift; this makes it impossible to hand over a
plan without acceptance criteria. Reuses card 19's parser, so do 19 first.

- Files: `internal/plan/plan.go`, `cmd/delegate/run.go`, tests.
- Seam: `Plan.Missing() []string`.
- Idioms: table-driven tests, `slices.Contains`.

### 21 ⬜ Cost, tokens and review verdict per run (about 1 hour)

Record what each run cost and whether its result was accepted, so the choice
between Cursor and Sonnet rests on data (today: 4 Cursor runs, 6 Sonnet runs,
no quality data).

- Switch both profiles to `--output-format json` (verified 2026-10-23):
  - `claude -p --output-format json` → `total_cost_usd`, `duration_ms`,
    `num_turns`, `usage.{input_tokens,output_tokens,cache_read_input_tokens,...}`;
    the reply text is in `result`.
  - `cursor-agent -p --trust --output-format json` → `usage.{inputTokens,outputTokens,cacheReadTokens,cacheWriteTokens}`,
    `duration_ms`, reply text in `result`. **No cost field.**
- Add `"usage": {"input_tokens", "output_tokens", "cost_usd"}` to `result.json`
  (`cost_usd` omitted for Cursor). Summary extraction now reads the JSON
  `result` field instead of raw stdout.
- New subcommand `delegate review <run-id> accepted|needs-fixes|failed [note]`
  writes `runs/<id>/review.json`. The skill calls it after the user decides.
- Later: `delegate stats` aggregates per profile (runs, accept rate, median
  tokens, cost).
- Files: `internal/config/config.go`, `internal/runner/summary.go`,
  `cmd/delegate/review.go`, tests.
- Idioms: `json.Unmarshal` into a small struct per profile shape,
  `omitempty`, a `Parse func([]byte) (Usage, string, error)` field on `Profile`.

### 22 ⬜ `claude-haiku` profile (about 10 minutes)

One entry next to `claude-sonnet` in `internal/config/config.go:23`, using
`claudeArgs("haiku")`. For Tier C work in `~/.claude/CLAUDE.md` model routing:
renames, mechanical sweeps, fully specified edits.

- Test: `Lookup("claude-haiku")` returns `--model haiku`.

### 23 ⬜ Skill: paths and line ranges, not pasted code (about 15 minutes)

`skills/delegate/SKILL.md` § Plan asks the planner to paste "the 5–20 lines
that matter" from each file. The worker can read files itself, so the planner
spends Opus tokens copying code the worker re-reads anyway. Change the template
to `path:start-end — why it matters`. Keep a short paste only for code the
worker must **not** read past (e.g. one function in a huge file).

- Files: `skills/delegate/SKILL.md`. Then reinstall the skill (card 9c).

### 24 ⬜ `repo` and `started_at` in `result.json` (about 15 minutes)

Today a past run can't be traced back to its repo or start time
(`result.json` has `duration_ms` but no timestamp or source repo). Add
`"repo": "<abs path>"` and `"started_at": "<RFC 3339 UTC>"`.

- Files: `cmd/delegate/run.go`, `run_test.go`.
- Idioms: `time.Now().UTC().Format(time.RFC3339)`, `filepath.Abs`.

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
