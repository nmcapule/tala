# Issue Tracker Design Audit

Date: 2026-06-14

Source of truth audited: `docs/issue-tracker-design.md`.

Implementation evidence:

- REST routing and handlers: `internal/httpapi/http.go`
- Domain service validation and mutations: `internal/app/service.go`
- SQLite persistence and hydration: `internal/store/store.go`
- MCP tools/resources and stdio transport: `internal/mcp/mcp.go`, `internal/mcp/stdio.go`, `cmd/tala-mcp-stdio`
- Frontend screens and shared components: `web/src/App.tsx`, `web/src/features/`, `web/src/components/common.tsx`
- Verification coverage: `internal/**/*_test.go`, `scripts/smoke.sh`, `scripts/browser-smoke.sh`

## Summary

The current implementation matches the design doc's core product model: a local-first Go and SQLite issue tracker with REST, stdio MCP, and React frontend surfaces over one shared issue model. Issues, tags, comments, parent-child relationships, blocker relationships, Markdown source preservation, image upload links, and computed blocked state are implemented and covered by automated Go tests, REST/MCP smoke coverage, and browser smoke coverage.

The audit found no open P0/P1 runtime gap against the design. The remaining gaps are design drift or follow-up contract decisions and have been filed as lower-priority issues.

## Matched Behavior

| Design area | Current evidence |
| --- | --- |
| Local-first single Go server with SQLite | `cmd/tala`, `internal/store`, `.tala/tala.db` workflow, `make own-db` |
| No real auth; username supplied by UI/header/tool args | Login local storage in frontend, `X-Tala-Username` in REST, required `username` in MCP mutating tools |
| Issue fields and statuses/priorities | `domain.Issue`, `domain.Status`, `domain.Priority` |
| Tags with optional color and case-insensitive uniqueness | `internal/store.Store.CreateTag`, `UpdateTag`, tag tests |
| Append-only Markdown comments | `Service.AddComment`, `Store.AddComment`, comment route/tests |
| Parent and blocker relationships with cycle validation | `Service.SetParent`, `Service.AddBlocker`, store cycle helpers, REST/MCP validation tests |
| Duplicate blocker relationship as no-op success | `INSERT OR IGNORE` plus no-op timestamp coverage |
| No issue delete support | No issue DELETE route; unsupported route tests cover the behavior |
| REST issue, tag, relationship, comment, and image endpoints | `Server.Routes` and `internal/httpapi/http_test.go` |
| Consistent REST error shape | `writeError`, domain `AppError`, REST error tests |
| MCP tools and resources | `tools()`, `resourceTemplates()`, `readResource`, MCP tests and smoke script |
| Markdown source preservation | REST and MCP exactness tests; frontend renders through `react-markdown` and `rehype-sanitize` |
| Board, detail, hierarchy, and blocker frontend workflows | `web/src/features/*`, `scripts/browser-smoke.sh` |
| Production static frontend serving | Vite output under `cmd/tala/static`, release checklist, production binary verification script |

## Documented Drift

| Drift | Current state | Filed follow-up |
| --- | --- | --- |
| MCP username defaults | Stdio MCP mutating tools require explicit `username`; optional username defaulting remains a future session-design decision. | `issue_807d80b4-5665-4d21-8778-6fef7a286bb3` |
| Frontend styling stack | Design doc names Tailwind CSS and shadcn/ui. Current app uses plain CSS, shared local React components, and lucide icons while following the Stitch visual direction. | `issue_7dc2a2a6-11db-4b19-b078-4c4a884d6969` |
| Store implementation notes | Design doc says `sqlc` generated query methods. Current store uses `database/sql` with hand-written SQL and transaction wrappers. | `issue_3001631b-c709-4f5e-a9c3-ab4173212f02` |

## Coverage Notes

The design doc's testing strategy is materially covered, but not always by the exact test type named in the design:

- Domain, REST, MCP, and store behavior have Go unit/integration tests.
- REST and stdio MCP smoke behavior is covered by `scripts/smoke.sh`.
- Frontend behavior is covered through `scripts/browser-smoke.sh` rather than a dedicated frontend unit-test runner.
- The root `package.json` currently provides `build`, `typecheck`, `dev`, and `preview`; it does not define a Vitest/Testing Library suite.

## Audit Result

The v1 design remains accurate for core behavior. The follow-up issues above should either update the design doc to reflect the implemented v1 contracts or drive implementation changes if those original design choices are still required.
