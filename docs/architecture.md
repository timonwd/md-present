# Architecture

`md-present` is a small Go CLI that turns one Markdown file into a local browser
presentation. Keep the implementation direct and readable.

## Code layout

- Keep the executable in `cmd/md-present/` so `go install` produces the
  `md-present` binary.
- Keep browser code as plain embedded HTML, CSS, and JavaScript under
  `cmd/md-present/web/`.
- Use `go:embed` for presentation UI assets.
- Do not add Node, frontend frameworks, external UI assets, or a frontend build
  pipeline.
- Keep dependencies minimal. Goldmark remains the authority for CommonMark
  parsing and raw-HTML safety.

## Security and runtime invariants

- Bind only to `127.0.0.1` on an automatically selected port.
- Print the final local URL exactly once after the listener is ready.
- Never expose the source file or its directory as an arbitrary filesystem
  endpoint.
- Resolve relative Markdown images from the deck directory and embed them in
  the generated page.
- Keep raw HTML disabled and avoid script-injection paths when changing
  rendering.
- Preserve fence-aware `---` slide splitting, including fenced blocks nested in
  Markdown containers.
- Preserve graceful Ctrl+C and SIGTERM shutdown.
- Treat last-tab shutdown as best-effort and retain a grace period for refreshes
  and navigation. A presentation that never connects must not stop immediately.
- Browser-launch failure must stop the server rather than leave an orphan
  process.
- Keep keyboard shortcuts inactive when modifiers are held or focus is
  editable.

## Coupled browser behavior

Hash navigation, progress state, light and dark mode, the responsive 16:9
layout, live reload, and one-slide-per-page printing share browser state and
layout rules. Test them together when changing the presentation UI.

The README is the canonical description of current user-visible behavior and
product limits. Update it when those intentionally change instead of maintaining
a second feature list here.
