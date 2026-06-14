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
