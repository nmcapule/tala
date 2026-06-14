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

### Project Roadmap Database Workflow

The repository's `.tala/tala.db` is the durable Tala project ledger. It is the source of truth for roadmap issues, known bugs, agent handoffs, estimates, and completion evidence.

Use `.tala/tala.db` only for durable project work:

- Run `make own-db` when you need the browser UI against the project roadmap.
- Use the repo-local Tala MCP tools for agent planning and progress updates.
- Commit `.tala/tala.db` changes only when they represent real roadmap, bug, release, or handoff updates.
- Before risky manual edits or release-candidate work, make a local backup with `cp .tala/tala.db .tala/tala.db.bak-$(date +%Y%m%d%H%M%S)`.

Do not use `.tala/tala.db` for throwaway verification data:

- Run smoke and browser-smoke checks against a disposable database such as `/tmp/tala-v1-candidate.db`.
- Use `DB`, `OWN_DB`, `TALA_DB`, or command-line `-db` overrides for experiments, demos, and reproductions.
- If a test accidentally writes to `.tala/tala.db`, remove only run-specific records after confirming their title, creator, tag, or timestamp, then re-run a search for known smoke prefixes.

To refresh or recreate the seeded roadmap database, start from a known-good checkout or backup, run `make own-db`, apply durable issue changes through the UI or Tala MCP tools, verify there are no temporary smoke records, and commit the updated `.tala/tala.db` together with any matching documentation changes. Do not regenerate the roadmap ledger from smoke scripts; smoke scripts are verification fixtures, not seed sources.

For repeatable demo or manual-QA data, create a disposable fixture database instead of editing the project ledger:

```sh
make fixture-db
go run ./cmd/tala -addr 127.0.0.1:8081 -db /tmp/tala-fixture.db
```

`make fixture-db` recreates `/tmp/tala-fixture.db`, starts a temporary local Tala server, seeds representative tags, parent/child issues, blockers, story points, and comments through the public REST API, then stops the server. Override `FIXTURE_DB` and `FIXTURE_ADDR` when needed. The fixture script refuses `.tala/tala.db` unless `TALA_ALLOW_PROJECT_DB_FIXTURE=1` is set intentionally.

Run the Vite frontend separately during frontend work:

```sh
bun run dev
```

The Vite dev server proxies `/api`, `/mcp`, and `/uploads` to `127.0.0.1:8080`, so start the Go server on that port when using the frontend dev server:

```sh
go run ./cmd/tala -addr 127.0.0.1:8080 -db .tala/tala.db
```

## Security Model

Tala v1 is a local-first tool and does not implement server-side accounts, API keys, authorization checks, or per-user data isolation. The username field records who performed a mutation, but it is not authentication. Anyone who can reach the HTTP server can read and mutate issues, comments, tags, uploads, REST endpoints, and MCP tools.

Keep the server bound to loopback for normal use:

```sh
go run ./cmd/tala -addr 127.0.0.1:8081 -db .tala/tala.db
```

Only expose Tala through a proxy when that proxy's access policy matches the data in the database. In an exe.dev VM, use the documented proxy controls at `https://exe.dev/docs/proxy.md`: private proxies are the default, `ssh exe.dev share set-public <vmname>` makes the selected port public, and `ssh exe.dev share set-private <vmname>` returns it to private access. Do not make a Tala port public unless the database is intentionally shareable.

The default database path is `.tala/tala.db`. Use `-db`, `TALA_DB`, or the Makefile `DB`/`OWN_DB` variables to point experiments, smoke tests, and shared demos at a separate database instead of the project work ledger.

## Current Capabilities

- Username-only local identity for issue edits, comments, and agent coordination.
- Board columns for `new`, `in_progress`, `completed`, and `canceled`, with drag/drop and status-select updates.
- Issue detail editing for title, Markdown description, status, priority, story points, assignee, tags, parent, blockers, and comments.
- Markdown source preservation with sanitized preview rendering in the frontend.
- Hierarchy and blocker planning views for parent/child work and dependency chains.
- Tag management with named color tokens and custom hex colors.
- Search and filtering by text, status, priority, assignee, tag, parent, and blocker.

## Story Points

Issues can carry an optional direct Fibonacci story point estimate: `1`, `2`, `3`, `5`, `8`, `13`, or `21`. API and MCP responses also include `story_points_total`, which adds the issue's direct estimate to all child and descendant estimates. A `2SP` child therefore contributes `2SP` to its parent and each ancestor.

Sizing guide:

- `1SP`: very easy, quick task.
- `2SP`: fairly easy task that still needs a human assignee to think a bit.
- `3SP`: about a day of human work.
- `5SP`: about two days of human work.
- `8SP`: about a week; agents must break this down into smaller child issues.
- `13SP` and `21SP`: large planning estimates; agents must break these down into smaller child issues.

The REST API and web UI can record large direct estimates for human planning. MCP agent mutation tools reject direct `8SP+` estimates on leaf issues so agent work is decomposed first.

## Verification

The v1 release gate is tracked in [docs/v1-release-checklist.md](docs/v1-release-checklist.md).

Run backend tests and frontend typecheck:

```sh
make test
```

Build production frontend assets and the Go binary:

```sh
make build
```

Verify the production binary serves the embedded frontend assets:

```sh
make verify-production-binary
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

`GET /api/issues` accepts `status`, `priority`, `assignee`, `tag`, `id`, `parent_id`, `blocked_by`, `blocker_of`, `state`, `q`, `sort`, and `order`. Valid states are `open`, `blocked`, and `done`; valid sort fields are `priority`, `updated_at`, `created_at`, `title`, and `status`. Issue create and update bodies accept nullable `story_points`; responses include `story_points` and computed `story_points_total`.

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

`issue_create` and `issue_update` accept nullable `story_points`. Agent tools reject direct `8SP+` estimates unless the issue already has child issues, requiring large work to be broken down first.

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

The plugin starts the Tala MCP server over stdio against the current workspace's `.tala/tala.db`. Each Codex project gets its own database by default. Set `TALA_DB` to use an explicit database, `TALA_WORKSPACE_ROOT` to choose the project root used for the default database path, or `TALA_SOURCE_ROOT` if the plugin cannot infer the Tala source checkout used to run the MCP server.

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
