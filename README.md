# delegate

Let your strongest model **plan and review**, and a cheaper agent CLI **do the typing**.

`delegate` hands a written plan to a worker agent CLI (Cursor, Claude Code with
Sonnet, ...), runs it in an **isolated git worktree**, and returns **one line of
JSON** instead of the worker's full transcript. The planner reads a few hundred
tokens, not tens of thousands.

It ships with a [Claude Code skill](skills/delegate/SKILL.md) that drives the whole
loop: Claude plans, delegates, reviews the diff, runs the tests and reports back.
You decide what happens next.

```
you ──▶ planner (e.g. Opus) ──plan.md──▶ delegate ──▶ worker CLI (cursor-agent / claude -p)
                ▲                            │               │ edits files in its own worktree
                └──── one line of JSON ◀──────┴───────────────┘
```

## Why

- **Cost:** the expensive model never reads the worker's transcript. It gets status,
  files changed, a diffstat and the worker's own short summary.
- **Safety:** every run gets its own worktree on branch `delegate/<run-id>`. Your
  checkout is untouched until you accept the change.
- **No lock-in:** a worker is just a CLI plus arguments (a *profile*). delegate does
  not know or care which model planned.

## Requirements

- Go 1.26+ (to install), `git`
- At least one worker CLI, logged in: [`cursor-agent`](https://cursor.com/cli) (default) or
  [`claude`](https://docs.anthropic.com/en/docs/claude-code)
- macOS or Linux

## Install

```bash
go install github.com/afplana/delegate/cmd/delegate@latest
delegate doctor
```

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`). If
`delegate` is not found in a new terminal, add that folder to your `PATH`:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
```

`delegate doctor` should print three `ok` lines:

```
ok    git: /usr/bin/git
ok    state dir: /Users/you/.local/state/delegate
ok    default profile: cursor: /Users/you/.local/bin/cursor-agent
```

## Use it from Claude Code (recommended)

Install the skill (a plain copy, so it never changes under you):

```bash
mkdir -p ~/.claude/skills/delegate
curl -fsSL https://raw.githubusercontent.com/afplana/delegate/main/skills/delegate/SKILL.md \
  -o ~/.claude/skills/delegate/SKILL.md
```

Then, in a repo on a feature branch with a clean working tree, ask Claude Code:

```
delegate add a test for the version command
delegate implement TICKET-123 with sonnet
```

Claude writes a self-contained plan, runs `delegate` in the background, reviews
the diff against the plan, runs your tests in the worktree, and reports:

```
Version test: ready to accept — cursor, 68 s, status ok
Changed: 1 file changed, 18 insertions(+) — cmd/delegate/version_test.go
Tests: go test -race ./... → pass · golangci-lint → 0 issues
Findings: none
Options: a) accept  b) fix it yourself  c) another round
```

It does **one round only**. There is no automatic fix loop; you pick the option.

## Use it directly

```bash
delegate run --task plan.md --repo . [--profile cursor|claude-sonnet] [--timeout 30m] [--in-place]
```

Output (one line, pretty-printed here):

```json
{
  "run_id": "20261015-085020-f97f",
  "profile": "cursor",
  "status": "ok",
  "exit_code": 0,
  "duration_ms": 68134,
  "workdir": "~/.local/state/delegate/worktrees/20261015-085020-f97f",
  "branch": "delegate/20261015-085020-f97f",
  "base": "69d9d67…",
  "files_changed": ["cmd/delegate/version_test.go"],
  "diffstat": "1 file changed, 18 insertions(+)",
  "summary": "Added version_test.go only … acceptance criteria met.",
  "log_dir": "~/.local/state/delegate/runs/20261015-085020-f97f"
}
```

| `status` | Meaning | Exit code |
|---|---|---|
| `ok` | worker exited 0 | 0 |
| `failed` | worker exited non-zero (partial work is still in the worktree) | 1 |
| `timeout` | worker was stopped, including its child processes | 1 |
| `error` | delegate itself could not run (see `error`) | 1 |

Bad flags print usage to stderr and exit 2. Full output lives in `log_dir`:
`prompt.md`, `stdout.log`, `stderr.log`, `result.json`.

The worker is told to end with a `<delegate-summary>` block; that becomes
`summary`. If it does not, `summary` is the last 20 lines of its output.

### Accepting the work

The changes sit uncommitted in `workdir`. To bring them into your branch:

```bash
git -C <workdir> add <files_changed...>
git -C <workdir> commit -m "..."
git merge --ff-only <branch>        # or: git cherry-pick <base>..<branch>
git worktree remove <workdir> && git branch -d <branch>
```

## Commands

| Command | What it does |
|---|---|
| `delegate run` | run one task with one profile (above) |
| `delegate profiles` | list profiles and whether each binary is on `PATH` |
| `delegate doctor` | check git, the state dir and the default profile's binary |
| `delegate version` | print the build version |

## Profiles

| Profile | Command | Notes |
|---|---|---|
| `cursor` (default) | `cursor-agent -p --trust --output-format text {prompt}` | No `--force`, so it also works where an admin disabled "Run Everything". Shell commands may be restricted; the planner runs the tests. |
| `claude-sonnet` | `claude -p --model sonnet --permission-mode acceptEdits {prompt}` | Edits are allowed; shell commands outside your Claude allow-list are denied. |

Both time out after 30 minutes by default. Custom profiles via a config file are
on the [roadmap](docs/roadmap.md).

## Limitations

- New worktrees start from `HEAD`: **commit first**, or uncommitted changes are invisible to the worker.
- `--in-place` works in your checkout instead (refused on `main`/`master`).
- No progress output while the worker runs.
- Worktrees and logs accumulate under `~/.local/state/delegate/` until you remove them.

## Develop

```bash
make setup   # once: install pre-commit hooks (golangci-lint + secret scan)
make watch   # tests on every save
make check   # tests + lint + secret scan, same as CI
make install # build and install with the current commit as version
```

Design and contracts: [docs/design.md](docs/design.md). What is next, as pick-up
cards: [docs/roadmap.md](docs/roadmap.md).

## License

MIT
