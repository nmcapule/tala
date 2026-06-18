# Tala v1 Release Checklist

Use this checklist before cutting a v1 release candidate. Tala v1 is local-first, single-binary software backed by SQLite and does not include authentication or authorization, so release readiness depends on correct local defaults, repeatable checks, and clear rollback guidance.

## Release Criteria

- The server binds to loopback by default and the README security model is current.
- The product behavior still matches `docs/issue-tracker-design.md`, including REST, MCP, and frontend workflows.
- Production frontend assets under `cmd/tala/static` are rebuilt from the current React source.
- The Go binary builds and serves the embedded frontend without a separate Vite process.
- REST, stdio MCP, and browser smoke checks pass against a disposable database.
- The project `.tala/tala.db` contains no temporary smoke records.
- All P0 release blockers are completed or explicitly canceled with a rationale.

## Required Checks

For CI candidate runs, record each command separately even when a Make target overlaps another check:

```sh
go test ./...
bun run typecheck
bun run build
go build ./cmd/tala
make verify-production-binary
```

Then run the smoke checks against a disposable database. Start the server in one shell:

```sh
go run ./cmd/tala -addr 127.0.0.1:8081 -db /tmp/tala-v1-candidate.db
```

Run REST and stdio MCP smoke coverage from another shell. Pass the same database path so stdio MCP checks read and mutate the server's disposable database:

```sh
make smoke SMOKE_URL=http://127.0.0.1:8081 TALA_SMOKE_DB=/tmp/tala-v1-candidate.db
```

Run browser smoke coverage against the same disposable database:

```sh
make browser-smoke SMOKE_URL=http://127.0.0.1:8081 TALA_SMOKE_DB=/tmp/tala-v1-candidate.db
```

Run static and unit-level checks:

```sh
make test
```

Build production frontend assets and the Go binary:

```sh
make build
```

Run the server against a disposable database:

```sh
go run ./cmd/tala -addr 127.0.0.1:8081 -db /tmp/tala-v1-candidate.db
```

In another shell, run REST and stdio MCP smoke checks:

```sh
make smoke SMOKE_URL=http://127.0.0.1:8081 TALA_SMOKE_DB=/tmp/tala-v1-candidate.db
```

Run browser smoke checks against the same disposable database. Set `TALA_SMOKE_DB` so the script can clean up records it creates:

```sh
make browser-smoke SMOKE_URL=http://127.0.0.1:8081 TALA_SMOKE_DB=/tmp/tala-v1-candidate.db
```

Verify the production binary serves the embedded app:

```sh
make verify-production-binary
```

## Backlog Gate

Before labeling a release candidate:

- Search Tala for open P0 and P1 issues.
- Confirm every remaining P0 has either been fixed, canceled, or explicitly accepted as a non-blocker.
- Confirm release parent issues describe any unresolved P1 risk that will ship.
- Add a completion comment to each issue resolved during the candidate pass with the commands that verified it.

## Known Risks To Recheck

- Tala usernames are attribution only, not authentication.
- Any public proxy exposure gives network users read and write access to the selected database.
- Browser and REST smoke tests create records; run them against a disposable database or verify cleanup afterward.
- Uploaded images are stored next to the configured database, so database path changes also change upload storage location.
- MCP mutation tools require a `username` argument on this local server.

## Rollback Guidance

Tala does not run migrations outside the local SQLite database, so rollback is repository and database oriented:

- Keep a backup copy of the target SQLite database before testing a release candidate.
- If the binary or embedded frontend regresses, stop the server and run the previous known-good binary or checkout.
- If a candidate polluted a project database with smoke records, remove only run-specific records after confirming their creator, title prefix, or tag prefix.
- If a Codex plugin release regresses, reinstall the previous plugin version or rerun `scripts/release-codex-plugin.sh` from the previous known-good checkout.

## Candidate Notes Template

```markdown
Release candidate:
- Commit:
- Database used for smoke checks:
- make test:
- make build:
- make smoke:
- make browser-smoke:
- Production binary static serving:
- Open P0 issues:
- Accepted P1 risks:
- Rollback artifact or known-good commit:
```
