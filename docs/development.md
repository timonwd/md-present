# Development

Run all commands from the repository root.

## Standard validation

```sh
gofmt -w ./cmd/md-present/*.go
go vet ./...
go test ./...
go build ./...
```

For server or UI changes, also build the executable and smoke-test `--no-open`
against `example.md`: wait for the URL, request the page and embedded assets,
verify representative slide content, then stop the process cleanly.

## Agent plugin

Codex and Claude Code share the plugin package under `plugins/md-present/`.
Maintain the skill only at `plugins/md-present/skills/md-present/SKILL.md`; do
not introduce distribution-specific copies.

For skill or plugin changes, validate the skill and the shared plugin package
before handing off.
