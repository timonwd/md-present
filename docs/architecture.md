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
- Accept browser layout diagnostics only as bounded, same-origin JSON. Treat
  them as viewport-specific observations and keep the presentation URL as the
  only normal stdout output.
- Validate loopback Host headers on both presentation and MCP HTTP servers so
  DNS rebinding cannot turn a local endpoint into a same-origin remote one.

## MCP service boundary

The optional MCP server is a separate, long-lived process managed by a macOS
LaunchAgent. Keep its stable Streamable HTTP listener separate from the
ephemeral presentation listeners:

- Bind the MCP endpoint only to `127.0.0.1` and require the configured port in
  Host and Origin headers. Do not enable CORS or non-loopback listening.
- Keep the transport stateless and use the official MCP Go SDK rather than
  maintaining protocol framing in this repository.
- Expose file presentation through `present_file`. Require an absolute regular
  file path, resolve symlinks, and use the resolved file's containing directory
  as the deck root.
- Preserve external-media consent. A first call reports remote and
  outside-directory references as a completed, structured tool error with
  `approval_required`; only an explicit `allow_external_media` retry may bypass
  that report.
- Limit concurrently running MCP-created presentations. Each presentation
  retains its own live-reload watcher, loopback listener, last-tab grace period,
  and browser lifecycle.
- Treat all same-user local processes as trusted. The MCP server is intentionally
  unauthenticated for clients that cannot send authorization headers; this is
  acceptable only while loopback binding and Host/Origin validation remain
  mandatory.
- Keep LaunchAgent installation and removal explicit and reversible. The plist
  points to the invoked executable path so Homebrew's stable binary symlink can
  survive upgrades.

## Coupled browser behavior

Hash navigation, progress state, light and dark mode, the responsive 16:9
layout, live reload, and one-slide-per-page printing share browser state and
layout rules. Test them together when changing the presentation UI.

The README is the canonical description of current user-visible behavior and
product limits. Update it when those intentionally change instead of maintaining
a second feature list here.
