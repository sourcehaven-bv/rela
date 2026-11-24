---
id: IMPL-ESWTN80
type: implementation-checklist
title: 'Implementation: Docs: query scopes with not/or lower into the store'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: docs-only change)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: docs-only change)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: docs-only change)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: docs-only change)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: docs-only change)

**Verification Evidence:** `just docs` exit 0; docs/metamodel.md shows the new
paragraph. Claims checked against queryplan.LowerCondition godoc,
dataentry/listpushdown.go lowerViewCondition, and
listcondition_pushdown_test.go.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: docs-only change)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: docs-only change)
- [x] No debug code left behind
