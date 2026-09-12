![md-present — Markdown in. Slides out.](fixtures/assets/md-present-cover.png)

# md-present

Turn a Markdown file into a clean, structured browser slide deck. Everything runs on your machine—no account, configuration, or internet connection required.

md-present is built to be agent-first. Agents already use Markdown for almost everything, so md-present gives them a way to clearly show ideas, plans, architectures, evaluations, and other structured work without forcing them into a separate format. The Markdown remains the source: easy to create, inspect, revise, and version, with a focused visual view whenever you need it.

## Features

- GitHub-Flavored Markdown, syntax highlighting, and Mermaid diagrams
- Trusted CommonMark raw HTML with built-in column layouts
- Local images and videos embedded in the browser output, with remote HTTP(S) media loaded on demand
- Light and dark mode
- Slide overview for quickly navigating longer decks
- Fullscreen viewing and presenting
- Optional local MCP server for clients that can call HTTP tools but cannot run the CLI

## Install

Requires an Apple silicon Mac.

```sh
brew tap timonwd/md-present https://github.com/timonwd/md-present
brew install --cask md-present
```

### Agent plugin

Install the shared skill for Codex:

```sh
codex plugin marketplace add timonwd/md-present
codex plugin add md-present@md-present
```

For Claude Code, run the equivalent commands inside a session:

```text
/plugin marketplace add timonwd/md-present
/plugin install md-present@md-present
```

Tools that support direct skill imports can download the canonical [`SKILL.md`](plugins/md-present/skills/md-present/SKILL.md).

### Local MCP server

Install an always-running per-user MCP server when an agent can reach localhost and the same filesystem but cannot execute `md-present` itself:

```sh
md-present mcp install
```

The command installs and starts a macOS LaunchAgent, then prints its Streamable HTTP endpoint. The default is `http://127.0.0.1:38473/mcp`; use `--port <port>` during installation when the default conflicts with another local service. Configure that URL in the MCP client. Homebrew upgrades restart an installed MCP LaunchAgent, so it runs the updated binary.

The server exposes `present_file`. Pass an absolute Markdown file path; its containing directory becomes the root for relative images and videos. The tool opens the presentation in the browser and returns its loopback URL. Raw HTML and external media use the same explicit trust boundaries as the CLI: a first call completes with a structured MCP tool error, `approval_required` is `true`, `raw_html` identifies HTML requiring approval, and `external_media` lists blocked remote or outside-directory references. No browser opens. The caller must show the reported content to the user and retry with `allow_raw_html: true` or `allow_external_media: true` only after the corresponding approval.

Remove and stop the service with:

```sh
md-present mcp uninstall
```

The MCP endpoint binds only to IPv4 loopback and rejects non-loopback Host and Origin headers. It does not authenticate requests because it is intended for MCP clients that cannot configure authorization headers. Any process running locally as the user can therefore call it; do not expose or proxy the endpoint beyond localhost. Diagnostic output is written to `~/Library/Logs/md-present-mcp.log`.

## Ideal workflow

1. Give your agent the context and ask it to structure an idea, plan, architecture, evaluation, or other piece of work in Markdown.
2. Ask it to run the file with md-present so you can explore the result as a focused browser view.
3. Review the structure and content, then give the agent concrete feedback.
4. Let the agent revise and validate the Markdown. The browser reloads whenever the file or local media changes.
5. Keep using the Markdown as a working artifact, share it through version control, or open the browser view in fullscreen when you want to present it.

## Use

```sh
md-present plan.md
```

Add `---` on its own line to start a new slide:

```md
# First slide

Hello, world.

---

## Second slide

- Simple Markdown
- No setup
```

The structured view opens in your browser and reloads when the Markdown file or local media changes.

### Themes

Decks can share a small, safe theme file. Reference it in front matter at the very start of each Markdown document:

```md
---
theme: ../shared/acme-theme
---

# Quarterly review
```

The `.json` extension is optional; the shorter `theme: acme-theme` form is recommended. md-present first resolves the name relative to the deck, then—when it is a bare name such as `acme-theme`—also looks in `~/.md-present/themes/acme-theme.json`. This lets you install an organization theme once and select it from any document with `theme: acme-theme`.

The referenced JSON file contains only visual tokens; it cannot add CSS, HTML, or JavaScript. Reuse the same file from any number of decks. All fields are optional, so omitted values retain md-present's default appearance:

```json
{
  "colors": {"canvas": "#f4f1ea", "stage": "#fffaf0", "ink": "#202020", "accent": "#a03d2f"},
  "darkColors": {"canvas": "#15110f", "stage": "#231b17", "ink": "#fff7ed", "accent": "#ffb4a8"},
  "fontFamily": "Avenir Next, Arial, sans-serif",
  "headingFontFamily": "Avenir Next Condensed, Arial, sans-serif",
  "headingWeight": "semibold",
  "headingLetterSpacing": "-0.04em",
  "headingAccent": "#a03d2f",
  "stageGutter": "2rem",
  "contentGutter": "2.5rem",
  "radius": "0.5rem",
  "shadow": "subtle",
  "footer": "Acme · Internal"
}
```

`colors`, `darkColors`, and optional `printColors` accept the tokens `canvas`, `stage`, `ink`, `muted`, `line`, `accent`, and `code`, each as a hexadecimal color. The remaining optional tokens are:

| Area | Tokens |
| --- | --- |
| Typography | `fontFamily`, `headingFontFamily`, `headingWeight` (`regular`, `medium`, `semibold`, `bold`), `headingLetterSpacing`, `headingCase`, `headingAccent` |
| Layout and surfaces | `stageGutter`, `contentGutter`, `radius`, `shadow` (`none`, `subtle`, `strong`), `layout` (`default`, `compact`, `editorial`) |
| Content | `tableHeader`, `tableStripe`, `codeBackground`, `quoteAccent`, `linkColor` |
| Cover slide | `coverAlignment` (`start`, `center`, `end`), `coverBackground`, `coverInk` |
| Mermaid | `mermaidNode`, `mermaidEdge`, `mermaidLabel` |
| Repeated metadata | `footer` (a short, single-line label) |

Font stacks accept a conventional comma-separated value. Length values accept one CSS length such as `2rem` or `24px`; all color values use hexadecimal notation. The tokens apply consistently to slides, controls, tables, code backgrounds, and Mermaid diagrams. A changed theme file triggers the same live reload as a changed deck file.
Before a deck with raw HTML opens, md-present asks whether you trust it; use `--allow-raw-html` to opt in without the prompt.
Remote media and local media outside the deck folder use a separate trust prompt; use `--allow-external-media` to opt in without that prompt.
Use `md-present --no-open plan.md` to start without opening a browser.
Press Ctrl+C in the terminal to stop the process.

Use the **Edit** button next to the slide overview to open a local source
editor. It contains only the current slide's Markdown and previews valid edits
without writing the file. Save with Cmd/Ctrl+S to persist the change; **Cancel**
restores the saved slide. Drag the panel's left edge to resize it. It works only
for the deck file selected on the command line and rejects saves when that file
changed outside the editor. It intentionally remains a plain textarea for now
so the Markdown renderer stays the source of truth; a richer editor can replace
this surface later without changing the save API. While editing, use the footer
arrows to change slides; they ask before discarding unsaved changes.

When a network connection is available, md-present also checks GitHub Releases in the background. If a newer version exists, it shows an update notice in the presentation and prints the Homebrew upgrade command to stderr; the check is optional and never blocks presenting offline.

### HTML and columns

CommonMark raw HTML is supported for trusted decks. Keep blank lines between HTML container tags and Markdown content so CommonMark parses the inner content as Markdown.

Use the built-in `columns` and `column` classes for equal-width columns:

```md
# Comparison

<div class="columns">

<div class="column">

## Left

- Standard Markdown
- **Formatting** still works

</div>

<div class="column">

## Right

![Architecture](architecture.png)

</div>

</div>
```

Two or three columns work best on the 16:9 stage. Use Markdown image syntax inside HTML containers so local media remains embedded, watched for changes, and covered by the external-media trust check. Raw HTML runs under md-present's restrictive Content Security Policy, but it can still change presentation behavior or load external resources; trust only files you intend to present.

### Mermaid diagrams

Use a fenced `mermaid` block for diagrams. The bundled offline Mermaid runtime supports flowcharts, sequence, class, state, entity-relationship, timeline, mind map, and architecture diagrams. Other Mermaid diagram types may also render, but are not part of the compatibility baseline. For syntax and examples, see the [Mermaid documentation](https://mermaid.ai/open-source/intro/syntax-reference.html).

## Controls

| Action | Key |
| --- | --- |
| Next slide | Right, Down, Page Down, or Space |
| Previous slide | Left, Up, or Page Up |
| First / last slide | Home / End |
| Toggle slide overview | Button in the upper-left corner or O |
| Navigate / select in overview | Arrow keys / Enter |
| Close presentation tab | Escape (when the browser permits script-closed tabs) |
| Fullscreen | Button in the upper-right corner |

Oversized slides scroll before navigation continues. Their content may be clipped when printed.

## License

[MIT](LICENSE). Third-party licenses are listed in [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
