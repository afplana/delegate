# delegate — design

## Goal

Let a strong "planner" model hand a written plan to a cheaper "worker" agent CLI,
and get back a **compact, structured result** instead of the worker's full transcript.

`delegate` is role-agnostic: it does not know or care who planned the task.
It runs one task with one configured **profile** and reports what happened.
Plan with Opus and code with Sonnet, plan with Opus and code with Cursor — same tool.

## Non-goals

- Planning, reviewing or judging the work. The caller does that.
- Retrying or looping. One task in, one result out. Fix-up rounds are the caller's decision.
- Storing credentials. Each backend CLI uses its own login.

## Concepts

| Term | Meaning |
|---|---|
| **Profile** | How to invoke one agent CLI: command, args, how the prompt is passed, timeout. Pure config. |
| **Task** | The prompt (usually a plan file) plus the directory to work in. |
| **Run** | One execution of a task with a profile. Has an ID, a log dir, and a result. |
| **Result** | The compact JSON contract returned to the caller. |

## CLI surface

```
delegate run --task plan.md [--profile cursor] [--repo .] [--in-place] [--timeout 30m]   # v1
delegate version                                                                          # v1
delegate profiles            # later: list profiles and whether their binary is on PATH
delegate doctor              # later: check config + backends
delegate skill install       # later: copy the Claude Code skill into ~/.claude/skills (copy, never symlink)
```

Exit codes: 0 = status ok, 1 = any other status (JSON still printed), 2 = bad flags (usage on stderr).

## Config

v1 ships the `cursor` and `claude-sonnet` profiles built in (`internal/config`).
Later (roadmap card 11): `~/.config/delegate/config.yaml`, optionally overridden by `<repo>/.delegate.yaml`, merged over the built-ins.

```yaml
default_profile: cursor
profiles:
  cursor:
    command: cursor-agent
    args: ["-p", "--force", "--trust", "--output-format", "text", "{prompt}"]
    timeout: 30m
  claude-sonnet:
    command: claude
    args: ["-p", "--model", "sonnet", "--permission-mode", "acceptEdits", "{prompt}"]
    timeout: 30m
  codex:            # untested in v1
    command: codex
    args: ["exec", "--full-auto", "{prompt}"]
  gemini:           # untested in v1
    command: gemini
    args: ["-p", "{prompt}", "--yolo"]
```

Placeholders: `{prompt}` (v1). Later: `{prompt_file}`, `{model}`, and `prompt_via: stdin`
for prompts too large for argv.

## Result contract

```json
{
  "run_id": "20261013-142900-ab12",
  "profile": "cursor",
  "status": "ok | failed | timeout | error",
  "exit_code": 0,
  "duration_ms": 412000,
  "workdir": "~/.local/state/delegate/worktrees/20261013-142900-ab12",
  "branch": "delegate/20261013-142900-ab12",
  "base": "<commit the worker started from>",
  "files_changed": ["internal/x.go"],
  "diffstat": "3 files changed, 40 insertions(+), 2 deletions(-)",
  "summary": "worker's own summary, extracted from its output",
  "log_dir": "~/.local/state/delegate/runs/20261013-142900-ab12",
  "error": "only when status is error"
}
```

Printed as one line. `log_dir` holds `prompt.md`, `stdout.log`, `stderr.log` and
`result.json`. The caller reads them only when needed. `files_changed` and
`diffstat` compare the workdir with `base`, so they include commits the worker made
and new untracked files.

## Error model

- `error` from `Run` = delegate could not do its job (binary missing, bad config, git failure).
  The CLI reports it as `status: error` with an `error` message, so the caller always gets JSON.
- A worker that runs and exits non-zero is **not** an error: it is a `Result` with `status: failed`.

## Isolation

By default each run gets its own git worktree on branch `delegate/<run-id>`.
`--in-place` runs in the given directory instead. Never runs on `main`/`master` in place.

## Packages

| Package | Responsibility | Test seam |
|---|---|---|
| `internal/runner` | Build the command from a profile, execute it, capture output and exit code | `runner.Run` against real processes (`echo`, `sh`) — no mocks |
| `internal/config` | Built-in profiles and lookup (YAML later) | `config.Lookup(config.Builtin(), name)` |
| `internal/workspace` | Create worktree, compute files changed and diffstat | Real temp git repos (`internal/gittest`) |
| `cmd/delegate` | Flag parsing, wiring, JSON output | `runCommands(ctx, args, stdout, stderr, profiles) int` with a fake `sh` worker — no binary build needed |

## Slices (TDD order)

1. ✅ scaffold — `runner.Run` passes `{prompt}` to a command and captures stdout
2. ✅ non-zero exit → `Result.ExitCode`, not an error; missing binary → error
3. ✅ timeout via `context` → `Result.TimedOut` (mapped to `status: timeout` in slice 8)
4. ✅ stderr captured (log dir moved to slice 8, where the run id exists)
5. ✅ summary extraction (`<delegate-summary>` block, fallback: last 20 lines)
6. ✅ built-in profiles + lookup (YAML moved to after v1)
7. ✅ `workspace`: worktree + diffstat
8. ✅ `cmd/delegate run` end-to-end with JSON output, run id, log dir
9. `profiles`, `doctor`, `skill install` (after v1)
10. ✅ Claude Code skill (`skills/delegate/SKILL.md`)

Remaining work, as pick-up cards: [roadmap.md](roadmap.md).
