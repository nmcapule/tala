# Ultra-Lightweight Issue Tracker Design

## Summary

This system is a small issue tracking hub for coordination between human users and AI agents. It provides one shared issue model through three interfaces:

- A web frontend for human triage, planning, and feedback.
- A REST API for scripts and automations.
- An MCP interface for AI agents.

The first version is intentionally local-first and lightweight. It runs as a single Go server backed by SQLite, with no real authentication or authorization. Users identify themselves by entering a username.

Issue descriptions and comments are always Markdown source. The UI renders Markdown safely for humans, while REST and MCP preserve and return the original Markdown text.

## Goals

- Track issues with title, Markdown description, status, priority, tags, assignee, comments, parent issue, child issues, and blocker issues.
- Make the triage board the primary human workflow.
- Include richer planning views for issue hierarchy and blockers.
- Give AI agents compact, reliable tools and resources for coordination.
- Keep deployment and operation simple enough for local development and small team use.

## Non-Goals

- Passwords, OAuth, SSO, API keys, roles, or authorization.
- Multi-tenant organizations or separate projects.
- File attachments.
- Email or push notifications.
- Real-time collaboration.
- Custom workflows or custom fields.
- Full OpenAPI generation in v1.

## Technology Choices

- Backend: Go.
- HTTP router: `chi`.
- Database: SQLite.
- Database access: `database/sql` with `sqlc` generated query methods.
- Frontend: React + Vite.
- UI styling: Tailwind CSS + shadcn/ui.
- REST API: endpoint tables documented in this design.
- MCP: served by the same Go process as the web frontend and REST API.
- MCP transport: Streamable HTTP at `/mcp`, using JSON-RPC.

The MCP transport should follow the current Model Context Protocol Streamable HTTP guidance: use a single endpoint that accepts POST and GET where supported, validate `Origin`, bind local deployments to `127.0.0.1` by default, and treat stronger authentication as future work.

References:

- MCP transports: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
- MCP tools: https://modelcontextprotocol.io/specification/2025-06-18/server/tools
- MCP resources: https://modelcontextprotocol.io/specification/2025-06-18/server/resources

## Domain Model

### Issue

An issue is the central unit of coordination.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | Stable generated ID. |
| `title` | string | Required, plain text. |
| `description_markdown` | string | Required Markdown source; may be empty. |
| `status` | enum | `new`, `in_progress`, `completed`, `canceled`. |
| `priority` | enum | `P0`, `P1`, `P2`, `P3`, `P4`; `P0` is highest. |
| `assignee` | string nullable | Username string. |
| `created_by` | string | Username string. |
| `parent_issue_id` | string nullable | Optional parent issue. |
| `created_at` | timestamp | Set on creation. |
| `updated_at` | timestamp | Updated on mutation. |

Blocked work is inferred from unresolved blocker issues. There is no separate `blocked` status in v1.

### Tag

Tags are reusable labels attached to issues many-to-many.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | Stable generated ID. |
| `name` | string | Required, unique, case-insensitive. |
| `color` | string nullable | Optional UI color token or hex value. |
| `created_at` | timestamp | Set on creation. |

### Comment

Comments are append-only Markdown entries.

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | Stable generated ID. |
| `issue_id` | string | Parent issue. |
| `author` | string | Username string. |
| `body_markdown` | string | Required Markdown source. |
| `created_at` | timestamp | Set on creation. |

Comments are not edited or deleted in v1. Corrections should be added as new comments.

### Relationships

Parent/child relationships model decomposition. Each issue may have at most one parent and any number of children.

Blocker relationships model dependency. If issue A is blocked by issue B, then B must be completed or canceled before A is considered unblocked.

Validation rules:

- An issue cannot be its own parent.
- Parent/child relationships cannot create cycles.
- An issue cannot block itself.
- Blocker relationships cannot create cycles.
- Duplicate blocker relationships are ignored or returned as a no-op success.
- Deleting an issue is not supported in v1; issues should be canceled instead.

## SQLite Schema

The schema should be implemented with migrations, even if v1 has only one migration.

```sql
CREATE TABLE issues (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description_markdown TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('new', 'in_progress', 'completed', 'canceled')),
  priority TEXT NOT NULL CHECK (priority IN ('P0', 'P1', 'P2', 'P3', 'P4')),
  assignee TEXT,
  created_by TEXT NOT NULL,
  parent_issue_id TEXT REFERENCES issues(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE tags (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  color TEXT,
  created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX tags_name_unique ON tags (lower(name));

CREATE TABLE issue_tags (
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (issue_id, tag_id)
);

CREATE TABLE comments (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  author TEXT NOT NULL,
  body_markdown TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE issue_blockers (
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  blocker_issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY (issue_id, blocker_issue_id),
  CHECK (issue_id <> blocker_issue_id)
);

CREATE INDEX issues_status_idx ON issues (status);
CREATE INDEX issues_priority_idx ON issues (priority);
CREATE INDEX issues_assignee_idx ON issues (assignee);
CREATE INDEX issues_parent_idx ON issues (parent_issue_id);
CREATE INDEX comments_issue_created_idx ON comments (issue_id, created_at);
CREATE INDEX issue_blockers_blocker_idx ON issue_blockers (blocker_issue_id);
```

Cycle checks should live in the Go domain layer rather than SQLite triggers, so REST and MCP share the same validation behavior and error messages.

## Backend Architecture

Use one Go binary with these internal boundaries:

- HTTP layer: request parsing, response formatting, username extraction.
- Domain services: issue mutation, relationship validation, filtering, Markdown field handling.
- Store layer: `sqlc` generated queries plus small transaction wrappers.
- REST handlers: thin wrappers around domain services.
- MCP handlers: thin wrappers around the same domain services.
- Static frontend serving: Vite build output served by the Go server in production.

The username should be supplied through:

- Web frontend: browser local storage after the username login screen.
- REST API: `X-Tala-Username` request header.
- MCP: an optional `username` argument on mutating tools, defaulting to the MCP session/user label if available.

If no username is supplied for a mutating operation, return a validation error.

## REST API

All responses use JSON. All mutating endpoints require a username through `X-Tala-Username`.

### Issues

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/issues` | List/search issues. |
| `POST` | `/api/issues` | Create an issue. |
| `GET` | `/api/issues/{id}` | Get issue detail, including tags, children, blockers, and recent comments. |
| `PATCH` | `/api/issues/{id}` | Update issue fields. |
| `POST` | `/api/issues/{id}/comments` | Append a Markdown comment. |
| `GET` | `/api/issues/{id}/comments` | List comments oldest first. |

`GET /api/issues` supports these query parameters:

- `status`
- `priority`
- `assignee`
- `tag`
- `id`
- `parent_id`
- `blocked_by`
- `blocker_of`
- `state` (`open`, `blocked`, `done`)
- `q`
- `sort` (`priority`, `updated_at`, `created_at`, `title`, `status`)
- `order` (`asc`, `desc`)

Create issue request:

```json
{
  "title": "Add MCP issue search",
  "description_markdown": "Expose issue search to agents.",
  "priority": "P2",
  "assignee": "alex",
  "tag_names": ["mcp", "api"],
  "parent_issue_id": null
}
```

Update issue request:

```json
{
  "title": "Add MCP issue search",
  "description_markdown": "Expose issue search to agents via a tool.",
  "status": "in_progress",
  "priority": "P1",
  "assignee": "sam",
  "tag_names": ["mcp", "api"]
}
```

Comment request:

```json
{
  "body_markdown": "I tested this with a local MCP client."
}
```

### Tags

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/tags` | List tags. |
| `POST` | `/api/tags` | Create a tag. |
| `PATCH` | `/api/tags/{id}` | Update tag name or color. |

### Relationships

| Method | Path | Purpose |
| --- | --- | --- |
| `PUT` | `/api/issues/{id}/parent` | Set or clear parent issue. |
| `POST` | `/api/issues/{id}/blockers` | Add a blocker issue. |
| `DELETE` | `/api/issues/{id}/blockers/{blocker_id}` | Remove a blocker issue. |

Set parent request:

```json
{
  "parent_issue_id": "issue_123"
}
```

Use `null` to clear the parent.

Add blocker request:

```json
{
  "blocker_issue_id": "issue_456"
}
```

### Error Format

Use one consistent error shape:

```json
{
  "error": {
    "code": "cycle_detected",
    "message": "Adding this blocker would create a dependency cycle.",
    "field": "blocker_issue_id"
  }
}
```

Common error codes:

- `validation_error`
- `not_found`
- `cycle_detected`
- `missing_username`
- `conflict`
- `internal_error`

## MCP Interface

The MCP server is exposed at `/mcp` using Streamable HTTP. It advertises tools and resources.

### Tools

Tools should return structured JSON content plus a short text summary where helpful.

| Tool | Purpose |
| --- | --- |
| `issue_create` | Create an issue with Markdown description, tags, assignee, priority, and optional parent. |
| `issue_update` | Update issue fields. |
| `issue_search` | Search/filter issues. |
| `issue_get` | Fetch issue detail. |
| `issue_comment` | Append a Markdown comment. |
| `issue_set_parent` | Set or clear parent issue. |
| `issue_add_blocker` | Add a blocker issue. |
| `issue_remove_blocker` | Remove a blocker issue. |
| `issue_assign` | Set or clear assignee. |
| `issue_set_status` | Change status. |
| `issue_set_priority` | Change priority. |

Markdown fields must be named explicitly:

- `description_markdown`
- `body_markdown`

Example `issue_comment` input:

```json
{
  "issue_id": "issue_123",
  "username": "agent",
  "body_markdown": "I found a failing case in `issue_search` filtering."
}
```

### Resources

| Resource URI | Purpose |
| --- | --- |
| `tala://board` | Compact board state grouped by status. |
| `tala://issues/{id}` | Issue detail context. |
| `tala://issues/{id}/tree` | Parent, siblings, and children. |
| `tala://issues/{id}/blockers` | Blockers and issues blocked by this issue. |
| `tala://planning` | High-level planning context across hierarchy and blockers. |

Resources should be concise and agent-readable. They should include Markdown source for descriptions and comments, not rendered HTML.

## Web Frontend

### Login

The first screen asks for a username. The username is stored in browser local storage and sent on REST mutations as `X-Tala-Username`.

There is no password and no server-side session in v1.

### Triage Board

The board is the main screen. Columns are:

- `new`
- `in_progress`
- `completed`
- `canceled`

Each issue card shows:

- Title.
- Priority.
- Assignee.
- Tags.
- Blocked indicator if any unresolved blocker exists.
- Child count.
- Comment count.

Users can:

- Create an issue.
- Open issue detail.
- Move issues between status columns.
- Filter by assignee, priority, tag, and text query.

### Issue Detail

The issue detail view supports:

- Editing title.
- Editing Markdown description.
- Previewing sanitized Markdown.
- Changing status, priority, assignee, and tags.
- Adding Markdown comments.
- Viewing parent, children, blockers, and blocked-by issues.
- Adding/removing blockers.
- Setting/clearing parent issue.

### Planning Views

Planning views supplement the board:

- Hierarchy view: tree of parent and child issues.
- Blocker view: dependency-focused view showing blockers and blocked-by relationships.

These views do not introduce new data concepts. They are alternate projections of the same issue model.

## Markdown Handling

Descriptions and comments are stored as Markdown source and never converted before persistence.

REST and MCP return Markdown source exactly as stored.

The web frontend renders Markdown through a sanitizer that removes unsafe HTML, scripts, event handlers, and dangerous URLs before display.

Recommended frontend rendering approach:

- Parse Markdown with a common React Markdown renderer.
- Sanitize output with a maintained sanitizer.
- Disable raw HTML unless there is a clear need to support it later.

Markdown search in v1 can be plain text matching over raw Markdown source.

## Validation And Behavior

- `title` is required and trimmed.
- `description_markdown` defaults to an empty string.
- `body_markdown` is required for comments.
- Unknown statuses and priorities are rejected.
- Unknown issue IDs return `not_found`.
- Parent and blocker cycles return `cycle_detected`.
- Canceling or completing an issue does not automatically update children or dependent issues.
- A blocked indicator is computed from blockers whose status is not `completed` or `canceled`.
- Updating tags by name creates missing tags automatically unless the tag endpoint is used to customize color.

## Testing Strategy

### Domain Tests

- Create and update issues.
- Add tags by name.
- Append comments and verify oldest-first ordering.
- Store and return Markdown unchanged.
- Reject invalid statuses and priorities.
- Reject parent cycles.
- Reject blocker cycles.
- Compute blocked state from unresolved blockers.

### REST Tests

- Issue CRUD and filtering.
- Relationship endpoints and validation errors.
- Comment append and retrieval.
- Missing username on mutations.
- Error response shape.

### MCP Tests

- Tool discovery includes all issue tools.
- Tools call the same domain services as REST.
- Markdown arguments are preserved.
- Resources return compact board, issue, hierarchy, and blocker context.
- Invalid mutations return structured errors.

### Frontend Tests

- Username login flow.
- Board grouping and drag/status change behavior.
- Issue detail editing.
- Markdown edit/preview rendering.
- Sanitization of unsafe Markdown/HTML input.
- Hierarchy and blocker views render correct relationships.

## Future Work

- Real authentication and authorization.
- API keys for automation.
- OpenAPI generation.
- Realtime updates.
- Attachments.
- Custom statuses or workflows.
- Projects or workspaces.
- Audit log.
- Notification integrations.
