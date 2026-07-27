# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Given the project is pre-1.0 (`0.x.y`), minor (`0.MINOR.0`) releases may include
behavioral and API changes that would be considered breaking after 1.0; patch
(`0.x.PATCH`) releases are backwards-compatible fixes.

## [Unreleased]

_Nothing yet. Move entries here as they land on master, then promote them under
a new version heading at release time (see RELEASING.md)._

## [0.8.0] - 2026-07-27

First release cut since v0.7.0. Brings the toolchain and CI up to date, clears
most known vulnerabilities, and adds a test suite. Contains one externally
visible breaking change to the HTTP API.

### ⚠ Breaking changes

- **`AuthAPI` now returns HTTP `401 Unauthorized` on a failed API-key check
  instead of `200 OK`.** The response body already reported `401` in its
  `status` field, but the actual HTTP status was incorrectly `200`. Any client
  branching on the HTTP status code (rather than the body) will now correctly
  see `401`. If a client was relying on the prior `200`, update it to expect
  `401` on bad credentials. ([3d581ba](https://github.com/bitcav/nitr/commit/3d581ba))

### Added

- Test suite across all packages (~92% coverage), including `cmd`, `database`,
  `handlers`, `models`, `utils`, `version`, and `main`. ([57bcfd6](https://github.com/bitcav/nitr/commit/57bcfd6))
- `Draft Release` CI job that, on a `v*` tag push, attaches the four
  cross-compiled binaries (`nitr_linux_amd64`, `nitr_linux_386`,
  `nitr_windows_amd64.exe`, `nitr_windows_386.exe`) to a draft GitHub release. ([32deb54](https://github.com/bitcav/nitr/commit/32deb54), [468e3ee](https://github.com/bitcav/nitr/commit/468e3ee))

### Changed

- Migrated CI from Travis to GitHub Actions (`.github/workflows/ci.yml`):
  `Vet & Test`, `Cross-compile binaries`, and `Draft Release` jobs. ([32deb54](https://github.com/bitcav/nitr/commit/32deb54))
- Bumped all GitHub Actions off the deprecated Node 20 runtime. ([d6fd5cf](https://github.com/bitcav/nitr/commit/d6fd5cf))
- Bumped the Go toolchain: `go` directive `1.13 → 1.26`, `toolchain` pinned to
  `go1.26.5`; the `go` version badge (`images/goversion.svg`) updated to match. ([f90cc72](https://github.com/bitcav/nitr/commit/f90cc72), [07943fc](https://github.com/bitcav/nitr/commit/07943fc))
- `database.GetUserByID` now returns `(models.User, error)` instead of panicking
  on DB open / unmarshal errors, and `database.GetApiKey` now returns
  `(string, error)`. Callers in `cmd/` and `handlers/` propagate the error
  rather than crashing. This is an internal Go API signature change. ([9912861](https://github.com/bitcav/nitr/commit/9912861))
- `utils.OpenBrowser` now returns an `error` instead of calling `log.Fatal`
  when the platform has no opener (`xdg-open`, `open`, etc.), so a missing
  browser helper no longer hard-crashes the process (notably in CI). ([6ed031e](https://github.com/bitcav/nitr/commit/6ed031e))
- Replaced the retired Go Report Card badge with a self-hosted SVG
  (`images/goreport.svg`). ([1298ba1](https://github.com/bitcav/nitr/commit/1298ba1))

### Fixed

- Test correctness: changed `assert.NoError` to `require.NoError` after
  `app.Test(req, timeout)` so that a timeout — which returns a nil
  `*http.Response` — stops the test immediately instead of letting the next
  line dereference `resp.StatusCode` and panic over the real timeout failure. ([d88e164](https://github.com/bitcav/nitr/commit/d88e164))

### Security

- Bumped vulnerable dependencies: `gorilla/schema v1.4.1`,
  `valyala/fasthttp v1.34.0`, and (via the Go toolchain bump) cleared 9 stdlib
  advisories. `govulncheck` findings dropped from 15 to 4. The remaining 4 are
  in `fiber v1.11.1` (EOL, no upstream fix); resolving them requires a
  `fiber v1 → v2` migration, tracked separately. ([f90cc72](https://github.com/bitcav/nitr/commit/f90cc72))

[Unreleased]: https://github.com/bitcav/nitr/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/bitcav/nitr/releases/tag/v0.8.0
