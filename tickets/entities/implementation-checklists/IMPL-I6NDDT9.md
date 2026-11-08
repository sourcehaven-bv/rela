---
id: IMPL-I6NDDT9
type: implementation-checklist
title: 'Implementation: Reconcile endpoint POST /api/v1/_reconcile'
started: "2026-11-07"
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
Built rela-server and ran it on the tickets project (1211 tickets and bugs).
- A three-range request (entity fingerprint over ticket+bug, a bounded list
  over ticket/TKT-VP..ticket/TKT-VQ, a relation fingerprint over implements)
  answered 200 with count 1211, two listed elements with last set, and a
  relation count of 963.
- An undeclared type answered 400 naming it; a cross-origin POST answered 403.
- 30 concurrent requests answered 503 reconcile_busy and then 429
  too_many_requests with Retry-After; sequential requests stayed 200.
- Tests: reconcile_handler_test.go (two principals get different counts and
  fingerprints, no hidden id listed, hidden field does not move the hash,
  fingerprint equals the fold of the list, paging and bounds, relation
  endpoint gating, validation table, range errors, deadline not_run, client
  cancel, gate limits, CSRF), dataentrywire TestServices_ReconcileServed (fs
  and sqlite tags), openapi TestReconcilePath, reconcile text_test.go.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
