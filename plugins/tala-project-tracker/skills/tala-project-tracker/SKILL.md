---
name: tala-project-tracker
description: Use Tala as a project-local issue tracker for planning, effort management, resumable agent work, and handoff. Trigger when Codex is asked to plan or implement project work, create or update feature/bug/task breakdowns, track progress in a project `.tala/tala.db`, consult a Tala MCP server, maintain parent/child/blocker relationships, add progress comments, or leave resumable checklists for future agents.
---

# Tala Project Tracker

## Core Rule

Use Tala as the durable project work ledger. Before meaningful project work, consult Tala. During work, keep the relevant issue updated. At interruption or handoff, leave a comment that lets another agent resume.

Prefer a configured Tala MCP server named `tala` when available. When installed through the repo-local Codex plugin, the plugin starts that server over stdio against the current workspace's `.tala/tala.db`; it does not require `make own-db`. Before using the helper script or reading `.tala/tala.db` directly, check whether the MCP tools are already visible. If they are not, use tool discovery/search for "tala issue search" or "tala project tracker MCP", then prefer `mcp__tala.issue_search`, `mcp__tala.issue_get`, and related `mcp__tala` tools when available.

Only use `scripts/tala_helper.py` from this skill against the project-local HTTP server if the Tala MCP tools cannot be discovered or fail. Only read `.tala/tala.db` directly for diagnostics, verification, or when both MCP and helper workflows are unavailable.

## Project Setup

- Default DB: `.tala/tala.db` in the project root.
- Plugin MCP DB override: `TALA_DB`.
- Plugin workspace override: `TALA_WORKSPACE_ROOT`.
- Default helper URL: `TALA_URL` or `http://127.0.0.1:8081`.
- Default username: `TALA_USERNAME` or `agent`.
- If using the helper and the HTTP server is not running, start it with the project command when available:
  - `make own-db`
  - otherwise `python <skill>/scripts/tala_helper.py serve`
- For detailed taxonomy, priority rules, and comment templates, read `references/tala-workflow.md`.

## Work Intake

1. Identify the task intent: feature, bug, investigation, release, docs, or tech debt.
2. Search Tala before creating anything.
   - MCP: first discover the Tala MCP tools if needed, then use `mcp__tala.issue_search` with a concise query.
   - Helper fallback: `python <skill>/scripts/tala_helper.py search --q "<query>"`
3. Reuse an existing issue when it clearly matches.
4. Create a new issue only when no good match exists. Format filed issue descriptions with `Context`, optional `References`, and `Action Items`; see `references/tala-workflow.md`.
5. For complex work, ensure a parent issue exists and create child issues for independently resumable work. Use nested children only when a child itself needs multiple implementation phases.
6. Use blockers only for real prerequisites that prevent progress.

## During Implementation

- Mark the active issue `in_progress` before making substantive changes.
- Add a planning comment before implementation when work is non-trivial. Include objective, checklist, risks, assumptions, and verification plan.
- Update or create child issues as understanding improves.
- Add progress comments after meaningful milestones, failed attempts, important decisions, and interruptions.
- Keep comments factual and resumable. Include file paths, commands run, failing symptoms, and next steps when useful.
- Do not mark an issue `completed` until verification appropriate to the issue has passed.

## Handoff

Before final response or context loss, add a handoff comment to the active Tala issue:

- What changed.
- What was verified.
- What remains.
- Current blockers.
- Exact commands or files needed for the next agent.

If the task produced child issues, update their statuses before updating the parent. Leave the parent open while any child or unresolved blocker remains.

## Helper Quick Reference

Use the helper when MCP tools are unavailable or when deterministic shell access is easier:

```bash
python <skill>/scripts/tala_helper.py health
python <skill>/scripts/tala_helper.py planning
python <skill>/scripts/tala_helper.py search --q "release"
python <skill>/scripts/tala_helper.py create --title "Feature: export report" --priority P2 --tag feature --tag roadmap
python <skill>/scripts/tala_helper.py comment --issue-id issue_123 --body-file /tmp/handoff.md
python <skill>/scripts/tala_helper.py set-status --issue-id issue_123 --status in_progress
```

Use `--url`, `--db`, and `--username` to override defaults.
