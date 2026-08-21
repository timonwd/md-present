# md-present

`md-present` turns one Markdown file into a clean, local browser presentation. It is a small Go CLI with an embedded interface: no account, configuration, frontend toolchain, or network dependency is required.

## Install from source

Go 1.22 or newer is required.

```sh
go install github.com/timonwd/md-present/cmd/md-present@latest
```

From a local checkout:

```sh
go build -o md-present ./cmd/md-present
```

## Usage

```sh
md-present [--no-open] <markdown-file>
```

Slides are separated by a line containing only `---` (surrounding whitespace is allowed). Separators inside fenced code blocks are preserved as code.
Relative image paths are resolved from the Markdown file's directory and embedded in the generated presentation.

```sh
md-present ./example.md
md-present --no-open ./example.md
```

The server listens only on a random `127.0.0.1` port. Without `--no-open`, the default browser opens automatically. Closing the last connected presentation tab normally stops the server after a short grace period; browser crashes and forced termination may prevent a final disconnect from being observed. Use Ctrl+C to stop it explicitly.

## Controls

| Action | Keys |
| --- | --- |
| Next slide | Right, Down, Page Down, Space |
| Previous slide | Left, Up, Page Up |
| First slide | Home |
| Last slide | End |

The active slide is stored in the URL hash. Printing uses one slide per landscape page.

## MVP limitations

This first release intentionally has no live reload, custom themes, presenter notes, transitions, Mermaid, math rendering, or syntax highlighting. Raw HTML in Markdown is not rendered. User-supplied remote image URLs may require network access in the browser; the presentation UI itself has no external dependencies.

## Development

```sh
gofmt -w ./cmd/md-present/*.go
go vet ./...
go test ./...
go build ./...
```
