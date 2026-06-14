#!/usr/bin/env sh
set -eu

find_workspace_root() {
	for value in "${TALA_WORKSPACE_ROOT:-}" "${CODEX_WORKSPACE_ROOT:-}" "${CODEX_PROJECT_ROOT:-}" "${WORKSPACE_ROOT:-}" "$(pwd -P)"; do
		if [ -n "$value" ] && [ -d "$value" ]; then
			candidate="$(CDPATH= cd -- "$value" && pwd -P)"
			while [ "$candidate" != "/" ]; do
				if [ -f "$candidate/go.mod" ] && [ -d "$candidate/cmd/tala-mcp-stdio" ]; then
					printf '%s\n' "$candidate"
					return 0
				fi
				candidate="$(dirname "$candidate")"
			done
		fi
	done
	return 1
}

ROOT="$(find_workspace_root)" || {
	printf '%s\n' "Unable to locate a Tala workspace root. Set TALA_WORKSPACE_ROOT to the repo root." >&2
	exit 1
}

if [ -z "${TALA_DB:-}" ]; then
	export TALA_DB="$ROOT/tala.db"
fi

cd "$ROOT"
exec go run ./cmd/tala-mcp-stdio "$@"
