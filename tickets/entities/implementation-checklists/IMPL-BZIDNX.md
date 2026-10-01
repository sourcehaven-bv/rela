---
id: IMPL-BZIDNX
type: implementation-checklist
title: 'Implementation: pgstore iterators hold a pool connection across yield, deadlocking the pool under concurrent listings'
started: "2026-10-01"
completed: "2026-10-01"
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

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the tests assert completion and absence of errors, not interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons in the new tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
<!-- Document what you tested and the results -->

- `storetest` IteratorNesting suite (all backends): four concurrent iterations of each of the seven iterators with a nested `GetRelation` per row, plus a nested call inside a `Tx` iteration. Against the unfixed pgstore all eight subtests fail (seven with `context deadline exceeded`, `InsideTx` immediately); with the fix all pass on pg, sqlite, fs and mem.
- pgstore conformance, graph differential and field-visible suites run with a two-row iterator page so fixtures cross page boundaries. A mutation that drops the final short page makes them fail.
- jobstest `HandlerContextHasDeadline`: fails without the `dispatch` timeout, passes with it.
- End to end: `rela-server-postgres` on a seeded perf database (`pool_max_conns=2`), eight scheduled Lua tasks listing tasks and persons as an editor, with a `leads` role relation that makes redaction check an edge per row. Old build: no run finished and an HTTP list request timed out after 10 s with `context canceled`. Fixed build: all eight runs succeeded in about 3.7 s each and the HTTP probe answered in 34 ms.
- `go test ./...`, the postgres-tagged suites, the sqlite store suite, golangci-lint, arch-lint and comment-lint all pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
