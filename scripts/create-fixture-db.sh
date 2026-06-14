#!/usr/bin/env bash
set -euo pipefail

db_path="${1:-/tmp/tala-fixture.db}"
addr="${2:-127.0.0.1:18082}"
root="$(pwd)"
db_abs="$(realpath -m "$db_path")"
project_db="$(realpath -m "$root/.tala/tala.db")"

if [[ "$db_abs" == "$project_db" && "${TALA_ALLOW_PROJECT_DB_FIXTURE:-}" != "1" ]]; then
  echo "Refusing to seed the project roadmap database: $db_abs" >&2
  echo "Use a disposable DB path, or set TALA_ALLOW_PROJECT_DB_FIXTURE=1 intentionally." >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" >/dev/null 2>&1; then
    kill "$server_pid" >/dev/null 2>&1 || true
    wait "$server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

mkdir -p "$(dirname "$db_abs")"
rm -f "$db_abs" "$db_abs-shm" "$db_abs-wal"

go run ./cmd/tala -addr "$addr" -db "$db_abs" >"$tmp_dir/server.log" 2>&1 &
server_pid="$!"
base="http://$addr"

for _ in {1..80}; do
  if curl -fsS "$base/healthz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$server_pid" >/dev/null 2>&1; then
    cat "$tmp_dir/server.log" >&2 || true
    exit 1
  fi
  sleep 0.1
done

if ! curl -fsS "$base/healthz" >/dev/null 2>&1; then
  cat "$tmp_dir/server.log" >&2 || true
  echo "Tala fixture server did not become healthy at $base" >&2
  exit 1
fi

cat >"$tmp_dir/seed.mjs" <<'JS'
const base = process.env.TALA_FIXTURE_BASE;
const username = "fixture";

async function api(path, options = {}) {
  const response = await fetch(`${base}${path}`, {
    method: options.method || "GET",
    headers: {
      "Content-Type": "application/json",
      "X-Tala-Username": username,
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
  const data = await response.json();
  if (!response.ok) {
    throw new Error(`${options.method || "GET"} ${path}: ${data.error?.message || response.statusText}`);
  }
  return data;
}

async function createTag(name, color) {
  return api("/api/tags", { method: "POST", body: { name, color } });
}

async function createIssue(input) {
  return api("/api/issues", { method: "POST", body: input });
}

await createTag("fixture", "primary-container");
await createTag("frontend", "secondary-container");
await createTag("backend", "surface-container-highest");
await createTag("release", "tertiary-container");
await createTag("bug", "error-container");

const release = await createIssue({
  title: "Fixture: v1 release readiness",
  description_markdown: "Representative parent issue for release planning and verification.",
  priority: "P1",
  story_points: 3,
  assignee: "fixture-owner",
  tag_names: ["fixture", "release"],
});

const frontend = await createIssue({
  title: "Fixture: polish issue detail workflow",
  description_markdown: "Exercise Markdown, tags, estimates, comments, and parent relationships.",
  priority: "P2",
  story_points: 2,
  assignee: "frontend-agent",
  parent_issue_id: release.id,
  tag_names: ["fixture", "frontend"],
});

const backend = await createIssue({
  title: "Fixture: harden REST and MCP contracts",
  description_markdown: "Representative backend work with blockers and comments.",
  priority: "P2",
  story_points: 3,
  assignee: "backend-agent",
  parent_issue_id: release.id,
  tag_names: ["fixture", "backend"],
});

const blocker = await createIssue({
  title: "Fixture: resolve schema question",
  description_markdown: "Blocking decision used by hierarchy and blocker demos.",
  priority: "P1",
  story_points: 1,
  assignee: "fixture-owner",
  tag_names: ["fixture", "bug"],
});

await api(`/api/issues/${backend.id}/blockers`, {
  method: "POST",
  body: { blocker_issue_id: blocker.id },
});

await api(`/api/issues/${frontend.id}/comments`, {
  method: "POST",
  body: { body_markdown: "Fixture comment with **Markdown** for preview and search coverage." },
});

await api(`/api/issues/${blocker.id}`, {
  method: "PATCH",
  body: { status: "in_progress" },
});

console.log(JSON.stringify({ release, frontend, backend, blocker }, null, 2));
JS

TALA_FIXTURE_BASE="$base" bun "$tmp_dir/seed.mjs" >"$tmp_dir/fixture.json"

echo "fixture db: $db_abs"
echo "fixture url: $base"
bun -e 'const data = require(process.argv[1]); for (const [name, issue] of Object.entries(data)) console.log(`${name}: ${issue.id} ${issue.title}`);' "$tmp_dir/fixture.json"
