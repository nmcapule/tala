# Roadmap Closures

This file records closure evidence for high-priority roadmap parent issues. Lower-priority follow-ups may remain open outside these parent scopes.

## Roadmap: planning and dependency model

Tala issue: `issue_652daafb-841d-4060-9d51-1d1a155fbc60`

Closed: 2026-06-14

Closure evidence:

- Parent/child decomposition is implemented in REST, MCP, store hydration, and the hierarchy planning view.
- Blocker relationships are implemented in REST, MCP, store hydration, detail controls, board indicators, and blocker planning view.
- Completed P1 children:
  - `issue_c108c197-961b-4981-84d2-8df144e7f611` Improve hierarchy planning view.
  - `issue_804d164d-92cf-45b5-81ea-17805c1a3835` Improve blocker planning view.
  - `issue_9e67c19c-ea77-4464-843c-be48333c6b85` Render cycle validation errors inline.
  - `issue_517f41aa-4a44-4525-bb25-5b615ad42a03` Separate active and resolved dependencies everywhere.
  - `issue_39280f20-a562-4b32-8f59-662453d638a3` Refresh planning context after out-of-band writes.

Verification evidence:

- `go test ./...`
- `bun run typecheck`
- `bun run build` for frontend planning-view changes
- Mobile browser screenshots attached to the hierarchy and blocker child issues

## Roadmap: MCP agent workflow

Tala issue: `issue_732d87c9-0c7b-4b27-9bdc-1342971fd1d1`

Closed: 2026-06-14

Closure evidence:

- MCP tools expose issue create, update, search, get, comments, relationship mutation, assignment, status, priority, and image upload workflows.
- MCP resources expose compact board, detail, tree, blocker, and planning context.
- Protocol edge cases, structured tool results, resource errors, origin checks, and stdio framing are covered by tests and smoke checks.
- Completed P1 children:
  - `issue_c1550c1a-046c-4238-ad73-fce10f595d3d` Improve MCP planning resource quality.
  - `issue_ff8be893-75e2-4e59-af65-03aae246432d` Tune MCP resource compactness.
  - `issue_cb878880-c149-4386-b91d-8a9fe32e5484` Keep protocol edge cases covered.
  - `issue_dc9ea56c-54b0-46d8-8377-dd4739b57901` Audit MCP tool schemas.

Lower-priority follow-ups moved outside this P1 scope:

- `issue_cfd39c9f-1ec8-4054-b414-93b2f23df441` Reconcile MCP transport and username behavior with design doc.
- `issue_807d80b4-5665-4d21-8778-6fef7a286bb3` Design future MCP session username behavior.

Verification evidence:

- `go test ./internal/mcp`
- `go test ./...`
- `scripts/smoke.sh` MCP coverage referenced by completed child issues

## Roadmap: backend/API hardening

Tala issue: `issue_80be304d-53c9-4155-b7fb-f9e0dcc49287`

Closed: 2026-06-14

Closure evidence:

- REST and MCP app-level errors expose structured code/message/field data.
- REST validation covers create, update, comments, tags, parent, blocker, filters, nulls, wrong types, missing fields, whitespace, unsupported methods, and static-handler fallbacks.
- Search and ordering are stable across title, Markdown, comments, tags, IDs, creator, assignee, status, and priority.
- SQLite migration and local persistence behavior are covered by store tests.
- No-op updates preserve `updated_at` across scalar, tag, parent, blocker, status, priority, and assignee mutations.
- Completed P1 children:
  - `issue_6d9a874a-4e85-4b7b-8fac-02075db1d08a` Normalize JSON error consistency.
  - `issue_c61f6649-fd6c-415e-aad0-fc7b7d71f01f` Complete REST validation matrix.
  - `issue_bcdeba68-9529-4c2d-a006-c11b85427e30` Guarantee deterministic search and ordering.
  - `issue_6fdc9b0d-b03b-4bd0-a1e7-773f03e9c791` Exercise SQLite migration idempotence.
  - `issue_46fc5e39-0f24-4b67-81f9-3301369e7ff7` Protect no-op timestamp guarantees.

Lower-priority follow-up moved outside this P1 scope:

- `issue_f5010ea9-0534-4465-a3b1-bd36575a034e` Harden tag color normalization.

Verification evidence:

- `go test ./internal/app ./internal/httpapi ./internal/store`
- `go test ./...`
- `scripts/smoke.sh` REST coverage referenced by completed child issues

## Roadmap: quality, testing, and release operations

Tala issue: `issue_18f3e881-5408-4941-851c-8ff0d3cbb01f`

Closed: 2026-06-14

Closure evidence:

- REST, MCP, browser UI, production assets, and DB persistence have repeatable verification paths.
- The release checklist documents disposable database usage and smoke cleanup expectations.
- Browser smoke coverage exercises login, board, create, detail editing, Markdown preview and sanitization, relationships, filters, blockers, tags, profile paths, and mobile wrapping.
- REST and MCP smoke coverage exercise validation, filters, relationships, comments, resources, tool success, tool errors, and protocol errors.
- Completed P1 children:
  - `issue_bbad6e6a-bcab-477d-9e56-697013d0f005` Expand browser smoke coverage.
  - `issue_41040cae-ab54-448d-b83f-5719f8ff0dbd` Expand MCP smoke coverage.
  - `issue_db2dcded-474c-4355-a9c3-8bfa588a7065` Draft CI candidate checklist.
  - `issue_81021e6a-cd24-4bf7-8199-4bee7f19b52d` Expand REST smoke coverage.
  - `issue_5f2969cf-4bd1-4a56-8773-a47cc937e1d4` Verify production build output stability.

Lower-priority follow-up moved outside this P1 scope:

- `issue_0069119d-ab23-4218-bd48-99e471dc85e6` Define stable fixture or seed data strategy.

Verification evidence:

- `go test ./...`
- `bun run typecheck`
- `bun run build`
- `scripts/smoke.sh` and `scripts/browser-smoke.sh` coverage referenced by completed child issues

## Roadmap: v1 release readiness

Tala issue: `issue_51784eee-927f-4a97-a3e4-8d77fb832138`

Closed: 2026-06-14

Closure evidence:

- Release criteria, commands, known risks, and rollback guidance are documented in `docs/v1-release-checklist.md`.
- The design audit confirms the implemented behavior matches the v1 design at the P0/P1 level, with non-blocking drift filed as lower-priority follow-ups.
- Known bugs were triaged and no open P0/P1 runtime bug remains in the release scope.
- Production frontend assets and the Go binary static-serving path have been verified by completed child work.
- Completed P0/P1 children:
  - `issue_770bcd71-f530-4e68-aa80-ee9722e18bef` Create v1 release checklist.
  - `issue_78ea1387-a2e4-48eb-8a46-9f4be5dbfb18` Triage known bugs before v1 candidate.
  - `issue_ab6377af-f1f4-4bee-af7f-04f4a829362e` Audit design doc against implemented behavior.
  - `issue_fd973c0a-ea5d-42bd-89cf-39280c172dc4` Verify production binary serves embedded frontend assets.
  - `issue_a69c3631-54b0-4890-8b61-78c7a9554d23` Write operator quickstart and default DB guidance.

Lower-priority follow-up moved outside this P0 scope:

- `issue_c6be910c-4715-4b83-8698-68094cf9cead` Preserve seeded roadmap DB workflow.

Verification evidence:

- `go test ./...`
- `bun run typecheck`
- `bun run build`
- `make verify-production-binary`
- `scripts/smoke.sh` and `scripts/browser-smoke.sh` coverage referenced by completed child issues
