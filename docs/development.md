# Development

Run all commands from the repository root.

## Build from source

Go 1.22 or newer is required.

Install the latest source release directly:

```sh
go install github.com/timonwd/md-present/cmd/md-present@latest
```

Build the current checkout for local development:

```sh
go build -o md-present ./cmd/md-present
```

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
