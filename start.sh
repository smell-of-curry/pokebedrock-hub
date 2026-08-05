#!/usr/bin/env bash
#
# start.sh - Download the latest pokebedrock-hub release binary and run it.
#
# Usage:
#   ./start.sh            Download latest release (if newer) and start the server.
#   ./start.sh --force    Re-download the binary even if it is already up to date.
#   ./start.sh --no-loop  Run once and exit instead of auto-restarting on crash.
#
# Linux prod (systemd): use --no-loop and let systemd handle restarts.
# Windows dev host: default loop mode auto-restarts on crash.

set -euo pipefail

FORGE="https://git.pokebedrock.com"
REPO="pokebedrock/pokebedrock-hub"

case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN*)
	ASSET="pokebedrock-hub.exe"
	;;
*)
	ASSET="pokebedrock-hub"
	;;
esac

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

# Forgejo has no /releases/latest/download/<asset> shortcut, so the newest tag
# has to be resolved through the API before the asset can be addressed.
latest_asset_url() {
	curl -fsSL "${FORGE}/api/v1/repos/${REPO}/releases/latest" \
		| tr ',{}' '\n\n\n' \
		| grep -F "\"browser_download_url\"" \
		| sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/' \
		| grep -E "/${ASSET}\$" \
		| head -1
}

download() {
	echo ">> Checking for the latest release of ${ASSET}..."

	local URL
	URL="$(latest_asset_url)"
	if [ -z "$URL" ]; then
		echo ">> Could not resolve a ${ASSET} asset on the latest release." >&2
		[ -f "$ASSET" ] || exit 1
		echo ">> Keeping the existing binary." >&2
		return
	fi

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
	chmod +x "$ASSET"
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
