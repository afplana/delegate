# delegate

A strong "planner" model to a cheaper worker agent CLI
(Cursor, Claude Code, Codex, Gemini, ...) and get back a compact, structured result.

Status: early development. See [docs/design.md](docs/design.md).

## Develop

```sh
make setup   # once: install pre-commit hooks (golangci-lint + secret scan)
make watch   # tests on every save
make check   # tests + lint + secret scan, same as CI
```

## License

MIT
