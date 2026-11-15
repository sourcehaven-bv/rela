---
id: IMPL-O6DD99B
type: implementation-checklist
title: 'Implementation: Document and dashboard tabs on entity pages'
started: "2026-11-15"
completed: "2026-11-15"
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

- `go test ./internal/dataentryconfig/ ./internal/configedit/ ./internal/apiwire/...`: ok.
- `go test ./internal/dataentry/`: ok (246s). New: `TestEntityDashboard_*` (counts per anchor, gate before
fold for count, breakdown, table and sum, hidden anchor 404 equals the entity
GET 404, bad requests, outgoing scope, empty reach) and
`TestQueryBudget_EntityDashboardCardIsSizeIndependent` (6 reads, rows read by
id).
- `npx vitest run` on DashboardView, PageView and DashboardMatrix tests: 73 passed. `npx vue-tsc -b`: ok.
- e2e `tests/pages.spec.ts`: 10 passed, including the new entity page test (document step renders
FEAT-001, dashboard step counts 1 task, FEAT-003 counts 0, no validation card,
no card link).
- `just arch-lint`: ok. golangci-lint on dataentryconfig, dataentry, apiwire, configedit: clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
