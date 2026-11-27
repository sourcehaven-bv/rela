---
id: IMPL-Q0Z2TRA
type: implementation-checklist
title: 'Implementation: Relation filter controls match on target id instead of display title'
started: "2026-11-27"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by HTTP-level list API tests and component tests; no browser session run)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `TestV1ListRelationFilter`: incoming and outgoing filters by id, `ne` by id, unknown id gives no rows.
- `TestV1ListRelationFilter_ByID`: two targets named "Done" filter apart by id; the old title value matches both; a rename keeps the id filter.
- `TestV1ListRelationFilterACL`: a hidden neighbor id with `eq` gives no rows; with `ne` gives all rows.
- `EntityTargetSelect.test.ts`: options commit ids, labels are `Title (ID)`, same-titled targets stay apart, a duplicate id is listed once, an old title value shows as given.
- `go test ./internal/dataentry/`, `npm run typecheck`, eslint, `just arch-lint`, golangci-lint and comment-lint all pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
