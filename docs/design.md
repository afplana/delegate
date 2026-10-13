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

## CLI surface (target v1)

```
delegate run --task plan.md [--profile cursor] [--repo .] [--in-place] [--timeout 30m]
delegate profiles            # list configured profiles and whether their binary is on PATH
delegate doctor              # check config + backends
delegate skill install       # copy the Claude Code skill into ~/.claude/skills (copy, never symlink)
delegate version
```

## Config

`~/.config/delegate/config.yaml`, optionally overridden by `<repo>/.delegate.yaml`.

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
  "status": "ok | failed | timeout",
  "exit_code": 0,
  "duration_ms": 412000,
  "worktree": "/path/to/worktree",
  "branch": "delegate/20261013-142900-ab12",
  "files_changed": ["internal/x.go"],
  "diffstat": "3 files changed, 40 insertions(+), 2 deletions(-)",
  "summary": "worker's own summary, extracted from its output",
  "log_dir": "~/.local/state/delegate/runs/20261013-142900-ab12"
}
```

The full stdout/stderr lives in `log_dir`. The caller reads it only when needed.

## Error model

- `error` from `Run` = delegate could not do its job (binary missing, bad config, git failure).
- A worker that runs and exits non-zero is **not** an error: it is a `Result` with `status: failed`.

## Isolation

By default each run gets its own git worktree on branch `delegate/<run-id>`.
`--in-place` runs in the given directory instead. Never runs on `main`/`master` in place.

## Packages

| Package | Responsibility | Test seam |
|---|---|---|
| `internal/runner` | Build the command from a profile, execute it, capture output and exit code | `runner.Run` against real processes (`echo`, `sh`) — no mocks |
| `internal/config` | Load, merge and validate YAML config | `config.Load` with temp files |
| `internal/workspace` | Create worktree, compute files changed and diffstat | Real temp git repos |
| `cmd/delegate` | Flag parsing, wiring, JSON output | `run(args, stdout, stderr) int` — no binary build needed |

## Slices (TDD order)

1. ✅ scaffold — `runner.Run` passes `{prompt}` to a command and captures stdout
2. non-zero exit → `Result.ExitCode`, not an error; missing binary → error
3. timeout via `context` → `status: timeout`
4. stderr + log dir written
5. summary extraction (`<delegate-summary>` block, fallback: output tail)
6. `config.Load` + profile lookup
7. `workspace`: worktree + diffstat
8. `cmd/delegate run` end-to-end with JSON output
9. `profiles`, `doctor`, `skill install`
10. Claude Code skill (`skills/delegate/SKILL.md`)
