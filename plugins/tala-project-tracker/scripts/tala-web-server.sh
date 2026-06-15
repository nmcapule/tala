#!/usr/bin/env sh
set -eu

canonical_dir() {
	if [ -n "$1" ] && [ -d "$1" ]; then
		CDPATH= cd -- "$1" && pwd -P
		return 0
	fi
	return 1
}

find_tala_source_root() {
	script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)"
	for value in "${TALA_SOURCE_ROOT:-}" "$script_dir" "$(pwd -P)"; do
		if [ -n "$value" ] && [ -d "$value" ]; then
			candidate="$(CDPATH= cd -- "$value" && pwd -P)"
			while [ "$candidate" != "/" ]; do
				if [ -f "$candidate/go.mod" ] && [ -d "$candidate/cmd/tala" ]; then
					printf '%s\n' "$candidate"
					return 0
				fi
				candidate="$(dirname "$candidate")"
			done
		fi
	done
	return 1
}

find_workspace_root() {
	for value in "${TALA_WORKSPACE_ROOT:-}" "${CODEX_WORKSPACE_ROOT:-}" "${CODEX_PROJECT_ROOT:-}" "${WORKSPACE_ROOT:-}" "$(pwd -P)"; do
		if root="$(canonical_dir "$value")"; then
			printf '%s\n' "$root"
			return 0
		fi
	done
	return 1
}

SOURCE_ROOT="$(find_tala_source_root)" || {
	printf '%s\n' "Unable to locate the Tala source checkout. Set TALA_SOURCE_ROOT to the Tala repo root." >&2
	exit 1
}

WORKSPACE_ROOT="$(find_workspace_root)" || {
	printf '%s\n' "Unable to locate a Tala workspace root. Set TALA_WORKSPACE_ROOT to the project root." >&2
	exit 1
}

if [ -z "${TALA_DB:-}" ]; then
	export TALA_DB="$WORKSPACE_ROOT/.tala/tala.db"
fi

if [ -z "${TALA_ADDR:-}" ]; then
	export TALA_ADDR="127.0.0.1:8081"
fi

cd "$SOURCE_ROOT"
exec go run ./cmd/tala "$@"
