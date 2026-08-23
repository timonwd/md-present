![md-present — Markdown in. Slides out.](fixtures/assets/md-present-cover.png)

# md-present

Turn a Markdown file into a clean, local browser presentation. Everything runs on your machine—no account, configuration, or internet connection required.

## Features

- GitHub-Flavored Markdown, syntax highlighting, and Mermaid diagrams
- Local images embedded in the presentation
- Light and dark mode
- Fullscreen presentation

## Install

Requires an Apple silicon Mac.

```sh
brew tap timonwd/md-present https://github.com/timonwd/md-present
brew install --cask md-present
```

## Use

```sh
md-present slides.md
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

The presentation opens in your browser and reloads when the Markdown file or a local image changes. Relative image paths are resolved from the Markdown file's folder.
Use `md-present --no-open slides.md` to start without opening a browser. Press Ctrl+C in the terminal to stop the presentation.

## Controls

| Action | Key |
| --- | --- |
| Next slide | Right, Down, Page Down, or Space |
| Previous slide | Left, Up, or Page Up |
| First / last slide | Home / End |
| Fullscreen | Button in the upper-right corner |

Oversized slides scroll before navigation continues. Their content may be clipped when printed.

## Agent plugin

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

### Direct Markdown import

Tools that support direct skill imports can download the canonical [`SKILL.md`](plugins/md-present/skills/md-present/SKILL.md).

## License

[MIT](LICENSE). Third-party licenses are listed in [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
