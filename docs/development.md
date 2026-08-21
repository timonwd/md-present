# Development

Run all commands from the repository root.

## Standard validation

```sh
gofmt -w ./cmd/md-present/*.go
go vet ./...
go test ./...
go build ./...
```

For server or UI changes, also build the executable and smoke-test `--no-open`
against `example.md`: wait for the URL, request the page and embedded assets,
verify representative slide content, then stop the process cleanly.

## Agent skill and plugin mirrors

The full skill is intentionally duplicated for different distribution systems.
Keep these files byte-identical:

- `SKILL.md`
- `skills/md-present/SKILL.md`
- `plugins/md-present/skills/md-present/SKILL.md`

Also keep these Codex UI metadata files byte-identical:

- `skills/md-present/agents/openai.yaml`
- `plugins/md-present/skills/md-present/agents/openai.yaml`

`cmd/md-present/skill_test.go` enforces the `SKILL.md` copies. Extend that test
if another skill mirror is introduced.

For skill or plugin changes, validate every skill copy and the packaged Codex
plugin. Confirm all mirrored files still match before handing off.
