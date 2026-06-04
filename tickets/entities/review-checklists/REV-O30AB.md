---
id: REV-O30AB
type: review-checklist
title: 'Review: PostgreSQL store + search backend with build-flag variants'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -race ./...` → 60 packages OK; pgstore conformance + fuzz + regression tests pass with `-race` against PostgreSQL (local + CI Postgres Backend job).
- [x] Lint clean — `just lint` 0 issues; also clean under `-tags postgres` and `-tags memorybackend`; `just arch-lint` OK.
- [x] Coverage maintained — `just ci` (incl. coverage-check) passes locally; pgstore is covered by the DB-gated postgres CI job (excluded from the default floor with documented reason).

## Code Review

- [x] Ran `/design-review` (go-architect-grounded) AND `/code-review` (cranky-code-reviewer) — two independent passes.
- [x] All critical review-responses addressed — RR-YXFYK (rename search_text), RR-U9RFH (conformance isolation), RR-8M34K (collation ordering).
- [x] All significant review-responses addressed — RR-P13ZK, RR-889RK, RR-LKK6A, RR-E7WNC, RR-WFF7Z, RR-TKNUJ, RR-R7BDG, RR-4SRPV.
- [x] Self-reviewed the diff for unrelated changes — none; the only adjacent touch was fixing a stale "Cobra" → kong reference in CLAUDE.md.

**Review Responses (all addressed / wont-fix, none open):** Design review:
RR-U9RFH, RR-P13ZK, RR-889RK, RR-LKK6A, RR-E7WNC, RR-WFF7Z, RR-9LYNN, RR-VB27Y.
Architecture review: RR-YXFYK (critical), RR-TKNUJ, RR-G4I74 (wont-fix,
documented), RR-9VTJM. Code review: RR-8M34K (critical), RR-R7BDG, RR-4SRPV,
RR-PGTUF.

## Acceptance Verification

**Acceptance Status:**
- AC1 (postgres build, no bleve) — PASS. `go build -tags postgres ./cmd/...` works; CI `go list -deps -tags postgres` → 0 bleve.
- AC2 (conformance + fuzz, -race, real DB) — PASS. CI Postgres Backend job (postgres:16 service container) green.
- AC3 (search) — PASS. RunSearchTests + pg substring/trgm; byte-order regression test.
- AC4 (default build unchanged) — PASS. 60 packages green; default deps have 0 pgx.
- AC5 (server/CLI against DB) — PASS. postgres CLI e2e (create→list→verify rows), `rela db migrate`/`status`.
- AC6 (4 binaries) — PASS. `goreleaser build --snapshot` produces rela, rela-server, rela-postgres, rela-server-postgres.

## Documentation (enhancement)

- [x] User-facing documentation updated — `docs/postgres-backend.md` (deployment guide incl. DSN, migrations, `rela db` commands, metamodel-on-disk) + CLAUDE.md backend/build-tag section; regenerated from `docs-project`, `just docs-check` clean.
- [x] ~~Separate docs-checklist~~ (N/A: docs authored inline as part of the change; `docs-check` gates them in CI).

## Final Checks

- [x] Commit messages explain the why (5 feature commits + review-fix commits, each describing rationale).
- [x] No TODOs/FIXMEs left unaddressed.
- [x] Ready for another developer to use — `just test-postgres` + the deployment guide.

## Pull Request

- [x] Ran `/pr` — PR created, CI monitored.
- [x] CI checks pass — Test, Lint, Architecture, Fuzz, Frontend, CodeQL, Vulnerability Check, and the new **Postgres Backend** job all green (Rela Tickets gate clears once this checklist + ticket reach done).

**PR:** https://github.com/sourcehaven-bv/rela/pull/893
