# md-present agent guide

Use this file as an index. Read the canonical document for the part of the
repository you are changing:

- [`README.md`](README.md): user-facing installation, usage, behavior, and
  current product limits.
- [`docs/architecture.md`](docs/architecture.md): implementation boundaries,
  security constraints, and runtime invariants.
- [`docs/development.md`](docs/development.md): validation, smoke testing, and
  skill/plugin mirror rules.
- [`docs/releasing.md`](docs/releasing.md): version alignment and the release
  process.

Update information only in its canonical document. Do not copy maintainer or
contributor guidance into the README, and do not duplicate the linked documents
in this index.

## Agent-specific instructions

- Keep changes focused on the requested outcome and preserve unrelated work.
- Treat the documented product limits as intentional unless the user explicitly
  asks to change them.
- Stage only intended paths or hunks. Do not commit or push unless explicitly
  requested.
- When a feature or fix is requested to be merged to `main`, commit the scoped
  change on its branch, push the branch, and create a pull request targeting
  `main`. Include `Closes #<issue-number>` (or the equivalent GitHub closing
  keyword) in the pull request body so the related issue closes automatically
  when the pull request is merged.
- Report validation results accurately, including checks blocked by the
  environment.
