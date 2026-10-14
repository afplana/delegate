# delegate

Hand a plan written by a strong "planner" model to a cheaper worker agent CLI
(Cursor, Claude Code, Codex, Gemini, ...) and get back a compact, structured result.

Status: v1 works end to end. See [docs/roadmap.md](docs/roadmap.md) for what is
next, and [docs/design.md](docs/design.md) for contracts.

## Use

```sh
make install                      # delegate into $GOPATH/bin
delegate run --task plan.md --repo . --profile claude-sonnet
```

It prints one line of JSON: status, files changed, diffstat, the worker's summary,
and where the worktree and full logs are.

The Claude Code skill in [skills/delegate](skills/delegate/SKILL.md) drives this
from the planner side. Install it as a copy with `make install-skill`.

## Develop

```sh
make setup   # once: install pre-commit hooks (golangci-lint + secret scan)
make watch   # tests on every save
make check   # tests + lint + secret scan, same as CI
```

## License

MIT
