#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release-codex-plugin.sh [--no-bump] [--dry-run]

Releases the repo-local Tala Codex plugin from the current checkout.

Options:
  --no-bump  Keep the current plugin version.
  --dry-run  Validate and test without editing plugin.json or reinstalling.
USAGE
}

dry_run=0
bump=1

for arg in "$@"; do
  case "$arg" in
    --dry-run)
      dry_run=1
      ;;
    --no-bump)
      bump=0
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

require_cmd git
require_cmd python3
require_cmd go
if [[ "$dry_run" -eq 0 ]]; then
  require_cmd codex
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

plugin_dir="plugins/tala-project-tracker"
manifest="$plugin_dir/.codex-plugin/plugin.json"
marketplace=".agents/plugins/marketplace.json"

python3 - "$manifest" "$marketplace" "$plugin_dir" <<'PY'
import json
import sys
from pathlib import Path

manifest_path = Path(sys.argv[1])
marketplace_path = Path(sys.argv[2])
plugin_dir = sys.argv[3]

errors = []
if not manifest_path.is_file():
    errors.append(f"missing plugin manifest: {manifest_path}")
if not marketplace_path.is_file():
    errors.append(f"missing marketplace file: {marketplace_path}")
if errors:
    for error in errors:
        print(error, file=sys.stderr)
    sys.exit(1)

manifest = json.loads(manifest_path.read_text())
marketplace = json.loads(marketplace_path.read_text())

plugin_name = manifest.get("name")
version = manifest.get("version")
marketplace_name = marketplace.get("name")
expected_source = f"./{plugin_dir}"

if plugin_name != "tala-project-tracker":
    errors.append(f"unexpected plugin name: {plugin_name!r}")
if not isinstance(version, str) or not version.strip():
    errors.append("plugin version must be a non-empty string")
if marketplace_name != "tala":
    errors.append(f"unexpected marketplace name: {marketplace_name!r}")

entries = marketplace.get("plugins")
if not isinstance(entries, list):
    errors.append("marketplace plugins must be a list")
else:
    matches = [entry for entry in entries if entry.get("name") == plugin_name]
    if len(matches) != 1:
        errors.append(f"expected exactly one marketplace entry for {plugin_name!r}")
    else:
        source_path = matches[0].get("source", {}).get("path")
        if source_path != expected_source:
            errors.append(f"marketplace source.path must be {expected_source!r}, got {source_path!r}")

for required in ["skills", "mcpServers"]:
    value = manifest.get(required)
    if not isinstance(value, str) or not value.strip():
        errors.append(f"plugin manifest field {required!r} must be a non-empty string")

if errors:
    for error in errors:
        print(error, file=sys.stderr)
    sys.exit(1)

print(f"Plugin: {plugin_name}")
print(f"Version: {version}")
print(f"Marketplace: {marketplace_name}")
print(f"Source: {expected_source}")
PY

if [[ "$bump" -eq 1 ]]; then
  if [[ "$dry_run" -eq 1 ]]; then
    stamp="$(python3 - <<'PY'
from datetime import datetime, timezone
print(datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S"))
PY
)"
    printf 'Dry run: would update %s to +codex.%s\n' "$manifest" "$stamp"
  else
    python3 - "$manifest" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

path = Path(sys.argv[1])
data = json.loads(path.read_text())
old_version = data["version"]
base = old_version.split("+", 1)[0]
stamp = datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S")
new_version = f"{base}+codex.{stamp}"
data["version"] = new_version
path.write_text(json.dumps(data, indent=2) + "\n")
print(f"Updated {path}: {old_version} -> {new_version}")
PY
  fi
else
  printf 'Keeping current plugin version in %s\n' "$manifest"
fi

printf 'Running go test ./...\n'
go test ./...

if [[ "$dry_run" -eq 1 ]]; then
  printf 'Dry run: would run codex plugin marketplace add .\n'
  printf 'Dry run: would run codex plugin add tala-project-tracker@tala\n'
else
  codex plugin marketplace add .
  codex plugin add tala-project-tracker@tala
fi

printf 'Release workflow complete. Start a new Codex thread before testing the refreshed plugin.\n'
