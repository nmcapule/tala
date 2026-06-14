# Tala

Tala is a local-first issue tracker for human users and AI agents. It runs as a single Go server backed by SQLite, serves a mobile-first React/Vite frontend, exposes REST endpoints under `/api`, and exposes MCP JSON-RPC tools/resources at `/mcp`.

Production frontend assets are embedded from `cmd/tala/static`, so the Go server can serve the built app without a separate Node/Bun process.

The product and behavior source of truth is [docs/issue-tracker-design.md](docs/issue-tracker-design.md). Visual direction comes from [.stitch/DESIGN.md](.stitch/DESIGN.md) and the mockups under `.stitch/designs/`.

## Requirements

- Go 1.26+
- Bun 1.3+

## Development

Install frontend dependencies:

```sh
bun install
```

Run the integrated Go server with the embedded frontend assets:

```sh
make dev
```

By default this listens on `127.0.0.1:8081` and stores data in `.tala/tala.db`.

You can override both:

```sh
make dev GO_ADDR=127.0.0.1:8090 DB=/tmp/tala.db
```

Run Tala against the project-local seeded database:

```sh
make own-db
```

`make own-db` listens on `127.0.0.1:8081` and uses `.tala/tala.db`. The currently seeded database contains Tala's own roadmap and known-bugs tracker.

You can override the seeded-db address and path:

```sh
make own-db OWN_DB_ADDR=127.0.0.1:8090 OWN_DB=/tmp/tala.db
```

Run the Vite frontend separately during frontend work:

```sh
bun run dev
```

The Vite dev server proxies `/api`, `/mcp`, and `/uploads` to `127.0.0.1:8080`, so start the Go server on that port when using the frontend dev server:

```sh
go run ./cmd/tala -addr 127.0.0.1:8080 -db .tala/tala.db
```

## Current Capabilities

- Username-only local identity for issue edits, comments, and agent coordination.
- Board columns for `new`, `in_progress`, `completed`, and `canceled`, with drag/drop and status-select updates.
- Issue detail editing for title, Markdown description, status, priority, assignee, tags, parent, blockers, and comments.
- Markdown source preservation with sanitized preview rendering in the frontend.
- Hierarchy and blocker planning views for parent/child work and dependency chains.
- Tag management with named color tokens and custom hex colors.
- Search and filtering by text, status, priority, assignee, tag, parent, and blocker.

## Verification

Run backend tests and frontend typecheck:

```sh
make test
```

Build production frontend assets and the Go binary:

```sh
make build
```

Run a smoke check against a running server:

```sh
make smoke
```

Or target a specific URL:

```sh
scripts/smoke.sh http://127.0.0.1:8090
```

The smoke check creates temporary issues/tags, verifies REST mutation behavior, checks MCP tool/resource behavior, and verifies static frontend serving.

Run the browser smoke check against a running server:

```sh
make browser-smoke
```

Or target a specific URL:

```sh
make browser-smoke SMOKE_URL=http://127.0.0.1:8090
```

The browser smoke check uses `agent-browser` to exercise the React UI, including login, board columns, issue creation/editing, Markdown preview/sanitization, relationships, and tag color rendering.

## Runtime Configuration

The Go server supports flags and environment variables:

```sh
go run ./cmd/tala -addr 127.0.0.1:8081 -db .tala/tala.db
```

- `-addr` or `TALA_ADDR`: listen address, default `127.0.0.1:8080`
- `-db` or `TALA_DB`: SQLite database path, default `.tala/tala.db`

## API Surfaces

Health:

- `GET /healthz`

Issues:

- `GET /api/issues`
- `POST /api/issues`
- `GET /api/issues/{id}`
- `PATCH /api/issues/{id}`

`GET /api/issues` accepts `status`, `priority`, `assignee`, `tag`, `id`, `parent_id`, `blocked_by`, `blocker_of`, `state`, `q`, `sort`, and `order`. Valid states are `open`, `blocked`, and `done`; valid sort fields are `priority`, `updated_at`, `created_at`, `title`, and `status`.

Comments:

- `GET /api/issues/{id}/comments`
- `POST /api/issues/{id}/comments`

Uploads:

- `POST /api/uploads/images`
- `GET /uploads/images/{filename}`

Image uploads accept multipart form field `image`, require `X-Tala-Username`, and store files under the configured database directory, for example `.tala/uploads/images` when using `.tala/tala.db`. The upload response includes a same-origin URL and ready-to-paste Markdown.

Relationships:

- `PUT /api/issues/{id}/parent`
- `POST /api/issues/{id}/blockers`
- `DELETE /api/issues/{id}/blockers/{blockerID}`

Tags:

- `GET /api/tags`
- `POST /api/tags`
- `PATCH /api/tags/{id}`

Mutating REST requests require `X-Tala-Username`.

## MCP Surface

The MCP endpoint is `/mcp`. It supports JSON-RPC initialize, tool listing/calls, resource listing, resource-template listing, and resource reads.

For Codex integration, the repo-local `tala-project-tracker` plugin starts the same MCP surface over stdio:

```sh
go run ./cmd/tala-mcp-stdio -db .tala/tala.db
```

Use the HTTP `/mcp` endpoint when running the full Tala server for browser, REST, or smoke-test workflows. Use the stdio command for clients, such as Codex, that launch MCP servers directly.

Tools:

- `image_upload`
- `issue_create`
- `issue_update`
- `issue_search`
- `issue_get`
- `issue_comment`
- `issue_set_parent`
- `issue_add_blocker`
- `issue_remove_blocker`
- `issue_assign`
- `issue_set_status`
- `issue_set_priority`

Use `image_upload` with a local screenshot path to get a Markdown image link for issue descriptions or comments. This is the preferred agent path for `agent-browser` screenshots.

Resources:

- `tala://board`
- `tala://planning`
- `tala://issues/{id}`
- `tala://issues/{id}/tree`
- `tala://issues/{id}/blockers`

Mutating MCP tools require a `username` argument on this local server. Tala does not establish an MCP session user label yet, so omitted or blank usernames return a `missing_username` validation error.

## Codex Plugin

This repo includes a local Codex plugin that packages the Tala MCP server and agent skill together:

- `plugins/tala-project-tracker`

Install the repo marketplace, then add the plugin:

```sh
codex plugin marketplace add .
codex plugin add tala-project-tracker@tala
```

Then invoke the skill in Codex with:

```text
$tala-project-tracker plan this work in Tala and keep the issue updated
```

The plugin starts the Tala MCP server over stdio against the current workspace's `.tala/tala.db`. Set `TALA_DB` to use another database, or `TALA_WORKSPACE_ROOT` if Codex launches the plugin from outside the repo.

### Codex Plugin Release

Release the repo-local plugin from the checkout with the helper script:

```sh
scripts/release-codex-plugin.sh
```

The script validates `plugins/tala-project-tracker`, checks `.agents/plugins/marketplace.json`, bumps the plugin `+codex.<timestamp>` cachebuster, runs `go test ./...`, refreshes the repo marketplace, and reinstalls `tala-project-tracker@tala`.

Use a dry run before releasing:

```sh
scripts/release-codex-plugin.sh --dry-run
```

Use `--no-bump` only when the manifest version should remain unchanged. After reinstalling, start a new Codex thread so the refreshed skill and MCP tools are loaded cleanly.
