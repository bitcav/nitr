# Releasing

Cutting a release is a version-bump commit followed by an annotated tag. The tag
push triggers CI to draft a GitHub release with binaries attached; a human then
publishes it.

## 1. Bump the version

Pick the next semver number (`0.MINOR.0` for a release with changes, `0.x.PATCH`
for a fixes-only release — the project is pre-1.0).

Regenerate the three version-bearing files from their templates with the
project's own tool:

```sh
go run releaser.go -version="X.Y.Z"
```

This rewrites `version/version.go`, `versioninfo.json`, and
`images/release.svg`. Then update the one hardcoded copy the tool does **not**
touch:

- `version/version_test.go` — the `assert.Equal(t, "X.Y.Z", Version)` line.

Grep to confirm nothing else still holds the old number:

```sh
git grep -n "$(git describe --tags --abbrev=0 | sed 's/^v//')"
```

## 2. Close the changelog entry

In `CHANGELOG.md`, rename the `## [Unreleased]` section to
`## [X.Y.Z] - YYYY-MM-DD`, add a fresh empty `## [Unreleased]` section above it,
and add the compare link at the bottom:

```
[Unreleased]: https://github.com/bitcav/nitr/compare/vX.Y.Z...HEAD
[X.Y.Z]: https://github.com/bitcav/nitr/releases/tag/vX.Y.Z
```

## 3. Commit and tag

```sh
git add -A
git commit -m "Release vX.Y.Z"
git tag -a vX.Y.Z -m "Release vX.Y.Z"
```

## 4. Push

```sh
git push && git push --tags
```

Pushing the tag runs the `release` (Draft Release) job in
`.github/workflows/ci.yml`. Because the workflow only fires on `refs/tags/v*`
(see the `if: startsWith(github.ref, 'refs/tags/')` guard), ordinary `master`
pushes do **not** draft a release.

## 5. Publish on GitHub

Once CI is green, open the draft release at
<https://github.com/bitcav/nitr/releases>. It is named `nitr vX.Y.Z` and has
the five binaries attached:

- `nitr_linux_amd64`
- `nitr_linux_386`
- `nitr_linux_arm64`
- `nitr_windows_amd64.exe`
- `nitr_windows_386.exe`

Edit the body (paste the changelog entry for `X.Y.Z`), then mark it as a
non-draft release to publish.
