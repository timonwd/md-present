# Development

Run all commands from the repository root.

## Build from source

Go 1.25, Node.js 24, and pnpm 11 are required.

Build the current checkout for local development:

```sh
pnpm install --frozen-lockfile --ignore-scripts
pnpm run assets
go build -o md-present ./cmd/md-present
```

The generated Mermaid JavaScript and license files are intentionally ignored by
Git. Release binaries embed them, so Homebrew users do not need Node.js or pnpm.

## Standard validation

```sh
pnpm install --frozen-lockfile --ignore-scripts
pnpm run assets
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
