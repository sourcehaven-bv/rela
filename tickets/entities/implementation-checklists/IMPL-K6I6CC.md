---
id: IMPL-K6I6CC
type: implementation-checklist
title: 'Implementation: Guard test forbids zero-face reads outside a shrinking allowlist; faced fixtures by default'
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
- With an empty allowlist the guard failed on every one of the 72 files (119 reads) with the DEC-NPZICR message; with the pinned allowlist it passes.
- `TestScanTree_FindsSyntheticViolation` writes a temporary tree with a new file holding `st.GetEntity(ctx, id)` and asserts the real walker and diff report it, while `_test.go`, `testdata/` and `internal/store/storetest` are skipped.
- `TestDiffAllowlist` covers new file, over count, under count and a vanished entry; `TestCountZeroFaceReads` covers method values, arity exclusions, the literal-empty `GetEntityState` face and `bareEntityID`.
- `internal/dataentry` suite passes: `seedDraftAndPublishedTicket` is deleted; its four callers run on `facedTicketApp` (ticket declares draft/published/review) with rows only at declared faces. Two tests skip with their bug ids (BUG-CTUW2N attachments, BUG-4SYAA6 history).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
