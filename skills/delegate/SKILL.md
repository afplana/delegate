---
name: delegate
description: >-
  Plan a coding task yourself, hand the plan to a cheaper worker agent CLI
  (Cursor, Claude Sonnet, ...) through the `delegate` CLI, then review the
  worker's diff, run the tests and report back — exactly one round, no
  auto-fix loop. Use when the user says "delegate", "delegate TICKET-123",
  "have cursor/sonnet implement this", "hand this plan to a worker", or asks
  to implement something cheaply with a worker agent while you plan and review.
---

# delegate — plan, hand off, review, report

You are the **planner and reviewer**. A worker agent does the typing.
`delegate` runs the worker in an isolated git worktree and prints **one line of
JSON**. You never read the worker's full transcript unless something went wrong.

One round only: plan → delegate → review → report. **Never** start a second
worker round or fix the worker's code yourself unless the user asks for it.
The user decides what happens with the result.

## 0. Preflight (30 seconds)

1. `delegate version` — if not found: tell the user to run `make install` in the
   delegate repo (or `go install github.com/afplana/delegate/cmd/delegate@latest`)
   and stop.
2. `git -C <repo> status --porcelain` — the worktree starts from **HEAD**, so
   uncommitted changes are **not** visible to the worker. If the repo is dirty,
   tell the user and ask: commit first, or run with `--in-place` (only on a
   feature branch; delegate refuses `main`/`master`).

## 1. Plan

Gather context the way you normally would: the ticket (via whatever tracker
connector is available), the code paths involved, the repo's conventions
(`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, lint config).

The worker has **none of your context**. Write a self-contained plan file to
your scratchpad (or `$TMPDIR`), named `delegate-<ticket-or-slug>.md`:

```markdown
# Goal
One or two sentences: the behaviour that must exist when done.

# Context
- Repo conventions the worker must follow (test framework, lint, naming).
- Relevant files with paths, and the 5–20 lines that matter from each.

# Steps
1. Concrete, ordered edits. Name files and functions.
2. ...

# Acceptance criteria
- Exact commands that must pass, e.g. `go test -race ./...`, `make lint`.
- Observable behaviour, e.g. "`Foo("")` returns ErrEmpty".

# Constraints
- Do not commit. Do not push. Do not add dependencies unless listed here.
- Do not touch: <files or areas out of scope>.
```

Keep it as short as it can be while staying unambiguous. Ambiguity is the main
reason workers drift. Do not add the summary-block instruction: `delegate`
appends it.

## 2. Delegate

Pick the profile: `cursor` (default) or `claude-sonnet`. Use what the user
named; otherwise the default.

Run it with the Bash tool **in the background** (`run_in_background: true`):
a worker can take up to 30 minutes and a foreground Bash call times out at 10.

```bash
delegate run --task <plan-file> --repo <repo> --profile <profile>
```

Optional flags: `--timeout 45m` (default 30m), `--in-place`.

Wait for the completion notification, then read the single JSON line:

| Field | Use it for |
|---|---|
| `status` | `ok` / `failed` (non-zero exit) / `timeout` / `error` (delegate itself failed — see `error`) |
| `workdir`, `branch`, `base` | where the work is, and the commit it started from |
| `files_changed`, `diffstat` | scope check before reading any code |
| `summary` | the worker's own report (or the last 20 lines if it skipped the block) |
| `log_dir` | `prompt.md`, `stdout.log`, `stderr.log`, `result.json` — read only when needed |

- `error`: report the `error` text and stop. Nothing ran or nothing was diffed.
- `timeout` or `failed`: still review — partial work may be useful. Read the
  tail of `stderr.log` before guessing why.

## 3. Review

1. Diff against the start commit (delegate makes new files visible to diff):
   `git -C <workdir> diff <base>`
2. Check it against the plan, not against taste:
   - Every acceptance criterion met?
   - Files outside the plan touched? Tests deleted, skipped or weakened?
   - Hardcoded values, swallowed errors, secrets, debug leftovers?
   - Repo conventions followed (imports, test style, naming)?
3. Run the acceptance commands **in the workdir** and keep the real output.
   Never claim tests pass without having just run them.

## 4. Report

Lead with the verdict, then evidence. Keep it short:

```markdown
**<ticket>: <ready to accept | needs fixes | failed>** — profile <p>, <duration>, status <status>

**Changed:** <diffstat> — <files>
**Tests:** `<command>` → <pass/fail, with the failing line if any>

**Findings** (most severe first, file:line):
1. blocker — ...
2. should-fix — ...

**Worker said:** <one or two lines from summary>

**Options:**
a) Accept — I bring the changes into your branch.
b) You fix the findings yourself in <workdir>.
c) Another delegate round with a follow-up plan for the findings.
```

Then stop and wait. Do not pick an option for the user.

## 5. After the user decides

- **Accept**: commit in the worktree, then bring it over. Show the commands first:
  ```bash
  git -C <workdir> add -A
  git -C <workdir> commit -m "<message following the repo's convention>"
  git -C <repo> cherry-pick <base>..<branch>
  ```
- **Cleanup** (deletes the worktree and its branch — confirm first):
  ```bash
  git -C <repo> worktree remove <workdir>
  git -C <repo> branch -D <branch>
  ```
- **Another round**: write a new plan containing only the findings to fix, and
  run `delegate run` again with `--repo <workdir> --in-place`, so it builds on
  the same branch.
