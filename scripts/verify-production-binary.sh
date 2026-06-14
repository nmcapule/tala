#!/usr/bin/env bash
set -euo pipefail

addr="${TALA_VERIFY_ADDR:-127.0.0.1:18082}"
base="http://$addr"
tmp_dir="$(mktemp -d)"
pid=""

cleanup() {
  if [[ -n "$pid" ]]; then
    kill "$pid" >/dev/null 2>&1 || true
    wait "$pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

bun run build
go build -o "$tmp_dir/tala" ./cmd/tala

"$tmp_dir/tala" -addr "$addr" -db "$tmp_dir/tala.db" >"$tmp_dir/tala.log" 2>&1 &
pid="$!"

for _ in {1..50}; do
  if curl -fsS "$base/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

curl -fsS "$base/healthz" | grep -q '"ok"'
curl -fsS "$base/" | grep -q '/assets/index.js'
curl -fsS "$base/" | grep -q '/assets/index.css'
curl -fsS "$base/assets/index.js" >/dev/null
curl -fsS "$base/assets/index.css" >/dev/null
curl -fsS "$base/not-a-real-spa-route" | grep -q '/assets/index.js'

missing_asset_code="$(
  curl -sS -o "$tmp_dir/missing-asset.txt" -w '%{http_code}' "$base/assets/not-found.js"
)"
test "$missing_asset_code" = "404"
