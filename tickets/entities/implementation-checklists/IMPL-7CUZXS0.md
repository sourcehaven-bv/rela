---
id: IMPL-7CUZXS0
type: implementation-checklist
title: 'Implementation: /_search: client cancels are logged as failures and answered 500; measure and reduce per-keystroke search cost'
started: "2026-10-31"
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
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
Local reproduction, seed scale 1, rela-server-postgres against PostgreSQL 18
(see TKT-T2I7P2 "Result"):

- Abort sweep, 32 aborts at 20..640 ms: 30x status=499 canceled=true, 2x 200,
  no WARN, no 500. Before: 1-3 WARN + 500 per sweep.
- Typing simulation at 200 ms key interval: 1 request, 0 aborts (before 8-12
  sent, 7-11 aborted).
- `limit=50` reaches the search: db_ms for "gateway" (bob) 240-390 ms against
  720-1080 ms on the old binary in the same session.
- EXPLAIN of the ranking query: "carol" 206 -> 0.25 ms, "tsk-0367" 244 -> 0.6 ms,
  "gateway" 208 -> 200 ms; the trigram index is used in custom and generic
  plans (TestWorldSearchExplain).
- Same rows and order: pg conformance suite (storetest) passes, including the
  new non-prime-face and face-allowlist cases.
- Frontend: vitest 1205 tests, vue-tsc clean; built into static/v2.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
