# Releasing md-present

GitHub Actions runs GoReleaser after a GitHub Release is published. It adds
an Apple silicon macOS archive and a checksum file to the existing release,
preserves its notes, and updates `Casks/md-present.rb` in this repository. Other
operating systems and Intel Macs are not supported release targets.

The repository doubles as its own explicitly named Homebrew tap, avoiding a
second repository. The release workflow uses its normal `GITHUB_TOKEN` with
`actions: read` to verify CI for the tagged commit. It creates a short-lived
installation token for the repository-scoped release GitHub App to upload
release assets and commit generated changes to `main`. The app needs
`contents: write`, must be installed on this repository, and must be an
always-allowed bypass actor for the default-branch pull-request ruleset. Its App
ID and private key are stored in the `RELEASE_APP_ID` and
`RELEASE_APP_PRIVATE_KEY` repository secrets.

## Publish a release

1. Run the checks in [`development.md`](development.md).
2. Commit and push the release changes.
3. On GitHub, draft a new release for the release commit. Create a plain
   semantic-version tag such as `0.1.0` (without a `v` prefix), write the
   release title and notes, and publish it.

Publishing the release starts `.github/workflows/release.yml`. The workflow
requires a successful `push` run of `.github/workflows/ci.yml` for the exact
tagged commit, then verifies that the tag matches the CLI and plugin versions
before adding artifacts. It does not rerun CI checks during publishing.
GoReleaser leaves the manually authored release notes unchanged. After the
artifacts and Homebrew cask are published, it calls the separate `Bump version`
workflow. That workflow advances the CLI, Codex and Claude plugin manifests,
and Claude marketplace to the next patch version in one commit on `main`.

The `Bump version` workflow can also be run manually. Leave its input blank to
increment the current patch version, or enter an exact semantic version to use
instead. It only updates the four version fields on `main`.

Pull requests and pushes to `main` run a non-publishing GoReleaser snapshot, so
packaging failures are caught before a release is published.
