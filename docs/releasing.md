# Releasing md-present

GitHub Actions runs GoReleaser after a GitHub Release is published. It adds
macOS, Linux, and Windows archives and a checksum file to the existing release,
preserves its notes, and updates `Casks/md-present.rb` in this repository.

The repository doubles as its own explicitly named Homebrew tap, avoiding a
second repository and cross-repository access token. The release workflow needs
`contents: write` so its normal `GITHUB_TOKEN` can upload release assets and
commit the generated cask to `main`.

## Publish a release

1. Update the CLI version in `cmd/md-present/main.go` and keep both plugin
   manifests and the Claude marketplace version aligned.
2. Run the checks in [`development.md`](development.md).
3. Commit and push the release changes.
4. On GitHub, draft a new release for the release commit. Create a plain
   semantic-version tag such as `0.1.0` (without a `v` prefix), write the
   release title and notes, and publish it.

Publishing the release starts `.github/workflows/release.yml`. The workflow
verifies that its tag matches the CLI and plugin versions before adding
artifacts. GoReleaser leaves the manually authored release notes unchanged.

Pull requests and pushes to `main` run a non-publishing GoReleaser snapshot, so
packaging failures are caught before a release is published.
