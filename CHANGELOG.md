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

- **Configuration via CLI flags, `NITR_*` environment variables, and a config
  file**, with a documented precedence: **`--flags` > `NITR_*` env > config
  file > built-in defaults.** Four new persistent flags — `--config`,
  `--port`, `--host` (alias `--bind`), and `--data-dir` — bind to the same
  keys the config file reads, and `viper.SetEnvPrefix("NITR")` plus
  `AutomaticEnv` means **every** config key (not just the four with flags —
  `save_logs`, `cors_origins`, `metrics_push_interval`, …) takes a `NITR_`
  uppercase env override, which is what makes the Docker image configurable
  without bind-mounting a config file. `--host 127.0.0.1` makes
  **localhost-only binding possible for the first time** — previously the
  server always listened on all interfaces; the default remains `0.0.0.0`
  (all interfaces), so behaviour without configuration is unchanged and the
  security-relevant default did not change. `--data-dir` relocates `nitr.db`
  (the directory is created if missing); `nitr.log` follows `--data-dir`
  when `save_logs` is on. `config.ini` is not affected and stays in the
  working directory. The config file is named
  `config.ini` but is **parsed as YAML** — write `key: value`, not INI
  `key=value` or `[sections]` — and keeps its name to avoid breaking existing
  installs. Verified against the built binary: `--port`, `--host`,
  `--data-dir`, `NITR_PORT`, and `--config <path>` all behave as documented,
  and a flag beats the matching env var which beats the file value which
  beats the default. ([73029b8](https://github.com/bitcav/nitr/commit/73029b8))
- Liveness and readiness probes at `GET /health` and `GET /ready`, plus a
  Docker `HEALTHCHECK` that polls `/health`. Both routes are registered on the
  root app before the `x-api-key` middleware and the panel session auth, so
  they answer with **no credentials** and never redirect to the login page —
  the point being that Docker / Compose / Kubernetes / uptime checkers can
  poll them unauthenticated. `/health` does no I/O and returns
  `200 {"status":"ok","version":"0.9.0"}`. `/ready` does an
  `os.Stat(database.DBPath())` — the resolved `nitr.db` path, honouring
  `--data-dir` — and nothing more: `200 {"status":"ready"}` when the
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
- **`nitr server`** — an explicit subcommand for starting the host service.
  Bare `nitr` still starts it too and is **not deprecated**: the Dockerfile
  `CMD ["./nitr"]` and the README Windows double-click flow both depend on
  the zero-argument default, which is unchanged. `nitr server` exists so the
  spelling is self-documenting — it is what `nitr --help` lists, what the
  installed system service's `ExecStart` invokes (`ServiceConfig.Arguments`
  is now `["server"]`), and what reads clearly in a process list. It
  inherits the root command's persistent flags (`nitr server --port 9000`
  works). Verified against the built binary: `nitr --help` lists `server`,
  and both `nitr` and `nitr server` reach the server entry point. ([96e3361](https://github.com/bitcav/nitr/commit/96e3361))
- **Admin panel redesign** at `/`: **dark mode** following the OS
  `prefers-color-scheme` with a manual toggle in the header (the choice
  persists in `localStorage` and overrides the OS in both directions);
  **tabbed navigation** split into Overview and Metrics where the page
  previously had a single view; **live CPU and RAM line charts** on the
  Metrics tab, fed from the existing `/status` WebSocket stream; and a
  **mobile-usable layout** that reflows to a single column with pinch-zoom
  re-enabled (the viewport meta no longer pins `maximum-scale=1.0`). The
  charts are **session-only browser history** — roughly the last six
  minutes (120 samples at the default 3s push interval), held in the
  browser, **nothing persisted server-side** — and start empty on each
  page load; historical retention is a separate, unbuilt feature. The
  uPlot chart library (~50 KB) is vendored into the binary, so no CDN is
  used and no build step was added — the panel still works air-gapped.
  Verified by logging into the running panel: the Overview/Metrics tabs,
  the theme toggle (with `nitr-theme` in `localStorage`), and the
  `cpuChart`/`ramChart` containers all render. ([ab5c032](https://github.com/bitcav/nitr/commit/ab5c032))
- **CLI info commands that read the collectors directly — no server, no
  API key, no network** — one subcommand per API path segment: `nitr
  cpu`, `ram`, `memory`, `disks`, `drives`, `bios`, `chassis`,
  `baseboard`, `product`, `gpu`, `network`, `bandwidth`, `isp`,
  `processes`, `devices`, `host`, and `overview`. Each prints an aligned
  key/value table by default; `--json` prints the raw JSON payload
  byte-identical to the matching API response (the same
  `encoding/json.Marshal` path fiber's `c.JSON` uses), and `--watch` /
  `-w` re-fetches and re-renders on an interval (`--watch=5s`; a bare
  `--watch` defaults to 2s). Until now the only way to read a metric was
  to start the server, authenticate, and make an HTTP request, even
  though every collector is a plain Go function returning a struct; this
  makes nitr usable standalone, in the neofetch/inxi/hwinfo niche.
  Verified against the built binary: `nitr cpu`, `nitr ram`, and `nitr
  disks` print their tables with no server running, `nitr cpu --json`
  emits the `{"vendor":...,"cores":12,...}` payload, and `nitr cpu
  --watch=1s` re-renders the table each second. ([a54b802](https://github.com/bitcav/nitr/commit/a54b802))
- **An OpenAPI 3.1 spec is now the API reference source of truth**,
  checked in at `docs/openapi.json` and served two ways: raw at `GET
  /openapi.json` (no credentials required) and rendered at `GET /docs`
  (behind the panel session auth), which fetches `/openapi.json`
  client-side so the rendered docs can never drift from the spec. It
  covers all 17 `GET /api/v1/*` endpoints with schemas taken from the
  actual Go structs, the `x-api-key` security scheme, and the privilege
  and deprecation notes that used to live in `docs/API.md`'s prose —
  which is removed; its hand-maintained schema tables had already
  drifted from the real `/processes` shape. A test
  (`TestOpenAPISpecCoversAllRegisteredRoutes`) fails the build if a
  registered route is missing from the spec. Verified against the
  running server: `/openapi.json` returns `200 application/json` with
  `"openapi": "3.1.0"` and a path entry for every v1 route including
  `/loadavg`, `/swap`, and `/sensors`, and `/docs` redirects
  unauthenticated clients to the login page. ([9ae8e37](https://github.com/bitcav/nitr/commit/9ae8e37))
- Three new collectors behind the same `x-api-key` auth:
  **`GET /api/v1/loadavg`** returns the 1/5/15-minute load average
  (`{"load1":0.19,"load5":0.15,"load15":0.23}`); it is not implemented
  on Windows (gopsutil has no equivalent concept there) and surfaces
  that as `501 Not Implemented` rather than a silently empty `200`.
  **`GET /api/v1/swap`** returns swap total/used/free/used-percent plus
  page-in/out counters — `/ram` only reports physical memory, so a host
  swapping heavily previously looked healthy. **`GET /api/v1/sensors`**
  returns temperature/fan sensor readings; a host with no exposed
  sensors is not an error and returns `null`, matching the other list
  endpoints' behaviour on an empty result. Verified against the running
  server on Linux: `/loadavg` and `/swap` return the shapes above;
  `/sensors` returns `null` on the machine tested (a WSL guest with no
  hwmon sensors exposed). ([71b8382](https://github.com/bitcav/nitr/commit/71b8382))
- **`install.sh` curl-pipe quick start** in the README:
  `curl -fsSL https://raw.githubusercontent.com/bitcav/nitr/master/install.sh | bash`
  downloads the right release binary for the host OS/arch into the
  current directory, sets the exec bit, and runs `nitr version` to
  confirm it works — modeled on direnv's installer, with no root or sudo
  step anywhere (the README's previous `sudo install` step is gone). ([4935256](https://github.com/bitcav/nitr/commit/4935256))

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
- **`GET /api/v1/processes` returns a much richer — and purely additive —
  response shape, and gains query parameters.** Each entry was
  `{pid, name}` and is now `{pid, ppid, name, user, cmdline, status,
  cpu_percent, mem_percent, rss, start_time}`; no keys were removed, so
  clients reading the old two keys are unaffected. New query parameters:
  `?sort=cpu|mem|name|pid` (default `pid`), `?order=asc|desc` (default
  `asc`), `?limit=<n>`, and `?search=<substring>` matched
  case-insensitively against name and cmdline. A process that exits
  mid-scan is skipped rather than failing the whole call. Verified
  against the running server: `?sort=cpu&limit=3` returns three objects
  carrying all ten fields, and `?search=nitr` filters the list to
  matching processes. ([44d4710](https://github.com/bitcav/nitr/commit/44d4710))
- **`/api/v1/bandwidth` and `/api/v1/isp` no longer block the request.**
  `/bandwidth` previously stalled every caller for ~1s while
  `bandwidth.Info()` computed its rx/tx delta inline; a background
  sampler now refreshes a cache every 5s and the handler serves the
  cache immediately — until the first sample lands it returns `null`.
  `/isp` previously made an outbound speedtest.net call with no timeout,
  hanging indefinitely on a slow or air-gapped host; it now caches a
  successful lookup for an hour and races a fresh lookup against a 5s
  timeout, falling back to the last good (or empty) cached value.
  Verified against the running server: the first `/bandwidth` call
  returns `null` in ~6ms (previously ~1s), and `/isp` answers in
  ~270ms with `{"isp":...,"ip":...,"lat":...,"lon":...}`. ([d7fe474](https://github.com/bitcav/nitr/commit/d7fe474))
- The README screenshots are now reproducible: `scripts/regen-images.sh`
  re-captures them via `scripts/web-screenshots.mjs` and two committed
  vhs tapes, replacing manual screenshotting. Contributor tooling; no
  shipped behaviour changes. ([2eb84fb](https://github.com/bitcav/nitr/commit/2eb84fb))
- Internal: the database layer now reuses a single bbolt handle instead
  of opening and closing the file on every call (`AuthAPI` alone opened
  it twice per authenticated request), and a second nitr process against
  the same `nitr.db` now fails startup after a 5s lock wait with
  `database is locked by another nitr process` instead of blocking
  forever — observed in practice during verification. `SetAPIData`
  propagates setup failures instead of logging and swallowing them, so a
  broken database fails startup loudly. Plus an idiom sweep (`%w` error
  wrapping, unified `fiber.Status*` constants) and lint/gofmt fixes
  keeping CI green. ([7c118c9](https://github.com/bitcav/nitr/commit/7c118c9), [1a35322](https://github.com/bitcav/nitr/commit/1a35322), [4a71ecb](https://github.com/bitcav/nitr/commit/4a71ecb), [385a73e](https://github.com/bitcav/nitr/commit/385a73e), [ec41333](https://github.com/bitcav/nitr/commit/ec41333))

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
