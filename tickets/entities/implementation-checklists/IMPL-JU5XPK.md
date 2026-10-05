---
id: IMPL-JU5XPK
type: implementation-checklist
title: 'Implementation: E2E fixture and test matrix for faces and worlds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the deliverable is e2e test code; it has no unit-testable logic of its own)
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

- `npx playwright test faces-`: 18 passed, 13 skipped (fixme and the postgres-gated history spec). `--repeat-each=3 --retries=0`: 54 passed, no flakes.
- Every fixme body was run with `.fixme` removed: all failed today, which confirms each is a real defect. One test (identity edge on the faceless target) passed and moved to the live browse spec. The family-delete test was moved to POL-2 so the untracked cascade defect does not refuse it for the wrong reason; it then fails as BUG-1YN750 describes.
- Smoke of existing specs (crud, comments, list, entity-detail, relation-cards): 56 passed.
- `npm run lint`, `npm run typecheck` (e2e) and `golangci-lint run` clean.
- New untracked defect found: `DELETE /policies/POL-1@draft` answers 403 "no role grants delete on relations from type \"\"". The cascade authorization in `internal/entitymanager/manager.go` resolves the edge source with `tx.GetEntity(ctx, rel.From)` on the bare id, which a faced type does not store, so FromType is empty and the check fails closed. Fixme spec added.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
