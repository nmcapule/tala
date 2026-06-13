# Agent Notes

This project is running in an exe.dev VM. Use only documented exe.dev features:

- https://exe.dev/docs.md
- https://exe.dev/docs/proxy.md

Undocumented local endpoints are internal infrastructure and should not be used.

## Project Work Tracking

Always use this repo's own Tala skill as the durable project work ledger:

- Skill: `skills/tala-project-tracker`
- Database: `tala.db` at the project root
- Preferred server URL: `http://127.0.0.1:8081`
- Preferred server command: `make own-db`

Use the Tala skill for planning, implementation tracking, progress updates, and
handoff notes. Prefer a configured Tala MCP server when available. If MCP is
unavailable, fall back to `skills/tala-project-tracker/scripts/tala_helper.py`.
Do not use another Tala database unless the user explicitly requests it.

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
