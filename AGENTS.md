# md-present agent instructions

## Project intent

Keep md-present a small, readable Go CLI that turns one Markdown file into a local browser presentation. Favor a coherent implementation over scaffolding for hypothetical future features.

## Architecture

- Keep the executable in `cmd/md-present/` so `go install` produces the `md-present` binary.
- Keep browser code as plain embedded HTML, CSS, and JavaScript under `cmd/md-present/web/`.
- Do not add Node, frontend frameworks, external UI assets, or a frontend build pipeline.
- Keep dependencies minimal. Goldmark is the Markdown renderer and should remain the authority for CommonMark parsing and raw-HTML safety.
- Use `go:embed` for presentation UI assets.

## Behavioral invariants

- Bind only to `127.0.0.1` on an automatically selected port.
- Print the final local URL exactly once after the listener is ready.
- Never expose the source file or its directory as an arbitrary filesystem endpoint.
- Resolve relative Markdown images from the deck directory and embed them in the generated page.
- Keep raw HTML disabled and avoid introducing script-injection paths when changing rendering.
- Preserve fence-aware `---` slide splitting, including fenced blocks nested in Markdown containers.
- Preserve graceful Ctrl+C and SIGTERM shutdown.
- Treat last-tab shutdown as best-effort and retain a grace period for refreshes and navigation. A presentation that never connects must not stop immediately.
- Browser-launch failure must stop the server rather than leave an orphan process.
- Keep keyboard shortcuts inactive when modifiers are held or focus is editable.
- Keep URL hashes, progress state, light/dark mode, responsive 16:9 layout, and one-slide-per-page printing working together.

## Product boundary

Do not add live reload, user configuration, extra themes, presenter notes, transitions, Mermaid, math, syntax-highlighting libraries, packaging automation, or release automation unless explicitly requested.

## Agent skill and plugin mirrors

The full skill is intentionally duplicated for different distribution systems. Keep these files byte-identical:

- `SKILL.md`
- `skills/md-present/SKILL.md`
- `plugins/md-present/skills/md-present/SKILL.md`

Also keep these Codex UI metadata files byte-identical:

- `skills/md-present/agents/openai.yaml`
- `plugins/md-present/skills/md-present/agents/openai.yaml`

`cmd/md-present/skill_test.go` enforces the `SKILL.md` copies. Extend that test if another skill mirror is introduced.

Keep the CLI version and the Codex and Claude plugin manifest versions aligned when preparing a release.

## Validation

Run from the repository root after changes:

```sh
gofmt -w ./cmd/md-present/*.go
go vet ./...
go test ./...
go build ./...
```

For server or UI changes, also build the executable and smoke-test `--no-open` against `example.md`: wait for the URL, request the page and embedded assets, verify representative slide content, then stop the process cleanly.

For skill or plugin changes, validate every skill copy and the packaged Codex plugin. Confirm all mirrored files still match before handing off.

## Change hygiene

- Preserve unrelated work and stage only intended files.
- Do not commit or push unless explicitly requested.
- Report validation results accurately, including environment-blocked checks.
