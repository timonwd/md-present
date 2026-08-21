---
name: md-present
description: Create, revise, validate, and run browser slide decks for the md-present CLI. Use when an agent needs to turn source material into an md-present deck, edit an existing deck, troubleshoot slide boundaries or images, launch a presentation, or verify its browser output.
---

# md-present

Use standard Markdown. Focus on md-present's presentation rules rather than teaching Markdown syntax.

## Prepare a deck

1. Keep each slide focused on one idea and sized for a 16:9 stage. Shorten content that would overflow; the browser deliberately clips the stage instead of scrolling it.
2. Separate slides with a line containing only `---`; surrounding whitespace is allowed. Do not use that line as a decorative rule.
3. Place local images beside the deck or in a relative subdirectory. md-present resolves relative image paths from the deck's directory and embeds their contents in the generated page.
4. Prefer local images when the presentation must work offline. Remote image URLs remain network-dependent.
5. Do not rely on raw HTML for layout or behavior; md-present omits it for safety.

Empty content before, after, or between separators does not create a slide. A `---` line inside a fenced code block stays in that code block, including fences nested in other Markdown structures.

## Respect the product boundary

Do not introduce or promise features outside the current tool:

- no live reload or user configuration
- no custom themes beyond automatic light and dark mode
- no presenter notes or transitions
- no Mermaid or math rendering
- no syntax-highlighting library
- no external UI assets or frontend build pipeline

Fenced code retains its language class but uses presentation styling without syntax coloring.

## Run the presentation

Use:

```text
md-present [--no-open] <deck.md>
```

Run `md-present --version` first when availability is uncertain. In an md-present source checkout, build with `go build -o md-present ./cmd/md-present`; do not add Node or another build system.

Use the default command for an interactive presentation. It prints one loopback URL and opens the browser. Use `--no-open` for automated validation; it prints the URL without launching a browser and continues running until signaled.

The server binds only to `127.0.0.1` on a free port. Stop it with Ctrl+C or SIGTERM. Closing the last connected presentation tab normally stops it after a short grace period, but treat that behavior as best-effort.

## Validate a deck

1. Confirm every relative image exists relative to the deck file.
2. Start the CLI with `--no-open` as a managed background process and wait for its single URL line.
3. Request that URL and verify a successful response, the expected slide count, and representative slide content.
4. When browser control is available, check the first and last slides, content fit, local images, and the absence of console errors.
5. Verify navigation when interaction changed: Right, Down, Page Down, and Space advance; Left, Up, and Page Up go back; Home and End jump.
6. Confirm the active slide uses a `#N` URL hash and an invalid hash normalizes to slide 1.
7. Stop the process cleanly after automated checks.

Printing should produce one slide per landscape page. The browser shows a subtle current/total counter during presentation but hides it for print.
