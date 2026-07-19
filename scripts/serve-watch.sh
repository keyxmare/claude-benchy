#!/usr/bin/env bash
# Development dashboard with auto-reload: rebuild and restart `benchy serve`
# whenever a Go source or an embedded template/asset changes. Templates and
# static files are compiled into the binary (go:embed), so every UI change
# needs a rebuild — this loop makes that automatic.
#
# The build goes through Docker (make build); the server runs natively, exactly
# as `make serve` does (it reads the host Keychain and launches the sandbox
# containers). Extra arguments are forwarded to `benchy serve`, e.g.:
#   scripts/serve-watch.sh --addr 127.0.0.1:8080
set -euo pipefail

cd "$(dirname "$0")/.."

BINARY="${BINARY:-./benchy}"
WATCH_PATHS=(cmd internal go.mod go.sum)
MARKER="$(mktemp)"
SERVER_PID=""

stop_server() {
	if [ -n "$SERVER_PID" ]; then
		kill "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
		SERVER_PID=""
	fi
}

cleanup() {
	stop_server
	rm -f "$MARKER"
}
trap 'printf "\n↩ arrêt\n"; cleanup; exit 0' INT TERM

# changed reports (exit 0) when a watched source is newer than the last build.
changed() {
	[ -n "$(find "${WATCH_PATHS[@]}" -type f \
		\( -name '*.go' -o -name '*.html' -o -name '*.css' -o -name 'go.mod' -o -name 'go.sum' \) \
		-newer "$MARKER" 2>/dev/null)" ]
}

rebuild_and_restart() {
	touch "$MARKER"
	printf '▶ build…\n'
	if ! make build; then
		printf '✗ build en échec — serveur inchangé, en attente de correction\n'
		return
	fi
	stop_server
	printf '✓ démarrage : %s serve %s\n' "$BINARY" "$*"
	"$BINARY" serve "$@" &
	SERVER_PID=$!
}

printf 'serve-watch — rebuild + relance à chaque modif (Ctrl-C pour arrêter)\n'
printf 'astuce : si le port est pris (conteneur actif), « make down » ou --addr 127.0.0.1:8080\n'

rebuild_and_restart "$@"
while true; do
	if changed; then
		printf '↻ modification détectée\n'
		rebuild_and_restart "$@"
	fi
	sleep 1
done
