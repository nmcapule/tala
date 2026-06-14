---
name: tala-project-tracker
description: Use Tala as a project-local issue tracker for planning, effort management, resumable agent work, and handoff. Trigger when Codex is asked to plan or implement project work, create or update feature/bug/task breakdowns, track progress in a project `.tala/tala.db`, consult a Tala MCP server, maintain parent/child/blocker relationships, add progress comments, upload issue images, or leave resumable checklists for future agents.
---

# Tala Project Tracker

## Core Rule

Use Tala as the durable project work ledger. Before meaningful project work, consult Tala. During work, keep the relevant issue updated. At interruption or handoff, leave a comment that lets another agent resume.

Use a configured Tala MCP server named `tala`. When installed through the repo-local Codex plugin, the plugin starts that server over stdio against the current workspace's `.tala/tala.db`; it does not require `make own-db`. Before reading `.tala/tala.db` directly, check whether the MCP tools are already visible. If they are not, use tool discovery/search for "tala issue search create comment update image upload project tracker MCP" with enough result slots to expose the full Tala tool set.

Expected MCP tools include `mcp__tala.issue_search`, `mcp__tala.issue_get`, `mcp__tala.issue_create`, `mcp__tala.issue_update`, `mcp__tala.issue_comment`, `mcp__tala.image_upload`, and relationship/status helpers such as `issue_set_status`, `issue_set_priority`, `issue_set_parent`, `issue_add_blocker`, and `issue_remove_blocker`. Do not conclude that create/comment/upload tools are unavailable just because an initial low-limit discovery call only returned search/get/status tools.

Only read `.tala/tala.db` directly for diagnostics or verification when MCP tools are unavailable.

## Project Setup

- Default DB: `.tala/tala.db` in the project root.
- Plugin MCP DB override: `TALA_DB`.
- Plugin workspace override: `TALA_WORKSPACE_ROOT`.
- Default username: `TALA_USERNAME` or `agent`.
- For detailed taxonomy, priority rules, and comment templates, read `references/tala-workflow.md`.

## Work Intake

1. Identify the task intent: feature, bug, investigation, release, docs, or tech debt.
2. Search Tala before creating anything.
   - First discover the full Tala MCP tools if needed, then use `mcp__tala.issue_search` with a concise query.
3. Reuse an existing issue when it clearly matches.
4. Create a new issue only when no good match exists. Format filed issue descriptions with `Context`, optional `References`, and `Action Items`; see `references/tala-workflow.md`.
5. For complex work, ensure a parent issue exists and create child issues for independently resumable work. Use nested children only when a child itself needs multiple implementation phases.
6. Use blockers only for real prerequisites that prevent progress.

## UI Evidence

For UI-related bugs and UI-related comment updates, always include one or more uploaded image references in the Tala issue description or comment. UI-related work includes visual regressions, layout/rendering defects, design mismatches, browser QA findings, frontend interaction bugs, and comments that report or compare visible UI state.

- Prefer screenshots captured through `agent-browser` when the state is reproducible in the UI.
- Save screenshots to a local file path, upload with `mcp__tala.image_upload` when available, and paste the returned `markdown` into the next issue description or comment update. Uploading the image alone is not enough; the uploaded image must be embedded in the Tala entry update that reports the UI state.
- Include enough context near the image to make it useful: viewport/device, route or screen, relevant state, and what the image proves.
- If Tala image upload is unavailable, capture or preserve the image locally when possible, add a clear blocker note, and explain what must be uploaded later.
- If no image can be captured, say why in the issue/comment and include the best reproducible UI context instead.

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
