#!/usr/bin/env bash
# linux_arm64_endpoint_probe.sh — evidence gathering for the linux/arm64
# ship decision (Raspberry Pi / ARM SBC audience). NOT a pass/fail test of
# endpoint contents: an endpoint that is empty or errors on ARM Linux is a
# FINDING and goes in the report; it must not fail the CI job.
#
# Exit codes:
#   0 — probe ran; report written (endpoint results may be good or bad)
#   1 — genuine failure: binary won't run (init()-panic class), server
#       won't start, or the API key can't be obtained
#
# Lifecycle note: the server holds nitr.db's flock for its whole lifetime,
# and `nitr key` needs the db. So: start server (provisions db + random key)
# -> stop server (releases flock) -> `nitr key` -> start server again -> probe.
#
# The report distinguishes three cases, because they mean different things
# for the ship decision: works / empty because the platform has no SMBIOS
# (structural — expected on SBCs) / actually broken.
#
# KNOWN ARTIFACT, do not wave through: under qemu-user on an x86 host the
# arm64 binary 500s on /cpu (index out of range) — ARM /proc/cpuinfo parsing
# code reading x86-format cpuinfo. That is benign IN EMULATION ONLY. A 500
# from /cpu on the real ubuntu-24.04-arm runner is NOT this artifact — real
# ARM hardware serves ARM-format cpuinfo — so it is a genuine finding and
# blocks shipping until explained.

set -u

BIN="${1:-./nitr_linux_arm64}"
REPORT="${2:-linux-arm64-endpoint-report.md}"
PORT=18765

ENDPOINTS=(/ /cpu /bios /bandwidth /chassis /disks /drives /devices /gpu
  /host /isp /network /processes /ram /baseboard /product /memory /swap
  /loadavg /sensors)

# Endpoints whose data comes from SMBIOS/DMI (/sys/class/dmi/id/*) via
# go-smbios / ghw. Most ARM SBCs (Raspberry Pi included) have no SMBIOS at
# all, so empty here is structural, not a bug.
SMBIOS_EPS="/bios /chassis /baseboard /product /memory "

ROWS=()
VERSION_OUT="(not run)"

add_row() { # ep http result note
  local note="${4//|/\\|}"
  note="${note//$'\n'/ }"
  ROWS+=("| $1 | $2 | $3 | ${note:0:140} |")
}

is_smbios_ep() { [[ "$SMBIOS_EPS" == *"$1 "* ]]; }

write_report() { # verdict
  {
    echo "# nitr linux/arm64 endpoint probe"
    echo
    echo "- Runner: ${RUNNER_OS:-local}/${RUNNER_ARCH:-$(uname -m)} (${RUNNER_NAME:-manual run})"
    echo "- Version string: \`${VERSION_OUT}\`"
    if [[ -d /sys/class/dmi/id ]]; then
      echo "- /sys/class/dmi/id: present (this machine exposes SMBIOS/DMI)"
    else
      echo "- /sys/class/dmi/id: ABSENT (no SMBIOS/DMI — the normal case on ARM SBCs like Raspberry Pi)"
    fi
    echo "- Verdict: $1"
    echo
    echo "SMBIOS-dependent endpoints: /bios /chassis /baseboard /product /memory."
    echo "Empty results there mean 'platform has no SMBIOS', not 'nitr is broken'."
    echo
    if ((${#ROWS[@]} > 0)); then
      echo "| Endpoint | HTTP | Result | Note |"
      echo "|----------|------|--------|------|"
      printf '%s\n' "${ROWS[@]}"
    fi
  } | tee "$REPORT"
}

DATA_DIR="$(mktemp -d /tmp/nitr-arm-probe.XXXXXX)"
SERVER_PID=""
NITR_OPEN_BROWSER_ON_STARTUP=false
export NITR_OPEN_BROWSER_ON_STARTUP

start_server() {
  "$BIN" server --host 127.0.0.1 --port "$PORT" --data-dir "$DATA_DIR" \
    >"$DATA_DIR/server.out.log" 2>"$DATA_DIR/server.err.log" &
  SERVER_PID=$!
}

stop_server() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null
    wait "$SERVER_PID" 2>/dev/null
  fi
  SERVER_PID=""
}

wait_ready() { # seconds
  local deadline=$((SECONDS + $1))
  while ((SECONDS < deadline)); do
    if curl -fsS -m 5 "http://127.0.0.1:$PORT/ready" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

trap stop_server EXIT

# --- 1. Smoke test gate ------------------------------------------------------
# The gate must distinguish HARNESS failure from PLATFORM failure. First run
# of this probe on ubuntu-24.04-arm (2026-07-29) reported "binary does not
# run on linux/arm64 (init()-panic class)" when the truth was that the
# downloaded artifact had no exec bit — a false negative about the platform.
# Each case below gets its own verdict so the report never claims a platform
# conclusion the evidence does not support.
if [[ ! -f "$BIN" ]]; then
  echo "HARNESS FAILURE: $BIN does not exist (artifact download problem?)"
  write_report "HARNESS FAILURE — $BIN not found. Says nothing about the platform."
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  echo "HARNESS FAILURE: $BIN is not executable."
  echo "actions/upload-artifact does not preserve POSIX file modes — chmod +x it after download."
  write_report "HARNESS FAILURE — $BIN not executable (artifact transport drops file modes; chmod +x after download). NOT a platform finding."
  exit 1
fi
VERSION_OUT="$("$BIN" version 2>&1)"
if [[ "$VERSION_OUT" != *"Nitr v"* ]]; then
  echo "SMOKE TEST FAILED: binary does not print a version string. Output was:"
  echo "$VERSION_OUT"
  case "$VERSION_OUT" in
    *"Permission denied"*)
      # Should be unreachable given the -x check above; kept as a second net.
      VERDICT="HARNESS FAILURE — $BIN not executable (artifact transport drops file modes). NOT a platform finding." ;;
    *"exec format error"*|*"Exec format error"*)
      # The kernel rejected the ELF: wrong-architecture binary. This one IS
      # a real finding — the build job produced something arm64 can't exec.
      VERDICT="FAIL — exec format error: the artifact is not a runnable linux/arm64 binary (cross-compile misconfiguration). Genuine platform finding." ;;
    *"cannot execute binary file"*)
      VERDICT="FAIL — shell cannot execute the artifact (likely wrong architecture). Genuine platform finding." ;;
    *)
      VERDICT="FAIL — binary runs but prints no version string (init()-panic class). Endpoint table moot." ;;
  esac
  write_report "$VERDICT"
  exit 1
fi
echo "Smoke test OK: $VERSION_OUT"

# --- 2. Server lifecycle -----------------------------------------------------
start_server
if ! wait_ready 60; then
  write_report "FAIL — server did not become ready within 60s"
  exit 1
fi
echo "Server start #1 OK (db provisioned)"
stop_server # releases the nitr.db flock so `nitr key` can open it

KEY_OUT="$(echo 123456 | "$BIN" key --data-dir "$DATA_DIR" 2>&1)"
if [[ "$KEY_OUT" =~ api\ key\ is:\ *([^[:space:]]+) ]]; then
  API_KEY="${BASH_REMATCH[1]}"
else
  echo "Could not obtain API key. Output was:"
  echo "$KEY_OUT"
  write_report "FAIL — could not obtain API key via \`nitr key\`"
  exit 1
fi
echo "API key obtained"

start_server
if ! wait_ready 60; then
  write_report "FAIL — server did not become ready on second start"
  exit 1
fi
echo "Server start #2 OK, probing endpoints"

# --- 3. Probe (results are data, never failures) -----------------------------
for ep in "${ENDPOINTS[@]}"; do
  url="http://127.0.0.1:$PORT/api/v1$ep"
  body="$(curl -sS -m 30 -w $'\n%{http_code}' -H "x-api-key: $API_KEY" "$url" 2>&1)"
  http="${body##*$'\n'}"
  body="${body%$'\n'*}"

  note=""
  is_smbios_ep "$ep" && note="SMBIOS-dependent"

  if [[ ! "$http" =~ ^[0-9]+$ ]]; then
    add_row "$ep" "—" "request failed" "$body"
    continue
  fi

  trimmed="$(echo "$body" | tr -d '[:space:]')"
  if [[ "$http" != 2* ]]; then
    add_row "$ep" "$http" "HTTP error" "${note:+$note; }$(echo "$body" | head -c 200)"
  elif [[ -z "$trimmed" ]]; then
    add_row "$ep" "$http" "empty body" "$note"
  elif [[ "$trimmed" == "null" || "$trimmed" == "{}" || "$trimmed" == "[]" || "$trimmed" == '""' ]]; then
    add_row "$ep" "$http" "empty ($trimmed)" "$note"
  else
    count="$(printf '%s' "$body" | python3 -c '
import json, sys
try:
    j = json.load(sys.stdin)
    print(len(j))
except Exception:
    sys.exit(1)' 2>/dev/null)" || count=""
    if [[ -z "$count" ]]; then
      add_row "$ep" "$http" "non-JSON body" "${note:+$note; }$(echo "$body" | head -c 200)"
    elif [[ "$count" == "0" ]]; then
      add_row "$ep" "$http" "empty JSON" "$note"
    else
      add_row "$ep" "$http" "populated ($count)" "$note"
    fi
  fi
done

write_report "probe completed — see table; bad cells are findings, not CI failures"
exit 0
