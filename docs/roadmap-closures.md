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
