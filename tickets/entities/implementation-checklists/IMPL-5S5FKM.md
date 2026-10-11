---
id: IMPL-5S5FKM
type: implementation-checklist
title: 'Implementation: Search returns hits whose only match is a visible:-hidden field on the bleve backend'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestMatchedFields_AgreesWithTheIndex, TestMatchedFields_IDTokens)
- [x] Integration tests written (test full flow, not just units) (TestSearchVisible_BleveHiddenOnlyMatchDropped: memstore + bleve + SearchVisibleFields)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: pure matcher, no error paths added)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: one-entity fixtures built inline with entity.New)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by the integration test through the visible-search seam)
- [x] Each acceptance criterion verified with test scenario from planning (all three tests fail before the fix and pass after)
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are table cases checked against the real index)

**Verification Evidence:**
Ported tests fail on origin/develop (8 matcher cases plus the hidden-only search case) and pass with the fix. go test ./internal/search/... ./internal/dataentry/... ./internal/visibility/... ./internal/store/... green.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
