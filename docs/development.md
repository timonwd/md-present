# Development

Run all commands from the repository root.

## Build from source

Go 1.27.0, Node.js 24, and pnpm 11 are required.

Build the current checkout for local development:

```sh
pnpm --dir cmd/md-present/web install --frozen-lockfile --ignore-scripts
pnpm --dir cmd/md-present/web run assets
go build -o md-present ./cmd/md-present
```

The generated Mermaid JavaScript and license files are intentionally ignored by
Git. Release binaries embed them, so Homebrew users do not need Node.js or pnpm.

## Standard validation

```sh
pnpm --dir cmd/md-present/web install --frozen-lockfile --ignore-scripts
pnpm --dir cmd/md-present/web run assets
pnpm --dir cmd/md-present/web audit --audit-level high
gofmt -w ./cmd/md-present/*.go
go mod verify
go vet ./...
go tool govulncheck ./...
go test ./...
go build ./...
```

For server or UI changes, also build the executable and smoke-test `--no-open`
against `fixtures/example.md`: wait for the URL, request the page and embedded
assets, verify representative slide content, then stop the process cleanly. For
layout or navigation changes, repeat the browser smoke test with
`fixtures/overflow.md` and verify the warning, collapsed indicator, scrolling,
and navigation boundary behavior.

For Mermaid renderer or browser changes, run `md-present --no-open
fixtures/mermaid-types.md` and open the printed URL. Confirm the eight baseline
diagrams render as SVG images, the invalid diagram and configuration-directive
example each show an in-slide alert with their escaped source, and no diagram
remains busy. This is a browser smoke test because Mermaid rendering occurs in
the embedded browser runtime rather than the Go server.

For media trust changes, run `md-present --no-open fixtures/external-media/deck.md`.
Confirm the prompt lists the outside-deck local image and both remote URLs; use
`--allow-external-media` only to exercise the explicit opt-in path. The
outside-deck image must be embedded, while the remote image and video remain
browser-loaded URLs.

## Agent plugin

Codex and Claude Code share the plugin package under `plugins/md-present/`.
Maintain the skill only at `plugins/md-present/skills/md-present/SKILL.md`; do
not introduce distribution-specific copies.

For skill or plugin changes, validate the skill and the shared plugin package
before handing off.
