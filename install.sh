#!/usr/bin/env bash
#
# Downloads the right nitr binary for this machine into the current
# directory, sets the exec bit, and runs `nitr version` to confirm it works.
#
# Modeled on direnv's install.sh (github.com/direnv/direnv/blob/master/install.sh):
# same shape of problem -- a single prebuilt Go binary fetched straight from
# GitHub releases, no checksums published to verify against.
#
# Usage: curl -fsSL https://raw.githubusercontent.com/bitcav/nitr/master/install.sh | bash
#    or: bash install.sh [output-name]
set -euo pipefail

{ # Prevent execution if this script was only partially downloaded

  log() {
    echo "[installer] $*" >&2
  }

  die() {
    log "$@"
    exit 1
  }

  at_exit() {
    ret=$?
    if [[ $ret -gt 0 ]]; then
      log "the script failed with error $ret." \
        "To report installation errors, open an issue at" \
        "    https://github.com/bitcav/nitr/issues/new"
    fi
    exit "$ret"
  }
  trap at_exit EXIT

  dest="${1:-nitr}"

  # A stale directory of the same name is the #1 support question here: `curl
  # -o nitr` fails with a cryptic "Is a directory" write error instead of a
  # clear one, so catch it up front.
  if [[ -d "$dest" ]]; then
    die "'$dest' is a directory, not a file -- refusing to overwrite it." \
      "Run this from an empty directory, or pass a different name: bash install.sh mybinary"
  fi

  kernel=$(uname -s)
  case "$kernel" in
    Linux) os_tag=linux ;;
    *)
      die "unsupported OS '$kernel' -- prebuilt binaries are Linux and Windows only." \
        "Windows: use Invoke-WebRequest, see README.md#quick-install"
      ;;
  esac

  machine=$(uname -m)
  case "$machine" in
    x86_64 | amd64) arch_tag=amd64 ;;
    i386 | i686) arch_tag=386 ;;
    *)
      die "unsupported architecture '$machine' -- nitr ships linux_amd64 and linux_386 only."
      ;;
  esac
  log "kernel=$kernel machine=$machine -> ${os_tag}_${arch_tag}"

  asset="nitr_${os_tag}_${arch_tag}"
  url="https://github.com/bitcav/nitr/releases/latest/download/${asset}"

  log "downloading ${asset}"
  # -f: fail on HTTP errors instead of saving the error page as the binary.
  curl -fL "$url" -o "$dest"
  chmod +x "$dest"

  case "$dest" in
    */*) run_path="$dest" ;;
    *) run_path="./$dest" ;;
  esac

  dest_abs="$(cd -- "$(dirname -- "$dest")" && pwd)/$(basename -- "$dest")"

  log "downloaded $dest_abs"
  log "nothing was installed system-wide -- $dest_abs is NOT on your PATH." \
      "Run it as '$run_path' from here, or make it the system nitr with:" \
      "    sudo mv $dest_abs /usr/local/bin/nitr"

  # Shadowing check: an older nitr already on PATH keeps winning over the
  # file just downloaded, so name it explicitly rather than letting the user
  # discover the mismatch one command later.
  existing=$(command -v nitr || true)
  if [[ -n "$existing" && "$existing" != "$dest_abs" ]]; then
    existing_version=$("$existing" version 2>/dev/null || echo "version unknown")
    log "WARNING: 'nitr' on your PATH resolves to $existing ($existing_version) --" \
        "running 'nitr' will keep using THAT binary, not the one just downloaded." \
        "To switch: sudo mv $dest_abs /usr/local/bin/nitr"
  fi

  log "version of the downloaded binary ($dest_abs):"
  "$run_path" version
}
