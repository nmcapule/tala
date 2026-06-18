#!/usr/bin/env bash
set -euo pipefail

base="${1:-http://127.0.0.1:8081}"
creator="smoke_creator_$$"

json_field() {
  bun -e "let s=''; process.stdin.on('data', d => s += d); process.stdin.on('end', () => console.log(JSON.parse(s)$1 ?? ''))"
}

curl -fsS "$base/healthz" >/dev/null

api_missing_code="$(
  curl -sS -o /tmp/tala-smoke-api-missing.json -w '%{http_code}' \
    "$base/api/not-a-route"
)"
test "$api_missing_code" = "404"
bun -e 'let d=require("/tmp/tala-smoke-api-missing.json"); if(d.error?.code !== "not_found") process.exit(1)'

missing_code="$(
  curl -sS -o /tmp/tala-smoke-missing.json -w '%{http_code}' \
    -X POST "$base/api/issues" \
    -H 'Content-Type: application/json' \
    -d '{"title":"Missing user","priority":"P2"}'
)"
test "$missing_code" = "401"

null_body_code="$(
  curl -sS -o /tmp/tala-smoke-null-body.json -w '%{http_code}' \
    -X POST "$base/api/issues" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d 'null'
)"
test "$null_body_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-null-body.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "body") process.exit(1)'

trailing_body_code="$(
  curl -sS -o /tmp/tala-smoke-trailing-body.json -w '%{http_code}' \
    -X POST "$base/api/issues" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d '{"title":"Trailing body","priority":"P2"} {}'
)"
test "$trailing_body_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-trailing-body.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "body") process.exit(1)'

invalid_color_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-tag-color.json -w '%{http_code}' \
    -X POST "$base/api/tags" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d '{"name":"invalid color smoke","color":"not-a-color"}'
)"
test "$invalid_color_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-tag-color.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "color") process.exit(1)'

tag_string_body_code="$(
  curl -sS -o /tmp/tala-smoke-tag-string-body.json -w '%{http_code}' \
    -X POST "$base/api/tags" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d '"tag"'
)"
test "$tag_string_body_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-tag-string-body.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "body") process.exit(1)'

token_tag="$(
  curl -fsS -X POST "$base/api/tags" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d "{\"name\":\"token color smoke $$\",\"color\":\"secondary-container\"}"
)"
printf '%s' "$token_tag" | bun -e 'let s=""; process.stdin.on("data", d => s += d); process.stdin.on("end", () => { let d=JSON.parse(s); if(d.color !== "secondary-container") process.exit(1); })'
token_tag_id="$(printf '%s' "$token_tag" | json_field '.id')"

curl -fsS "$base/api/tags" >/tmp/tala-smoke-tags.json
bun -e 'let d=require("/tmp/tala-smoke-tags.json"); if(!d.some(t => t.id === process.argv[1] && t.color === "secondary-container")) process.exit(1)' "$token_tag_id"

curl -fsS -X PATCH "$base/api/tags/$token_tag_id" \
  -H 'Content-Type: application/json' \
  -H "X-Tala-Username: $creator" \
  -d "{\"name\":\"token color smoke renamed $$\",\"color\":null}" >/tmp/tala-smoke-tag-update.json
bun -e 'let d=require("/tmp/tala-smoke-tag-update.json"); if(d.name !== process.argv[1] || d.color !== null) process.exit(1)' "token color smoke renamed $$"

issue="$(
  curl -fsS -X POST "$base/api/issues" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d '{"title":"Smoke issue","description_markdown":"Smoke **markdown**","priority":"P2","assignee":"sam","tag_names":["smoke","api"]}'
)"
issue_id="$(printf '%s' "$issue" | json_field '.id')"

unsupported_issue_delete_code="$(
  curl -sS -o /tmp/tala-smoke-unsupported-issue-delete.json -w '%{http_code}' \
    -X DELETE "$base/api/issues/$issue_id"
)"
test "$unsupported_issue_delete_code" = "405"
bun -e 'let d=require("/tmp/tala-smoke-unsupported-issue-delete.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "method") process.exit(1)'

unsupported_comment_patch_code="$(
  curl -sS -o /tmp/tala-smoke-unsupported-comment-patch.json -w '%{http_code}' \
    -X PATCH "$base/api/issues/$issue_id/comments/comment_missing" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $creator" \
    -d '{"body_markdown":"comments are append-only"}'
)"
test "$unsupported_comment_patch_code" = "404"
bun -e 'let d=require("/tmp/tala-smoke-unsupported-comment-patch.json"); if(d.error?.code !== "not_found" || d.error?.field !== "path") process.exit(1)'

parent="$(
  curl -fsS -X POST "$base/api/issues" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{"title":"Smoke parent","priority":"P3","tag_names":["smoke"]}'
)"
parent_id="$(printf '%s' "$parent" | json_field '.id')"

invalid_status_filter_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-status-filter.json -w '%{http_code}' \
    "$base/api/issues?status=shipped"
)"
test "$invalid_status_filter_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-status-filter.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "status") process.exit(1)'

invalid_priority_filter_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-priority-filter.json -w '%{http_code}' \
    "$base/api/issues?priority=P9"
)"
test "$invalid_priority_filter_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-priority-filter.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "priority") process.exit(1)'

invalid_state_filter_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-state-filter.json -w '%{http_code}' \
    "$base/api/issues?state=waiting"
)"
test "$invalid_state_filter_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-state-filter.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "state") process.exit(1)'

invalid_sort_filter_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-sort-filter.json -w '%{http_code}' \
    "$base/api/issues?sort=rank"
)"
test "$invalid_sort_filter_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-sort-filter.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "sort") process.exit(1)'

invalid_order_filter_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-order-filter.json -w '%{http_code}' \
    "$base/api/issues?order=reverse"
)"
test "$invalid_order_filter_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-order-filter.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "order") process.exit(1)'

blocker="$(
  curl -fsS -X POST "$base/api/issues" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{"title":"Smoke blocker","priority":"P1","tag_names":["smoke"]}'
)"
blocker_id="$(printf '%s' "$blocker" | json_field '.id')"

curl -fsS -X POST "$base/api/issues/$issue_id/blockers" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d "{\"blocker_issue_id\":\"$blocker_id\"}" >/dev/null

curl -fsS -X PUT "$base/api/issues/$issue_id/parent" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d "{\"parent_issue_id\":\"$parent_id\"}" >/tmp/tala-smoke-set-parent.json
bun -e 'let d=require("/tmp/tala-smoke-set-parent.json"); if(d.parent_issue_id !== process.argv[1]) process.exit(1)' "$parent_id"

invalid_parent_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-parent.json -w '%{http_code}' \
    -X PUT "$base/api/issues/$issue_id/parent" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{"parent_issue_id":42}'
)"
test "$invalid_parent_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-parent.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "parent_issue_id") process.exit(1)'

null_blocker_code="$(
  curl -sS -o /tmp/tala-smoke-null-blocker.json -w '%{http_code}' \
    -X POST "$base/api/issues/$issue_id/blockers" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{"blocker_issue_id":null}'
)"
test "$null_blocker_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-null-blocker.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "blocker_issue_id") process.exit(1)'

curl -fsS -X POST "$base/api/issues/$issue_id/comments" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d '{"body_markdown":"Smoke comment."}' >/dev/null

curl -fsS "$base/api/issues/$issue_id/comments" >/tmp/tala-smoke-comments.json
bun -e 'let d=require("/tmp/tala-smoke-comments.json"); if(d.length !== 1 || d[0].body_markdown !== "Smoke comment.") process.exit(1)'

missing_comments_code="$(
  curl -sS -o /tmp/tala-smoke-missing-comments.json -w '%{http_code}' \
    "$base/api/issues/issue_missing/comments"
)"
test "$missing_comments_code" = "404"
bun -e 'let d=require("/tmp/tala-smoke-missing-comments.json"); if(d.error?.code !== "not_found" || d.error?.field !== "issue_id") process.exit(1)'

null_comment_code="$(
  curl -sS -o /tmp/tala-smoke-null-comment.json -w '%{http_code}' \
    -X POST "$base/api/issues/$issue_id/comments" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{"body_markdown":null}'
)"
test "$null_comment_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-null-comment.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "body_markdown") process.exit(1)'

invalid_comment_code="$(
  curl -sS -o /tmp/tala-smoke-invalid-comment.json -w '%{http_code}' \
    -X POST "$base/api/issues/$issue_id/comments" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{"body_markdown":42}'
)"
test "$invalid_comment_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-invalid-comment.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "body_markdown") process.exit(1)'

missing_comment_body_code="$(
  curl -sS -o /tmp/tala-smoke-missing-comment-body.json -w '%{http_code}' \
    -X POST "$base/api/issues/$issue_id/comments" \
    -H 'Content-Type: application/json' \
    -H 'X-Tala-Username: smoke' \
    -d '{}'
)"
test "$missing_comment_body_code" = "400"
bun -e 'let d=require("/tmp/tala-smoke-missing-comment-body.json"); if(d.error?.code !== "validation_error" || d.error?.field !== "body_markdown") process.exit(1)'

curl -fsS "$base/api/issues/$issue_id" >/tmp/tala-smoke-detail.json
bun -e 'let d=require("/tmp/tala-smoke-detail.json"); if(!d.blocked || d.blockers.length !== 1 || d.recent_comments.length !== 1) process.exit(1)'

curl -fsS -X PATCH "$base/api/issues/$issue_id" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d '{"status":"in_progress"}' >/dev/null

curl -fsS "$base/api/issues/$issue_id" >/tmp/tala-smoke-before-noop-update.json
curl -fsS -X PATCH "$base/api/issues/$issue_id" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d '{}' >/tmp/tala-smoke-noop-update.json
bun -e 'let before=require("/tmp/tala-smoke-before-noop-update.json"); let after=require("/tmp/tala-smoke-noop-update.json"); if(after.id !== before.id || after.updated_at !== before.updated_at) process.exit(1)'

curl -fsS -X PATCH "$base/api/issues/$issue_id" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d '{"title":" Smoke issue ","status":" in_progress ","priority":" P2 ","assignee":" sam ","tag_names":[" API ","smoke","api"," "]}' >/tmp/tala-smoke-normalized-noop-update.json
bun -e 'let before=require("/tmp/tala-smoke-noop-update.json"); let after=require("/tmp/tala-smoke-normalized-noop-update.json"); if(after.id !== before.id || after.updated_at !== before.updated_at) process.exit(1); let tags=(after.tags||[]).map(t => t.name).sort().join(","); if(tags !== "api,smoke") process.exit(1)'

curl -fsS "$base/api/issues?status=in_progress" >/tmp/tala-smoke-filter-status.json
bun -e 'let d=require("/tmp/tala-smoke-filter-status.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?assignee=sam" >/tmp/tala-smoke-filter-assignee.json
bun -e 'let d=require("/tmp/tala-smoke-filter-assignee.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?blocked_by=$blocker_id" >/tmp/tala-smoke-filter-blocked-by.json
bun -e 'let d=require("/tmp/tala-smoke-filter-blocked-by.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?id=$issue_id" >/tmp/tala-smoke-filter-id.json
bun -e 'let d=require("/tmp/tala-smoke-filter-id.json"); if(d.length !== 1 || d[0].id !== process.argv[1]) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?blocker_of=$issue_id" >/tmp/tala-smoke-filter-blocker-of.json
bun -e 'let d=require("/tmp/tala-smoke-filter-blocker-of.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$blocker_id"

curl -fsS "$base/api/issues?parent_id=$parent_id" >/tmp/tala-smoke-filter-parent.json
bun -e 'let d=require("/tmp/tala-smoke-filter-parent.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?state=blocked" >/tmp/tala-smoke-filter-state-blocked.json
bun -e 'let d=require("/tmp/tala-smoke-filter-state-blocked.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?sort=updated_at&order=desc" >/tmp/tala-smoke-filter-sort.json
bun -e 'let d=require("/tmp/tala-smoke-filter-sort.json"); if(!Array.isArray(d) || d.length === 0) process.exit(1)'

curl -fsS "$base/api/issues?tag=api&priority=P2&q=Smoke" >/tmp/tala-smoke-filter-combined.json
bun -e 'let d=require("/tmp/tala-smoke-filter-combined.json"); if(!d.some(i => i.id === process.argv[1])) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?q=Smoke%20comment" >/tmp/tala-smoke-filter-comment.json
bun -e 'let d=require("/tmp/tala-smoke-filter-comment.json"); if(d.length !== 1 || d[0].id !== process.argv[1]) process.exit(1)' "$issue_id"

curl -fsS "$base/api/issues?q=$creator" >/tmp/tala-smoke-filter-creator.json
bun -e 'let d=require("/tmp/tala-smoke-filter-creator.json"); if(d.length !== 1 || d[0].id !== process.argv[1]) process.exit(1)' "$issue_id"

curl -fsS -X PATCH "$base/api/issues/$issue_id" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d '{"assignee":null}' >/tmp/tala-smoke-clear.json
bun -e 'let d=require("/tmp/tala-smoke-clear.json"); if(d.assignee !== null) process.exit(1)'

curl -fsS -X PUT "$base/api/issues/$issue_id/parent" \
  -H 'Content-Type: application/json' \
  -H 'X-Tala-Username: smoke' \
  -d '{"parent_issue_id":null}' >/tmp/tala-smoke-clear-parent.json
bun -e 'let d=require("/tmp/tala-smoke-clear-parent.json"); if(d.parent_issue_id !== null) process.exit(1)'

mcp_db="${TALA_SMOKE_DB:-.tala/tala.db}"
mcp_bin="${TALA_SMOKE_MCP_BIN:-/tmp/tala-smoke-mcp-stdio-$$}"
if [ -z "${TALA_SMOKE_MCP_BIN:-}" ]; then
  go build -o "$mcp_bin" ./cmd/tala-mcp-stdio
  trap 'rm -f "$mcp_bin"' EXIT
fi

mcp_request() {
  local payload="$1"
  printf '%s\n' "$payload" | "$mcp_bin" -db "$mcp_db"
}

mcp_request '{"jsonrpc":"2.0","id":9,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"tala-smoke","version":"0.0.0"},"capabilities":{}}}' >/tmp/tala-smoke-initialize.json
bun -e 'let d=require("/tmp/tala-smoke-initialize.json"); if(d.error || d.result?.protocolVersion !== "2025-06-18" || d.result?.serverInfo?.name !== "tala" || d.result?.serverInfo?.version !== "0.2.1" || !d.result?.capabilities?.tools || !d.result?.capabilities?.resources) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' >/tmp/tala-smoke-tools.json
bun -e 'let d=require("/tmp/tala-smoke-tools.json"); if(!d.result.tools.some(t => t.name === "issue_search" && t.inputSchema?.properties?.q)) process.exit(1); for (const name of ["issue_create","issue_update","issue_comment","issue_set_parent","issue_add_blocker","issue_remove_blocker","issue_assign","issue_set_status","issue_set_priority"]) { const tool = d.result.tools.find(t => t.name === name); if(!tool?.inputSchema?.properties?.username || !tool.inputSchema.required?.includes("username")) process.exit(1); }'

mcp_request '{"jsonrpc":"2.0","id":9007199254740993,"method":"tools/list","params":{}}' >/tmp/tala-smoke-mcp-large-id.json
bun -e 'let body=require("fs").readFileSync("/tmp/tala-smoke-mcp-large-id.json", "utf8"); if(!body.includes("\"id\":9007199254740993")) process.exit(1); let d=JSON.parse(body); if(d.error || !Array.isArray(d.result?.tools)) process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":10,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_get\",\"arguments\":{\"issue_id\":\"$issue_id\"}}}" >/tmp/tala-smoke-issue-get.json
bun -e 'let d=require("/tmp/tala-smoke-issue-get.json"); let r=d.result; if(r.isError !== false || r.structuredContent?.id !== process.argv[1]) process.exit(1); if(!Array.isArray(r.content) || r.content.length < 2) process.exit(1); let mirrored=JSON.parse(r.content[1].text); if(mirrored.id !== process.argv[1] || mirrored.description_markdown !== "Smoke **markdown**") process.exit(1)' "$issue_id"

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":13,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_create\",\"arguments\":{\"username\":\"smoke\",\"title\":\"MCP smoke child $$\",\"description_markdown\":\"Created through MCP **tool**.\",\"priority\":\"P3\",\"parent_issue_id\":\"$parent_id\",\"tag_names\":[\"mcp\",\"smoke\"]}}}" >/tmp/tala-smoke-mcp-create.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-create.json"); let r=d.result; if(d.error || r?.isError !== false) process.exit(1); let issue=r.structuredContent; if(issue.title !== process.argv[1] || issue.parent_issue_id !== process.argv[2] || !issue.tags?.some(t => t.name === "mcp")) process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.id !== issue.id || mirrored.description_markdown !== "Created through MCP **tool**.") process.exit(1)' "MCP smoke child $$" "$parent_id"
mcp_child_id="$(bun -e 'let d=require("/tmp/tala-smoke-mcp-create.json"); console.log(d.result.structuredContent.id)')"

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":14,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_search\",\"arguments\":{\"parent_id\":\"$parent_id\",\"tag\":\"mcp\",\"q\":\"MCP smoke child $$\"}}}" >/tmp/tala-smoke-mcp-search.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-search.json"); let r=d.result; if(d.error || r?.isError !== false || !Array.isArray(r.structuredContent) || !r.structuredContent.some(i => i.id === process.argv[1])) process.exit(1)' "$mcp_child_id"

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":33,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_set_priority\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$mcp_child_id\",\"priority\":\"P2\"}}}" >/tmp/tala-smoke-mcp-priority.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-priority.json"); let r=d.result; if(d.error || r?.isError !== false || r.structuredContent?.priority !== "P2") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"issue_comment\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"body_markdown\":\"No ID should not mutate.\"}}}" >/tmp/tala-smoke-mcp-tool-no-id.txt
test ! -s /tmp/tala-smoke-mcp-tool-no-id.txt
curl -fsS "$base/api/issues/$issue_id" >/tmp/tala-smoke-no-id-detail.json
bun -e 'let d=require("/tmp/tala-smoke-no-id-detail.json"); if(d.comment_count !== 1) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":26,"method":"tools/call","params":{"name":"issue_get","arguments":{"issue_id":42}}}' >/tmp/tala-smoke-mcp-invalid-get-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-get-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "issue_id") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "issue_id") process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":27,"method":"tools/call","params":{"name":"issue_update","arguments":{"username":"smoke","issue_id":"'"$issue_id"'","title":42}}}' >/tmp/tala-smoke-mcp-invalid-update-title-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-update-title-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "title") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "title") process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"issue_set_status","arguments":{"username":"smoke","issue_id":"issue_missing","status":"in_progress"}}}' >/tmp/tala-smoke-mcp-tool-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-tool-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "not_found" || r.structuredContent?.field !== "issue_id") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "not_found" || mirrored.field !== "issue_id") process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"issue_comment","arguments":{"issue_id":"issue_missing","body_markdown":"Missing username should be a tool error."}}}' >/tmp/tala-smoke-mcp-username-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-username-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "missing_username" || r.structuredContent?.field !== "username") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "missing_username" || mirrored.field !== "username") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":25,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_set_status\",\"arguments\":{\"username\":42,\"issue_id\":\"$issue_id\",\"status\":\"in_progress\"}}}" >/tmp/tala-smoke-mcp-invalid-username-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-username-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "username") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "username") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":18,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_comment\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"body_markdown\":null}}}" >/tmp/tala-smoke-mcp-null-comment-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-null-comment-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "body_markdown") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "body_markdown") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":23,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_comment\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"body_markdown\":42}}}" >/tmp/tala-smoke-mcp-invalid-comment-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-comment-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "body_markdown") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "body_markdown") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":19,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_add_blocker\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"blocker_issue_id\":null}}}" >/tmp/tala-smoke-mcp-null-blocker-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-null-blocker-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "blocker_issue_id") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "blocker_issue_id") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":20,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_set_parent\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"parent_issue_id\":42}}}" >/tmp/tala-smoke-mcp-invalid-parent-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-parent-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "parent_issue_id") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "parent_issue_id") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":21,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_update\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"tag_names\":42}}}" >/tmp/tala-smoke-mcp-invalid-tags-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-tags-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "tag_names") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "tag_names") process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":24,"method":"tools/call","params":{"name":"issue_set_status","arguments":{"username":"smoke","issue_id":42,"status":"in_progress"}}}' >/tmp/tala-smoke-mcp-invalid-issue-id-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-invalid-issue-id-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "issue_id") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "issue_id") process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":17,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_set_priority\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\"}}}" >/tmp/tala-smoke-mcp-missing-argument-error.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-missing-argument-error.json"); let r=d.result; if(d.error || r?.isError !== true) process.exit(1); if(r.structuredContent?.code !== "validation_error" || r.structuredContent?.field !== "priority") process.exit(1); let mirrored=JSON.parse(r.content?.[1]?.text || "{}"); if(mirrored.code !== "validation_error" || mirrored.field !== "priority") process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":11,"method":"resources/list","params":{}}' >/tmp/tala-smoke-resources.json
bun -e 'let d=require("/tmp/tala-smoke-resources.json"); if(!d.result.resources.some(r => r.uri === "tala://board") || d.result.resources.some(r => r.uriTemplate)) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":12,"method":"resources/templates/list","params":{}}' >/tmp/tala-smoke-resource-templates.json
bun -e 'let d=require("/tmp/tala-smoke-resource-templates.json"); for (const uri of ["tala://issues/{id}","tala://issues/{id}/tree","tala://issues/{id}/blockers"]) if(!d.result.resourceTemplates.some(r => r.uriTemplate === uri)) process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"resources/read\",\"params\":{\"uri\":\"tala://issues/$issue_id/blockers\"}}" >/tmp/tala-smoke-blockers.json
bun -e 'let d=require("/tmp/tala-smoke-blockers.json"); let b=JSON.parse(d.result.contents[0].text); if(!b.unresolved_blockers?.some(i => i.title === "Smoke blocker") || !Array.isArray(b.resolved_blockers) || !Array.isArray(b.unresolved_blocked_by) || !Array.isArray(b.resolved_blocked_by)) process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":35,\"method\":\"resources/read\",\"params\":{\"uri\":\"tala://issues/$issue_id\"}}" >/tmp/tala-smoke-issue-resource.json
bun -e 'let d=require("/tmp/tala-smoke-issue-resource.json"); let issue=JSON.parse(d.result.contents[0].text); if(issue.id !== process.argv[1] || issue.description_markdown !== "Smoke **markdown**" || !Array.isArray(issue.recent_comments)) process.exit(1)' "$issue_id"

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":36,\"method\":\"resources/read\",\"params\":{\"uri\":\"tala://issues/$parent_id/tree\"}}" >/tmp/tala-smoke-tree-resource.json
bun -e 'let d=require("/tmp/tala-smoke-tree-resource.json"); let tree=JSON.parse(d.result.contents[0].text); if(tree.issue?.id !== process.argv[1] || !Array.isArray(tree.children) || !tree.children.some(i => i.title === process.argv[2])) process.exit(1)' "$parent_id" "MCP smoke child $$"

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":34,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_set_parent\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$mcp_child_id\",\"parent_issue_id\":null}}}" >/tmp/tala-smoke-mcp-clear-parent.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-clear-parent.json"); let r=d.result; if(d.error || r?.isError !== false || r.structuredContent?.parent_issue_id !== null) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":22,"method":"resources/read","params":{"uri":"tala://issues/issue_missing"}}' >/tmp/tala-smoke-resource-missing.json
bun -e 'let d=require("/tmp/tala-smoke-resource-missing.json"); if(d.error?.code !== -32002 || d.error?.data?.uri !== "tala://issues/issue_missing") process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"tala://board"}}' >/tmp/tala-smoke-board.json
bun -e 'let d=require("/tmp/tala-smoke-board.json"); let board=JSON.parse(d.result.contents[0].text); for (const s of ["new","in_progress","completed","canceled"]) if (!Array.isArray(board[s])) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"tala://planning"}}' >/tmp/tala-smoke-planning.json
bun -e 'let d=require("/tmp/tala-smoke-planning.json"); let p=JSON.parse(d.result.contents[0].text); if (!p.children_by_parent || !Array.isArray(p.blocked) || !Array.isArray(p.blocking)) process.exit(1); if (!p.blocked.some(c => c.issue?.title === "Smoke issue" && c.unresolved_blockers?.some(b => b.title === "Smoke blocker") && Array.isArray(c.resolved_blockers))) process.exit(1); if (!p.blocking.some(c => c.issue?.title === "Smoke blocker" && c.blocked_by?.some(b => b.title === "Smoke issue") && c.unresolved_blocked_by?.some(b => b.title === "Smoke issue") && Array.isArray(c.resolved_blocked_by))) process.exit(1)'

mcp_request "{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"issue_remove_blocker\",\"arguments\":{\"username\":\"smoke\",\"issue_id\":\"$issue_id\",\"blocker_issue_id\":\"$blocker_id\"}}}" >/tmp/tala-smoke-mcp-remove-blocker.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-remove-blocker.json"); let r=d.result; if(d.error || r?.isError !== false || r.structuredContent?.status !== "ok") process.exit(1)'

curl -fsS "$base/api/issues/$issue_id" >/tmp/tala-smoke-unblocked-detail.json
bun -e 'let d=require("/tmp/tala-smoke-unblocked-detail.json"); if(d.blocked || d.blockers.length !== 0) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":5,"method":"tools/list"} {}' >/tmp/tala-smoke-mcp-trailing.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-trailing.json"); if (d.error?.code !== -32700) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":28,"method":"tools/call","params":{"name":"issue_search"}}' >/tmp/tala-smoke-mcp-omitted-arguments.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-omitted-arguments.json"); let r=d.result; if(d.error || r?.isError !== false || !Array.isArray(r.structuredContent)) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":29,"method":"tools/call","params":{"name":"issue_search","arguments":null}}' >/tmp/tala-smoke-mcp-null-arguments.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-null-arguments.json"); if(d.error?.code !== -32602) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":30,"method":"tools/call","params":{"name":"issue_search","arguments":[]}}' >/tmp/tala-smoke-mcp-array-arguments.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-array-arguments.json"); if(d.error?.code !== -32602) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":31,"method":"tools/call","params":{}}' >/tmp/tala-smoke-mcp-missing-tool-name.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-missing-tool-name.json"); if(d.error?.code !== -32602) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":32,"method":"resources/read","params":{"uri":null}}' >/tmp/tala-smoke-mcp-null-resource-uri.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-null-resource-uri.json"); if(d.error?.code !== -32602) process.exit(1)'

mcp_request '[{"jsonrpc":"2.0","id":33,"method":"tools/list"}]' >/tmp/tala-smoke-mcp-batch.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-batch.json"); if(d.error?.code !== -32600) process.exit(1)'

mcp_request '"not a request object"' >/tmp/tala-smoke-mcp-string-request.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-string-request.json"); if(d.error?.code !== -32600) process.exit(1)'

mcp_request '{"jsonrpc":"2.0","id":1.5,"method":"tools/list","params":{}}' >/tmp/tala-smoke-mcp-fractional-id.json
bun -e 'let d=require("/tmp/tala-smoke-mcp-fractional-id.json"); if(d.error?.code !== -32600) process.exit(1)'

curl -fsSI "$base/" | awk 'NR == 1 { if ($2 != 200) exit 1 }'

echo "smoke ok: $base"
