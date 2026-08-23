---
name: md-present
description: Create, revise, validate, and run browser slide decks for the md-present CLI. Use when an agent needs to turn source material into an md-present deck, edit an existing deck, troubleshoot slide boundaries or images, launch a presentation, or verify its browser output.
---

# md-present

Use standard Markdown and GitHub-Flavored Markdown tables, strikethrough, task lists, and automatic links. Add a language to fenced code blocks when syntax highlighting is useful. Use a fenced `mermaid` block for diagrams that benefit from flow, sequence, hierarchy, or other relationships. Focus on md-present's presentation rules rather than teaching Markdown syntax.

## Prepare a deck

1. Keep each slide focused on one idea and sized for a 16:9 stage. Shorten content that would overflow when practical. The browser warns about oversized slides and makes them scrollable while presenting, but printing still uses a fixed landscape page and may clip overflow.
2. Separate slides with a line containing only `---`; surrounding whitespace is allowed. Do not use that line as a decorative rule.
3. Place local images beside the deck or in a relative subdirectory. md-present resolves relative image paths from the deck's directory and embeds their contents in the generated page.
4. Prefer local images when the presentation must work offline. Remote image URLs remain network-dependent.
5. Do not rely on raw HTML for layout or behavior; md-present omits it for safety.

Empty content before, after, or between separators does not create a slide. A `---` line inside a fenced code block stays in that code block, including fences nested in other Markdown structures.

## Respect the product boundary

Do not introduce or promise features outside the current tool:

- no user configuration
- no custom themes beyond automatic light and dark mode
- no presenter notes or transitions
- no math rendering
- no runtime-loaded external UI assets, frontend framework, or bundler

Fenced Mermaid diagrams and recognized code languages render locally from embedded assets. Unknown or omitted languages retain escaped plain-code presentation styling.

## Run the presentation

Use:

```text
md-present [--no-open] <deck.md>
```

Run `md-present --version` first when availability is uncertain. In an md-present source checkout, run `pnpm --dir cmd/md-present/web install --frozen-lockfile --ignore-scripts`, `pnpm --dir cmd/md-present/web run assets`, then `go build -o md-present ./cmd/md-present`.

Use the default command for an interactive presentation. It prints one loopback URL and opens the browser. Use `--no-open` for automated validation; it prints the URL without launching a browser and continues running until signaled.

The server binds only to `127.0.0.1` on a free port. Stop it with Ctrl+C or SIGTERM. Closing the last connected presentation tab normally stops it after a short grace period, but treat that behavior as best-effort.

Saving the Markdown file or a referenced local image live-reloads connected tabs. The URL hash preserves the active slide when it still exists; if a save cannot be rendered, the last valid presentation stays visible until the source is valid again.

After a browser lays out the deck, md-present temporarily warns about slides that exceed the regular 16:9 slide area and writes the same diagnostic to the terminal. The warning is viewport-specific: with `--no-open`, no overflow can be reported until a browser opens the printed URL. Oversized slides scroll while normal slides retain their fixed layout. On an oversized slide, the warning collapses to a small indicator that can expand it again.

## Validate a deck

1. Confirm every relative image exists relative to the deck file.
2. Start the CLI with `--no-open` as a managed background process and wait for its single URL line.
3. Request that URL and verify a successful response, the expected slide count, and representative slide content.
4. When browser control is available, check the first and last slides, content fit, local images, and the absence of console errors.
5. Treat an overflow warning as a reason to simplify or split a slide unless scrolling is intentional. For intentional overflow, verify the browser warning and terminal diagnostic, the collapsed indicator, and scrolling to all content. In a source checkout, use `fixtures/overflow.md` for this behavior.
6. Verify navigation when interaction changed: Right always advances; Left always goes back; Down, Page Down, and Space scroll an oversized slide before advancing at its bottom; Up and Page Up scroll it before going back at its top; Home and End jump. Confirm navigation stays bounded on the first and last slides.
7. Confirm the active slide uses a `#N` URL hash and an invalid hash normalizes to slide 1.
8. Stop the process cleanly after automated checks.

Printing should produce one slide per landscape page. The browser shows a subtle current/total counter during presentation but hides it for print.
