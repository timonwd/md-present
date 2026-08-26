# Architecture

`md-present` is a small Go CLI that turns one Markdown file into a local browser
presentation. Keep the implementation direct and readable.

## Code layout

- Keep the executable in `cmd/md-present/` so release builds produce the
  `md-present` binary.
- Keep browser code as plain embedded HTML, CSS, and JavaScript under
  `cmd/md-present/web/`.
- Use `go:embed` for presentation UI assets.
- Keep the `package.json`, pnpm lockfile, and asset-generation script in
  `cmd/md-present/web/`. They materialize the pinned Mermaid browser asset
  before Go compilation. Keep generated assets ignored.
- Do not add a frontend framework or bundler. Release builds embed the generated
  Mermaid runtime so presentations remain offline-capable.
- Keep dependencies minimal. Goldmark remains the authority for CommonMark
  parsing and raw-HTML safety.

## Security and runtime invariants

- Bind only to `127.0.0.1` on an automatically selected port.
- Print the final local URL exactly once after the listener is ready.
- Never expose the source file or its directory as an arbitrary filesystem
  endpoint.
- Resolve relative Markdown images and videos from the deck directory, and
  embed local absolute paths in the generated page. Allow remote HTTP(S) image
  and video URLs; the viewer's browser loads those directly rather than
  proxying them through the local server.
- Require an explicit trust confirmation before opening a deck with remote or
  outside-deck local media, unless `--allow-external-media` was supplied. Do
  not infer trust from file provenance: download metadata is platform-specific
  and can be removed by copying or extracting a file.
- Render CommonMark raw HTML only after an explicit trust confirmation or the
  `--allow-raw-html` opt-in. Keep the restrictive Content Security Policy and
  Goldmark's dangerous-URL filtering for ordinary Markdown links; do not add
  inline script or style allowances.
- Preserve fence-aware `---` slide splitting, including fenced blocks nested in
  Markdown containers.
- Preserve graceful Ctrl+C and SIGTERM shutdown.
- Treat last-tab shutdown as best-effort and retain a grace period for refreshes
  and navigation. A presentation that never connects must not stop immediately.
- Browser-launch failure must stop the server rather than leave an orphan
  process.
- Keep keyboard shortcuts inactive when modifiers are held or focus is
  editable.
- Accept browser layout diagnostics only as bounded, same-origin JSON. Treat
  them as viewport-specific observations and keep the presentation URL as the
  only normal stdout output.

## Coupled browser behavior

Hash navigation, progress state, light and dark mode, the responsive 16:9
layout, live reload, and one-slide-per-page printing share browser state and
layout rules. Test them together when changing the presentation UI.

The README is the canonical description of current user-visible behavior and
product limits. Update it when those intentionally change instead of maintaining
a second feature list here.
