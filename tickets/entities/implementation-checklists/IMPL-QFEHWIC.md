---
id: IMPL-QFEHWIC
type: implementation-checklist
title: 'Implementation: Flows M1b: run store on file, pg and sqlite'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (`newRun`, harness helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no caller until TKT-9IT3LN8; covered by backend suites)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race ./internal/flowrun/... ./internal/sqlexec/... ./internal/channels/...
./internal/flow/... ./internal/flowcfg/... ./internal/sqlitedb/...`: pass.
- Conformance (`flowruntest.RunAll`, 16 cases incl. two-store concurrency)
passes on kvflowrun (MemFS and disk with `filelock`), sqlflowrun over
sqliteflowrun, and pgflowrun against local PostgreSQL with
`RELA_TEST_DATABASE_REQUIRED=1` (no skips). pgchannels suite also passes on the
shared `pgexec` adapter.
- `RELA_TEST_DATABASE_URL=... go test -race ./internal/store/pgstore/`:
pass with migration 43; sqlitedb ladder plus `TestMigrate_v34ToV35FlowRuns`:
pass.
- AC1 to AC5 map to `TestConformance*`, the concurrency cases with
`Harness.Second`, `TestReopen` (kv, sqlite), `TestDamagedDocument`, and the
migration tests. `TestDueUsesIndex` pins the due query plan.
- Builds with `-tags sqlite` and `-tags postgres`; `just arch-lint`,
`just plimsoll`, `just comment-lint` and golangci-lint on the changed packages
are clean.
- No end-to-end run: nothing calls the store yet (TKT-9IT3LN8 wires it).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
