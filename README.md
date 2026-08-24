![md-present — Markdown in. Slides out.](fixtures/assets/md-present-cover.png)

# md-present

Turn a Markdown file into a clean, structured browser slide deck. Everything runs on your machine—no account, configuration, or internet connection required.

md-present is built to be agent-first. Agents already use Markdown for almost everything, so md-present gives them a way to clearly show ideas, plans, architectures, evaluations, and other structured work without forcing them into a separate format. The Markdown remains the source: easy to create, inspect, revise, and version, with a focused visual view whenever you need it.

## Features

- GitHub-Flavored Markdown, syntax highlighting, and Mermaid diagrams
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

The command installs and starts a macOS LaunchAgent, then prints its Streamable HTTP endpoint. The default is `http://127.0.0.1:38473/mcp`; use `--port <port>` during installation when the default conflicts with another local service. Configure that URL in the MCP client.

The server exposes `present_file`. Pass an absolute Markdown file path; its containing directory becomes the root for relative images and videos. The tool opens the presentation in the browser and returns its loopback URL. When remote media, absolute local media, or local media outside the file's directory is present, the first call completes with a structured MCP tool error: `error` states that `allow_external_media` is required, `approval_required` is `true`, and `external_media` lists the blocked references. No browser opens. The caller must show those references to the user and retry with `allow_external_media: true` only after explicit approval.

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
Before a deck with remote media or local media outside its folder opens, md-present asks whether you trust it; use `--allow-external-media` to opt in without the prompt.
Use `md-present --no-open plan.md` to start without opening a browser.
Press Ctrl+C in the terminal to stop the process.

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
