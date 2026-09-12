# Development

Run all commands from the repository root.

## Build from source

Go 1.26.6, Node.js 24, and pnpm 11 are required.

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

For editor changes, run `md-present --no-open fixtures/example.md`.
Open the reported URL, use **Edit** to load the current slide's Markdown, make
a small change, and save with Cmd/Ctrl+S. Confirm the active preview updates
without closing the editor, an invalid empty deck remains unsaved with an editor-local error, and
an external file edit produces a save conflict rather than an overwrite.

For raw HTML or column-layout changes, run `md-present --no-open
fixtures/layouts.md` and confirm the raw-HTML trust prompt appears. After
approval, verify the two- and three-column slides in regular view, overview,
and print preview; confirm the local image is embedded and inline HTML remains
visible. Use `--allow-raw-html` only to exercise the explicit opt-in path.

For theme changes, create a small deck with top-of-file `theme:` front matter
and a shared JSON theme file. The front-matter theme name may omit `.json`; test
both forms for a deck-relative theme and a bare theme resolved from
`~/.md-present/themes/`. Confirm the rendered page applies the selected colors
and typography in both system color schemes, including a Mermaid slide, then
edit the theme file and confirm the open presentation reloads. Invalid or
unknown theme fields must fail before the browser opens.

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

For MCP changes, run the Go integration tests and build `md-present`, then start
`md-present mcp serve --port <unused-port>`. Connect a Streamable HTTP MCP client
to the printed `/mcp` endpoint, list tools, and call `present_file` with an
absolute fixture path. Verify the browser opens, the returned presentation URL
loads, live reload works, foreign Host and Origin headers are rejected, and an
HTML or external-media deck is refused until its explicit approval field is
set.

For LaunchAgent changes, use a disposable port and a binary in a stable path.
Run `md-present mcp install --port <unused-port>`, confirm the service with
`launchctl print gui/$(id -u)/com.timonwd.md-present.mcp`, exercise the MCP tool,
then run `md-present mcp uninstall` and confirm the plist and service are gone.
Do not leave a development LaunchAgent installed after validation.

## Agent plugin

Codex and Claude Code share the plugin package under `plugins/md-present/`.
Maintain the skill only at `plugins/md-present/skills/md-present/SKILL.md`; do
not introduce distribution-specific copies.

For skill or plugin changes, validate the skill and the shared plugin package
before handing off.
