# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Given the project is pre-1.0 (`0.x.y`), minor (`0.MINOR.0`) releases may include
behavioral and API changes that would be considered breaking after 1.0; patch
(`0.x.PATCH`) releases are backwards-compatible fixes.

## [Unreleased]

### ⚠ Breaking changes

- **`/api/v1/product` no longer emits `assetTag`; the product name moved to a
  new `name` key.** The value serialized under `assetTag` was always the
  product **name** (`ghw.Product().Name`), never an asset tag — ghw's
  `ProductInfo` has no asset-tag field — so anyone building an asset inventory
  off `/product` was filling the asset-tag column with product names. The key
  is removed outright rather than left permanently empty: an always-`""`
  `assetTag` reads as "this machine has no asset tag", which is worse than the
  key being absent. The machine's real asset tag is already served, correctly,
  at `GET /api/v1/baseboard` (`assetTag`, from `ghw.Baseboard().AssetTag`) —
  read it there instead. The new `name` key carries the product name under its
  own name. Root cause lives in the `github.com/bitcav/nitr-core` dependency
  (v0.1.0, [51c5cc9](https://github.com/bitcav/nitr-core/commit/51c5cc98e13111e3be0bebaa1fad18fb09e63f5d)) and reaches `nitr` via the `go.mod` bump to
  `v0.1.0`.
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
- The server now applies a baseline middleware stack to every response —
  **rate limiting, opt-in CORS, gzip compression, ETags, security headers,
  and per-request IDs** — with three new `config.ini` keys. **Rate
  limiting:** the login `POST /` is capped at `rate_limit_login_max`
  requests per minute per client IP (default 20) against password
  brute-forcing, and `/api/v1/*` plus `/metrics` at `rate_limit_api_max`
  (default 300); over the limit the server returns `429 Too Many Requests`,
  which API pollers should expect and back off from. **CORS is
  deny-by-default:** browser origins get no `Access-Control-Allow-Origin`
  header unless listed in the new `cors_origins` key (comma-separated;
  empty, the default, denies all) — a browser-hosted dashboard on another
  origin needs it set before it can call the API cross-origin. Responses may
  now be gzipped when the client sends `Accept-Encoding: gzip`
  (`compress`), and may answer `304 Not Modified` to conditional
  `If-None-Match` requests (`etag`), so polling the slow-moving endpoints
  skips unchanged bodies. Every response carries the baseline security
  headers (`helmet`) and an `X-Request-Id` unique to the request, which the
  access log records per line when `save_logs` is on — a client-side error
  report can now be matched against the server's log. Verified against the
  running server: with `cors_origins` unset, a request carrying an `Origin`
  header comes back with no `Access-Control-Allow-Origin`; the login POST
  returns `302` for the first `rate_limit_login_max` attempts and `429`
  thereafter; `/api/v1/cpu` returns `429` after 300 requests in a minute;
  and a repeated `If-None-Match` request returns `304`. ([22a5ac8](https://github.com/bitcav/nitr/commit/22a5ac8))
- The panel now shows **live CPU, RAM, and disk usage**, streamed to it over
  the existing `/status` WebSocket. The socket previously carried only
  inbound messages; the server now pushes a metrics frame (host overview plus
  per-disk usage) to every connected panel client — the first frame
  immediately on connect, then one every `metrics_push_interval` seconds (a
  new `config.ini` key, default `3`, clamped to a 1s floor) — and the panel
  renders CPU/RAM/disk widgets from the stream. The route sits behind the
  panel session auth, so the stream is only available to logged-in sessions.
  Each connection's writer goroutine stops when the client disconnects, and
  both the read loop and the writer recover their own panics, so a
  metrics-collection panic cannot take the process down. The WebSocket
  handler also joins the writer goroutine before returning: a metrics tick
  landing after the handler had released the connection wrote to freed
  state, a use-after-free the race detector flagged, so the handler now
  waits for the writer to stop first — and CI runs the suite with `-race`
  to keep it that way. ([c46e4b5](https://github.com/bitcav/nitr/commit/c46e4b5), [752a686](https://github.com/bitcav/nitr/commit/752a686))
- Service lifecycle commands: **`nitr install` / `uninstall` / `start` /
  `stop` / `status`**. The binary could always *run* as a system service, but
  shipped no way to register or control one — installation was a manual
  exercise per platform. The new commands drive the host's service manager
  (systemd, launchd, Windows SCM, via `kardianos/service`) under the service
  name `NitrService`. `status` distinguishes installed-and-running,
  installed-but-stopped, and not-installed; `start`/`stop` against a host
  with no installed unit say to run `nitr install` first; and a failure that
  reads like a permission denial carries a "try running as root or with
  sudo" hint, so "needs root" stays distinguishable from "broken". Verified
  against the built binary: `nitr status` on a bare host prints
  `"NitrService" is not installed on this host (linux-systemd).` and exits 0. ([032fac2](https://github.com/bitcav/nitr/commit/032fac2))

### Changed

- Bumped `github.com/bitcav/nitr-core` from the
  `v0.0.0-20200823224936-5500912f5599` pseudo-version to the tagged **v0.1.0**
  release. This is the dependency bump that delivers the `/product` fixes
  above (correct `family`, deprecated `familiy`, `name`, dropped `assetTag`);
  it is recorded separately because it is the mechanism, not just the symptom.
  No local `replace` directive is involved — `go.mod` resolves the published
  v0.1.0 directly.
- Static assets and views are now embedded with the standard library's
  **`go:embed`** instead of `go.rice`. This removes two dependencies
  (`github.com/GeertJohan/go.rice` and `github.com/daaku/go.zipexe`), deletes
  637 KB of committed generated source (`rice-box.go`), and drops the
  `make rice-box` regeneration step — the embedded copy is compiled from
  `app/assets` and `app/views` at build time, so it can never go stale, and
  there is no generated file left to commit. It also retires the go.zipexe
  runtime-executable-parsing path whose `init()` panic killed every published
  v0.8.0 binary (see [0.8.1]): with no runtime parsing left, that failure
  class is gone rather than patched. Nothing consumer-facing changes — the
  panel and `/assets` are served exactly as before, and `go build` remains
  the only build step. ([207bc2a](https://github.com/bitcav/nitr/commit/207bc2a))

### Fixed

- **`/api/v1/product` now emits `family` (the documented, correctly spelled
  key).** `Product.Family` was serialized under the misspelled JSON tag
  `familiy`, so clients written against the documented `family` key silently
  read an absent field. The misspelled `familiy` key is retained for one
  release as a deprecated duplicate carrying the same value, to avoid a silent
  break, and **will be removed** in a later breaking release — switch to
  `family`. Fixed in the `github.com/bitcav/nitr-core` dependency (v0.1.0,
  [51c5cc9](https://github.com/bitcav/nitr-core/commit/51c5cc98e13111e3be0bebaa1fad18fb09e63f5d)).
- **An unauthenticated `POST /` with a malformed body killed the whole
  server.** `LoginSubmit` — reachable with no credentials — called
  `log.Fatal` when `BodyParser` failed, which `os.Exit(1)`s straight past
  fiber's recover middleware: anyone who could reach the login page could
  terminate the process with a single bad request, a remote denial of
  service. The authenticated `PasswordSubmit` had the same bug. Both now
  return `400 Bad Request`. The same commit fixes a second kill-the-server
  path: `GetLocalIP` called `log.Fatal` when the dial fails, which happens on
  any host with no default route (air-gapped box, restricted container,
  egress-filtered network) — loading the panel on such a host took the
  server down. It now returns an error that surfaces as a `500`. ([d35c6ed](https://github.com/bitcav/nitr/commit/d35c6ed))
- **Concurrent API-key generation raced and could produce duplicate or
  correlated keys.** `utils.RandString` — used for API keys (`POST
  /generate` from the panel) and for the default user's key at first start —
  shared a single `*rand.Rand` across goroutines, and `rand.Rand` is not
  safe for concurrent use: simultaneous generation raced on its internal
  state, and in a racy build the output can repeat or correlate — two keys
  that are equal, or predictable from each other. It now uses the
  `math/rand/v2` package-level source, which is goroutine-safe and
  auto-seeded. ([1f038bf](https://github.com/bitcav/nitr/commit/1f038bf))
- `database.GetUserByID` and `database.SetUserData` no longer panic against a
  `nitr.db` that exists but is missing its `users` bucket (a touched or
  restored-empty file): they return an error naming the database instead of
  dereferencing a nil bucket. `SetAPIData` now re-runs bucket creation
  unconditionally, so such a database self-heals on the next server start
  instead of panicking. ([b8e7193](https://github.com/bitcav/nitr/commit/b8e7193))
- **The five CLI password prompts no longer ignore input errors, and every
  failure path in the credential commands now exits non-zero.** `nitr
  passwd` (three prompts), `nitr key`, and `nitr qr` called `fmt.Scan`
  without checking its return, so a closed or broken stdin left the password
  variable empty and the command silently compared that empty string against
  the stored hash — printing "Wrong password." for what was really a read
  failure. Each prompt now reports the read error and aborts. On top of
  that, the three commands converted from cobra `Run` to `RunE`: **all
  failure paths — a read error, a wrong password, mismatched new passwords,
  a database error — now exit 1**, where previously each printed its message
  and exited 0. Any script wrapping `nitr passwd` / `key` / `qr` can now
  branch on the exit status, and must: a wrong password previously looked
  like success to the shell. Failures are reported as `Error: <message>`
  (lowercased, e.g. `Error: wrong password`, `Error: passwords don't
  match`). Verified against the built binary: `nitr passwd < /dev/null`
  prints `Error: failed to read password: EOF` and exits 1; `nitr key` with
  a wrong password prints `Error: wrong password` and exits 1. ([9517dbc](https://github.com/bitcav/nitr/commit/9517dbc), [a4fdc70](https://github.com/bitcav/nitr/commit/a4fdc70), [5e28eb9](https://github.com/bitcav/nitr/commit/5e28eb9))

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
