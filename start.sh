#!/usr/bin/env bash
#
# start.sh - Download the latest pokebedrock-hub release binary and run it.
#
# Usage:
#   ./start.sh            Download latest release (if newer) and start the server.
#   ./start.sh --force    Re-download the binary even if it is already up to date.
#   ./start.sh --no-loop  Run once and exit instead of auto-restarting on crash.
#
# Works under Git Bash / WSL on the Windows production host. Requires curl.

set -euo pipefail

REPO="smell-of-curry/pokebedrock-hub"
ASSET="pokebedrock-hub.exe"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
ETAG_FILE=".${ASSET}.etag"

FORCE=0
LOOP=1
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    --no-loop) LOOP=0 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

cd "$(dirname "$0")"

download() {
  echo ">> Checking for the latest release of ${ASSET}..."

  # Use the asset's ETag to skip re-downloading when nothing changed.
  local prev_etag=""
  [ -f "$ETAG_FILE" ] && prev_etag="$(cat "$ETAG_FILE")"

  local new_etag
  new_etag="$(curl -fsSLI "$URL" | tr -d '\r' | awk -F': ' 'tolower($1)=="etag"{print $2}')"

  if [ "$FORCE" -eq 0 ] && [ -f "$ASSET" ] && [ -n "$new_etag" ] && [ "$new_etag" = "$prev_etag" ]; then
    echo ">> Already up to date ($new_etag); skipping download."
    return
  fi

  echo ">> Downloading latest ${ASSET}..."
  curl -fL --retry 3 --retry-delay 2 -o "${ASSET}.tmp" "$URL"
  mv -f "${ASSET}.tmp" "$ASSET"
  [ -n "$new_etag" ] && printf '%s' "$new_etag" > "$ETAG_FILE"
  echo ">> Download complete."
}

download

if [ "$LOOP" -eq 0 ]; then
  echo ">> Starting server (single run)..."
  exec ./"$ASSET"
fi

echo ">> Starting server (auto-restart on exit; Ctrl+C twice to stop)..."
while true; do
  ./"$ASSET" || true
  echo ">> Server exited. Restarting in 5s... (press Ctrl+C to abort)"
  sleep 5
done
