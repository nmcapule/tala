# Agent Notes

This project is running in an exe.dev VM. Use only documented exe.dev features:

- https://exe.dev/docs.md
- https://exe.dev/docs/proxy.md

Undocumented local endpoints are internal infrastructure and should not be used.

## Project Work Tracking

Always use this repo's own Tala skill as the durable project work ledger:

- Codex plugin: `plugins/tala-project-tracker`
- Skill: `tala-project-tracker` from the plugin
- Database: `tala.db` at the project root
- Codex MCP: the repo-local plugin starts `tala` over stdio against the workspace `tala.db`
- HTTP server URL for browser/helper workflows: `http://127.0.0.1:8081`
- HTTP server command for browser/helper workflows: `make own-db`

Use the Tala skill for planning, implementation tracking, progress updates, and
handoff notes. Prefer a configured Tala MCP server when available. If MCP is
unavailable, fall back to
`plugins/tala-project-tracker/skills/tala-project-tracker/scripts/tala_helper.py`.
Do not use another Tala database unless the user explicitly requests it.

Before finishing any work that creates temporary issues, tags, comments, or
other test records in this repo's `tala.db`, clean them up or document why they
must remain. This applies especially to browser/helper smoke tests that create
records with names such as `browser_smoke_*`, `detail-smoke-*`,
`profile-smoke-*`, `overflow-*`, or other run-specific markers. Run a post-check
against `tala.db` to confirm temporary entries are gone, while preserving real
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
