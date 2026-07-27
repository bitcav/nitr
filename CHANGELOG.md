# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Given the project is pre-1.0 (`0.x.y`), minor (`0.MINOR.0`) releases may include
behavioral and API changes that would be considered breaking after 1.0; patch
(`0.x.PATCH`) releases are backwards-compatible fixes.

## [Unreleased]

### ⚠ Breaking changes

- **`/api/v1/memory` no longer returns `200 OK` with a `null` body when the
  memory collector fails.** It previously printed the error to server stdout
  and returned `200` with an empty body, so a caller could not distinguish
  "needs root", "broken", or "no memory devices" from success — every poll
  looked healthy. It now returns `403 Forbidden` when the underlying error is
  a permission error and `500 Internal Server Error` otherwise, with the
  standard error envelope (`{"message": "...", "status": <code>}`), so
  callers branching on the HTTP status (rather than the body) must react.
  Verified on the wire, running non-root: `GET /api/v1/memory` → `403
  {"message":"open /dev/mem: permission denied","status":403}`. The DMI
  endpoints (`/product`, `/chassis`, `/baseboard`) keep their per-field
  `"unknown"` degradation behaviour and are unchanged. ([728e195](https://github.com/bitcav/nitr/commit/728e195))

### Added

- Liveness and readiness probes at `GET /health` and `GET /ready`, plus a
  Docker `HEALTHCHECK` that polls `/health`. Both routes are registered on the
  root app before the `x-api-key` middleware and the panel session auth, so
  they answer with **no credentials** and never redirect to the login page —
  the point being that Docker / Compose / Kubernetes / uptime checkers can
  poll them unauthenticated. `/health` does no I/O and returns
  `200 {"status":"ok","version":"0.9.0"}`. `/ready` does an
  `os.Stat("nitr.db")` and nothing more: `200 {"status":"ready"}` when the
  file is present, `503 {"status":"not ready","error":"..."}` (the raw stat
  error) when it is not. It confirms the database file exists; it does **not**
  confirm bolt can be opened right now, nor that the DB is uncorrupted —
  `database.SetupDB` opens bolt with nil options whose exclusive flock has no
  timeout and blocks forever under contention, so a probe that opened the DB
  could pile up blocked goroutines when polled every few seconds. The Docker
  `HEALTHCHECK` (`wget -q -O /dev/null http://localhost:8000/health`,
  30s interval / 5s timeout / 10s start period / 3 retries) makes `docker ps`
  report a health status for the container. ([0262123](https://github.com/bitcav/nitr/commit/0262123))

## [0.9.0] - 2026-07-27

### ⚠ Breaking changes

- **`nitr key`, `nitr passwd`, and `nitr qr` now fail when `nitr.db` is absent
  instead of silently creating one.** They previously provisioned a fresh
  `nitr.db` and `config.ini` (with the default `123456` user) on every
  invocation, so running them in a directory where the server had never been
  started minted a second, divergent credential store. They now exit with a
  clear error naming the working directory and pointing at the server, which
  remains the only thing that creates the database. `nitr version` likewise no
  longer leaves `nitr.db` and `config.ini` behind as a side effect — it and
  the other informational commands (`-h`, `help`, unknown commands) keep the
  working directory clean. If you scripted `key`/`passwd`/`qr` against an
  empty directory expecting them to provision, start the server there first. ([cd09fb1](https://github.com/bitcav/nitr/commit/cd09fb1), [836a422](https://github.com/bitcav/nitr/commit/836a422))
- **The web panel no longer honours a `remember` cookie.** Login and the auth
  middleware granted access on `Cookie: remember=1`, a value the server never
  set, so anyone who could reach the panel could authenticate by sending it.
  Authentication is now session-only; no legitimate session is affected. See
  **Security** below for the full rationale. ([4db6420](https://github.com/bitcav/nitr/commit/4db6420))

### Added

- Prometheus `/metrics` endpoint, emitting the standard exposition format
  behind the same `x-api-key` header as the JSON API. Exposed series are
  `nitr_cpu_seconds_total{cpu,mode}` (a counter; derive utilisation with
  `rate()` in PromQL), `nitr_ram_{total,free,used}_bytes`, and
  `nitr_disk_{free,size,used}_bytes{mountpoint}` — all `nitr_`-prefixed,
  snake_case, base units. Per-interface bandwidth is deliberately not exposed:
  `bandwidth.Info` sleeps 1s to derive a netdev delta, which would cause
  scrape timeouts; it will land with a background sampler. The README
  documents a working `scrape_config` that passes the key via
  `http_headers.secrets`. ([ac9cdb8](https://github.com/bitcav/nitr/commit/ac9cdb8))
- The test suite now runs on Windows (`windows-2025`) alongside Linux, with a
  per-leg smoke test that builds the real binary and runs `nitr version` on
  each OS and asserts a sane version string. This is the guard that would have
  caught the v0.8.0 `init()`-panic class of bug, where every compiled binary
  died before `main` while `go test` stayed green. ([7584cd2](https://github.com/bitcav/nitr/commit/7584cd2), [dc3c547](https://github.com/bitcav/nitr/commit/dc3c547), [8c1c0cf](https://github.com/bitcav/nitr/commit/8c1c0cf))

### Changed

- The README was overhauled: a 30-second quick start, a platform badge, a
  rewritten intro, and a reorganised table of contents. It now also documents
  the CLI database precondition (`key`/`passwd`/`qr` operate on the database
  in the current working directory and require the server to have run there)
  and states which platforms CI actually verifies on each push. ([b8fc37d](https://github.com/bitcav/nitr/commit/b8fc37d), [d65060a](https://github.com/bitcav/nitr/commit/d65060a))
- The README network table was corrected: `/network` returns an array of
  objects (`[{"ip": ...}]`), not an array of strings as previously documented.
  A JSON example was added, since the nested shape is not expressible in the
  flat key/type table. ([34235f2](https://github.com/bitcav/nitr/commit/34235f2))
- The API field reference moved out of `README.md` into a new `docs/API.md`.
  Those 16 sections previously lived inside collapsed `<details>` blocks, so
  every "JSON Data" link in the endpoint table scrolled to hidden content; the
  table now links across to `docs/API.md`, where the targets are ordinary
  visible headings. No field-reference content changed. ([faab898](https://github.com/bitcav/nitr/commit/faab898))
- The README quick start no longer opens with `sudo` and states plainly that
  Nitr does not need root to run. Privilege notes are now per-field rather than
  per-endpoint: only `serial`/`uuid` on the DMI endpoints require root
  (verified against the kernel's `drivers/firmware/dmi-id.c`), while `/memory`
  requires it wholesale. ([faab898](https://github.com/bitcav/nitr/commit/faab898))
- The README endpoint table now documents `GET /api/v1/`, which was routed but
  previously undocumented. ([faab898](https://github.com/bitcav/nitr/commit/faab898))

### Fixed

- `nitr version` printed no trailing newline, so its output ran into the next
  shell prompt. ([1b7549b](https://github.com/bitcav/nitr/commit/1b7549b))
- `program.Stop` — the service-manager shutdown hook — was an empty stub that
  returned `nil` without stopping anything, so stopping the Nitr service left
  the HTTP server running. It now calls `app.Shutdown()`. ([c1389a0](https://github.com/bitcav/nitr/commit/c1389a0))
- The `/status` websocket reader ran on a hijacked-connection goroutine that
  fiber's `recover` middleware does not cover, so a panic while handling an
  inbound message crashed the whole process. It now recovers explicitly, logs
  the panic, and lets the library release the connection. ([f1a2af8](https://github.com/bitcav/nitr/commit/f1a2af8))
- A startup failure opening `nitr.log` (when `save_logs` is on) called
  `log.Fatalf` from the background goroutine spawned by the service manager,
  `os.Exit`-ing the process from outside `main`'s error-reporting path. The
  error is now returned through `main` and reported cleanly. ([76f329f](https://github.com/bitcav/nitr/commit/76f329f))
- CLI argument routing through the internal dispatch path now forwards its
  arguments to the subcommand parser instead of falling back to `os.Args`.
  The two were equivalent for normal `nitr <command>` use, so no production
  behaviour changed, but dispatch now routes as its signature promised and is
  testable without parsing the test binary's own flags. ([20eb9b9](https://github.com/bitcav/nitr/commit/20eb9b9))
- The password-change success message read "Password changed succesfully!"
  ("succesfully" → "successfully"). ([a2182c7](https://github.com/bitcav/nitr/commit/a2182c7))

### Security

- **Removed a forgeable `remember` cookie that bypassed panel login.** Both
  the login redirect and the auth middleware granted access when
  `c.Cookies("remember") == "1"`, but nothing in the codebase ever set that
  cookie — it was the surviving read half of a "remember me" feature with no
  write path. Anyone who could reach the panel could authenticate by sending
  `Cookie: remember=1`, gaining the panel, host info, and the API key, and
  through it the entire `/api/v1` surface. Both checks are now session-only.
  This is also called out under **⚠ Breaking changes** above. ([4db6420](https://github.com/bitcav/nitr/commit/4db6420))
- Migrated from the end-of-life `gofiber/fiber` v1.11.1 to `fiber/v2`
  (v2.52.14), closing the four outstanding `govulncheck` findings the
  `[0.8.0]` Security entry attributed to EOL fiber v1 and explicitly deferred
  as "tracked separately." That promise is now closed: handler signatures
  return `error`, `Send` → `SendString`, `app.Serve` → `app.Listener`, and
  bundled middleware (logger, recover, session, filesystem) replaces the old
  standalone `gofiber/{embed,recover,logger,session}` modules. Two transitive
  advisories Dependabot flags regardless of reachability were bumped alongside
  it (`golang.org/x/text` and `golang.org/x/sys`). `govulncheck ./...` at this
  commit reports **no vulnerabilities** (the four fiber-v1 findings are gone). ([e79ba5d](https://github.com/bitcav/nitr/commit/e79ba5d))

## [0.8.1] - 2026-07-27

**If you downloaded v0.8.0, replace it.** Every v0.8.0 binary crashed on
startup before reaching `main`, so every command — on every platform —
exited immediately; v0.8.1 is that fix.

### Added

- Quick-install command added to the README. ([925d004](https://github.com/bitcav/nitr/commit/925d004))

### Fixed

- Built binaries panicked in an `init()` before `main` ran, so every command
  crashed on launch. Bumping `go.rice` to `v1.0.3` pulls the fixed
  `go.zipexe v1.0.2` transitively, and a smoke test that builds and runs the
  binary now guards against this shipping green again. ([9a41e4c](https://github.com/bitcav/nitr/commit/9a41e4c))
- `make build` produced no binary at all — the target was missing the `-o`
  flag — so it now writes `nitr` as expected. ([f234a60](https://github.com/bitcav/nitr/commit/f234a60))

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

[Unreleased]: https://github.com/bitcav/nitr/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/bitcav/nitr/releases/tag/v0.9.0
[0.8.1]: https://github.com/bitcav/nitr/releases/tag/v0.8.1
[0.8.0]: https://github.com/bitcav/nitr/releases/tag/v0.8.0
