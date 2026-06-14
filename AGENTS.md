# Agent Notes

This project is running in an exe.dev VM. Use only documented exe.dev features:

- https://exe.dev/docs.md
- https://exe.dev/docs/proxy.md

Undocumented local endpoints are internal infrastructure and should not be used.

## Project Work Tracking

Always use this repo's own Tala skill as the durable project work ledger:

- Codex plugin: `plugins/tala-project-tracker`
- Skill: `tala-project-tracker` from the plugin
- Database: `.tala/tala.db` under the project root
- Codex MCP: the repo-local plugin starts `tala` over stdio against the workspace `.tala/tala.db`
- HTTP server URL for browser workflows: `http://127.0.0.1:8081`
- HTTP server command for browser workflows: `make own-db`

Use the Tala skill for planning, implementation tracking, progress updates, and
handoff notes. Before reading `.tala/tala.db` directly, check for the
repo-local Tala MCP tools. If the `mcp__tala` tools are not already visible,
use tool discovery/search for "tala issue search" or "tala project tracker
MCP", then use `mcp__tala.issue_search`, `mcp__tala.issue_get`, and related
`mcp__tala` tools. Only read `.tala/tala.db` directly for diagnostics or
verification when MCP tools are unavailable. Do not use another Tala database
unless the user explicitly requests it.

Before finishing any work that creates temporary issues, tags, comments, or
other test records in this repo's `.tala/tala.db`, clean them up or document why they
must remain. This applies especially to browser smoke tests that create
records with names such as `browser_smoke_*`, `detail-smoke-*`,
`profile-smoke-*`, `overflow-*`, or other run-specific markers. Run a post-check
against `.tala/tala.db` to confirm temporary entries are gone, while preserving real
project issues, roadmap items, and durable bug/task records.

## Product Design References

The technical design document is the source of truth for the issue tracker:

- `docs/issue-tracker-design.md`

The Google Stitch UI/UX mockups and design-system assets live under:

- `.stitch/DESIGN.md`
- `.stitch/metadata.json`
- `.stitch/designs/`

Stitch project:

- Project title: `Tala Issue Tracker Mobile Mockups`
- Project ID: `2814771326082574657`
- Design system asset: `assets/dfb1f957bbf24691aa401e82e1a35e7d`

When implementing frontend views, align behavior with `docs/issue-tracker-design.md`
and align visual/UI decisions with the Stitch mockups and `.stitch/DESIGN.md`.
