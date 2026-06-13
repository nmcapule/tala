#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
BIN="$ROOT/.codex/bin/tala-mcp-stdio"

needs_build=0
if [ ! -x "$BIN" ]; then
	needs_build=1
elif find "$ROOT/cmd/tala-mcp-stdio" "$ROOT/internal" -name '*.go' -newer "$BIN" -print -quit | grep -q .; then
	needs_build=1
elif find "$ROOT/go.mod" "$ROOT/go.sum" -newer "$BIN" -print -quit | grep -q .; then
	needs_build=1
fi

if [ "$needs_build" -eq 1 ]; then
	mkdir -p "$ROOT/.codex/bin"
	(cd "$ROOT" && go build -o "$BIN" ./cmd/tala-mcp-stdio)
fi

exec "$BIN" "$@"
