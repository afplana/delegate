# AGENTS.md

Guidance for coding agents (and people) working on `delegate`.

## What this is

A Go CLI that hands a written plan to a worker agent CLI, runs it in its own
git worktree and prints one line of JSON. It is role-agnostic: it never knows
which model planned, and a worker is only a command plus arguments (a profile).
Design: [docs/design.md](docs/design.md). Backlog: [docs/roadmap.md](docs/roadmap.md).

## Layout

| Path | What |
|---|---|
| `cmd/delegate/` | subcommands: `run`, `review`, `stats`, `clean`, `profiles`, `doctor`, `skill`, `version` |
| `internal/runner/` | runs one profile: prompt via arg or stdin, timeout, output parsers, `--verify` |
| `internal/workspace/` | worktree per run, diff, cleanup of old runs |
| `internal/config/` | built-in profiles, user config file, restricted repo file |
| `internal/plan/` | plan sections check, `# Files` allowlist |
| `internal/history/` | review verdicts and `stats` |
| `skills/` | the embedded Claude Code skill (`delegate skill install`) |

## Commands

```bash
make check   # tests (-race), vet, golangci-lint, gitleaks: what CI runs
make test    # tests only
make fmt     # gofmt + import order
make setup   # once: pre-commit hooks
```

Run `make check` before every push. The pre-push hook runs it too.

## Conventions

- Go 1.26. Use the current idioms: `errors.AsType`, `t.Context()`, `maps`/`slices`.
- Tests first, with testify (`require` for preconditions, `assert` for checks).
  Prefer behaviour tests through `runCommands` and real git repos
  (`internal/gittest`) over mocks.
- Imports in three groups: standard library, this module, third party
  (golangci-lint's `gci` enforces it; `make fmt` fixes it).
- Errors name what failed and what to do next (file and key for config errors).
- Every change updates the docs it touches: `README.md`, `docs/design.md`,
  `docs/roadmap.md` (mark the card done, with the date) and
  `skills/delegate/SKILL.md` if the planner's behaviour changes.

## Rules that must not regress

- **Never force.** `clean` uses `git worktree remove` without `--force` and
  `git branch -d`, never `-D`: an unmerged `delegate/*` branch may hold work
  nobody accepted yet.
- **A repo can't choose what runs.** `.delegate.yaml` may set only
  `default_profile` and timeouts; `command`, `args`, `output` and `prompt_via`
  come from built-ins or the user's own config.
- **No profile switching.** If a profile's CLI is missing, fail and name the
  installed profiles; never pick another one silently.
- **Verified flags only.** A built-in profile's flags must have been run for
  real against that CLI; record the check in `docs/design.md`.
- **No nested runs.** Workers get `DELEGATE_RUN_ID`; `delegate run` refuses to
  start when it is set. If you are a worker reading this, follow your plan and
  the worker contract only.
