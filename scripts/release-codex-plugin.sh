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
mcp_file="$plugin_dir/.mcp.json"
marketplace=".agents/plugins/marketplace.json"

sync_packaged_source() {
  rm -rf "$plugin_dir/cmd" "$plugin_dir/internal"
  cp go.mod go.sum "$plugin_dir/"
  cp -R cmd internal "$plugin_dir/"
}

verify_packaged_source_synced() {
  diff -qr go.mod "$plugin_dir/go.mod" >/dev/null
  diff -qr go.sum "$plugin_dir/go.sum" >/dev/null
  diff -qr cmd "$plugin_dir/cmd" >/dev/null
  diff -qr internal "$plugin_dir/internal" >/dev/null
}

if [[ "$dry_run" -eq 1 ]]; then
  printf 'Dry run: verifying packaged Tala source is current\n'
  verify_packaged_source_synced
else
  printf 'Refreshing packaged Tala source in %s\n' "$plugin_dir"
  sync_packaged_source
fi

python3 - "$manifest" "$mcp_file" "$marketplace" "$plugin_dir" <<'PY'
import json
import sys
from pathlib import Path

manifest_path = Path(sys.argv[1])
mcp_path = Path(sys.argv[2])
marketplace_path = Path(sys.argv[3])
plugin_dir = sys.argv[4]

errors = []
if not manifest_path.is_file():
    errors.append(f"missing plugin manifest: {manifest_path}")
if not mcp_path.is_file():
    errors.append(f"missing plugin MCP file: {mcp_path}")
if not marketplace_path.is_file():
    errors.append(f"missing marketplace file: {marketplace_path}")
if errors:
    for error in errors:
        print(error, file=sys.stderr)
    sys.exit(1)

manifest = json.loads(manifest_path.read_text())
mcp = json.loads(mcp_path.read_text())
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

if manifest.get("skills") != "./skills/":
    errors.append("plugin manifest skills must be './skills/'")
if manifest.get("mcpServers") != "./.mcp.json":
    errors.append("plugin manifest mcpServers must be './.mcp.json'")

servers = mcp.get("mcpServers")
if not isinstance(servers, dict) or "tala" not in servers:
    errors.append("plugin MCP file must define mcpServers.tala")
else:
    tala = servers["tala"]
    args = tala.get("args")
    if tala.get("command") != "sh":
        errors.append("mcpServers.tala.command must be 'sh'")
    if not isinstance(args, list) or len(args) < 3:
        errors.append("mcpServers.tala.args must invoke the plugin-root launcher through sh -c")
    else:
        command = args[1]
        required_refs = [
            "${PLUGIN_ROOT:-}",
            "${TALA_SOURCE_ROOT:-}",
            "$(pwd -P)/plugins/tala-project-tracker",
            "scripts/tala-mcp-stdio.sh",
        ]
        if args[0] != "-c" or any(ref not in command for ref in required_refs):
            errors.append("mcpServers.tala.args must locate the launcher from PLUGIN_ROOT, TALA_SOURCE_ROOT, or the workspace plugin path")

if errors:
    for error in errors:
        print(error, file=sys.stderr)
    sys.exit(1)

print(f"Plugin: {plugin_name}")
print(f"Version: {version}")
print(f"Marketplace: {marketplace_name}")
print(f"Source: {expected_source}")
PY

verify_mcp_startup() {
  local plugin_root="$1"
  local workspace="${2:-$repo_root}"

  python3 - "$plugin_root" "$workspace" <<'PY'
import json
import os
import subprocess
import sys
from pathlib import Path

plugin_root = Path(sys.argv[1]).resolve()
workspace = Path(sys.argv[2]).resolve()
errors = []

required_paths = [
    plugin_root / "go.mod",
    plugin_root / "cmd" / "tala-mcp-stdio",
    plugin_root / "internal" / "mcp",
    plugin_root / "scripts" / "tala-mcp-stdio.sh",
    plugin_root / ".mcp.json",
]
for path in required_paths:
    if not path.exists():
        errors.append(f"missing installed plugin path: {path}")

if errors:
    for error in errors:
        print(error, file=sys.stderr)
    sys.exit(1)

mcp = json.loads((plugin_root / ".mcp.json").read_text())
server = mcp["mcpServers"]["tala"]
command = [server["command"], *server.get("args", []), "--help"]
env = os.environ.copy()
env.pop("PLUGIN_ROOT", None)
env["TALA_SOURCE_ROOT"] = str(plugin_root)
env.setdefault("TALA_WORKSPACE_ROOT", str(workspace))

result = subprocess.run(
    command,
    cwd=workspace,
    env=env,
    text=True,
    capture_output=True,
    timeout=30,
)
output = result.stdout + result.stderr
if result.returncode != 0:
    print(output, file=sys.stderr)
    sys.exit(result.returncode)
if "SQLite database path" not in output:
    print(output, file=sys.stderr)
    print("MCP launcher --help did not look like tala-mcp-stdio output", file=sys.stderr)
    sys.exit(1)

print(f"Verified MCP launcher from {workspace}")
PY
}

printf 'Verifying MCP launcher from /tmp\n'
verify_mcp_startup "$repo_root/$plugin_dir" "/tmp"

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
printf 'Running packaged plugin go test ./...\n'
(cd "$plugin_dir" && go test ./...)

if [[ "$dry_run" -eq 1 ]]; then
  printf 'Dry run: would run codex plugin marketplace add .\n'
  printf 'Dry run: would run codex plugin add tala-project-tracker@tala\n'
else
  codex plugin marketplace add .
  install_output="$(codex plugin add --json tala-project-tracker@tala)"
  printf '%s\n' "$install_output"
  installed_root="$(python3 - "$install_output" "$manifest" <<'PY'
import json
import sys
from pathlib import Path

payload = json.loads(sys.argv[1])
manifest = json.loads(Path(sys.argv[2]).read_text())
version = manifest["version"]

def walk(value):
    if isinstance(value, dict):
        for key, item in value.items():
            if key in {"installedPath", "installedRoot"} and isinstance(item, str):
                return item
            found = walk(item)
            if found:
                return found
    elif isinstance(value, list):
        for item in value:
            found = walk(item)
            if found:
                return found
    return None

root = walk(payload)
if root:
    print(root)
    sys.exit(0)

fallback = Path.home() / ".codex" / "plugins" / "cache" / "tala" / "tala-project-tracker" / version
print(fallback)
PY
)"
  printf 'Verifying installed MCP launcher at %s\n' "$installed_root"
  verify_mcp_startup "$installed_root" "${TALA_RELEASE_VERIFY_WORKSPACE:-/tmp}"
fi

printf 'Release workflow complete. Start a new Codex thread before testing the refreshed plugin.\n'
