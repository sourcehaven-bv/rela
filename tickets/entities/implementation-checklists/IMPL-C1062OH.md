---
id: IMPL-C1062OH
type: implementation-checklist
title: 'Implementation: Reconcile client and rela reconcile diff command'
started: "2026-11-08"
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

Two copies of the tickets project. On the peer copy one ticket was deleted and
one edited. The peer ran as `rela-server` on 127.0.0.1:18765.

- `rela --project=local reconcile diff --remote http://127.0.0.1:18765 --types ticket,feature,implements`
  printed the deleted ticket and its `implements` relation under "Only here",
  the edited ticket under "Different", and "4 requests to the remote". Exit 0.
- `-o json --exit-code` printed the same result as JSON and exited 1.
- `--remote http://rela.example` was refused with "refusing plain http to
  rela.example; use https" before any request.

Automated: `internal/dataentry/reconcile_client_test.go` runs the real handler
over two memstores (identical, overlapping, differing hash, hidden field, fewer
rows, paging, wrong token, missing identity). `internal/reconclient` tests
cover too_large, not_run, 429, malformed answers, meta gating, hostile remotes
and the round-trip budget.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
