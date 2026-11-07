---
id: IMPL-4X4XWPC
type: implementation-checklist
title: 'Implementation: Range fingerprints for types with redacted fields'
started: "2026-11-07"
completed: "2026-11-07"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no user-facing surface yet; the HTTP endpoint is step 5. The flow is tested end to end over memstore with a real acl policy, resolver and gate)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Hidden field change keeps the reader's fingerprint, visible change moves it, role-relation grant on an assigned row moves it: `TestElements_HiddenFieldChangeKeepsFingerprint`.
- Elements equal `reconcile.EntityElement` of the redacted row; List sums to Fingerprint; address order: `TestElements_ElementsAreRedactedViews`.
- Stored path and Go path agree for a reader with no hidden field, over three ranges: `TestElements_GoPathAgreesWithStoredPath`.
- Value under a roll-up name stays out: `TestElements_StoredRollupValueLeftOut`.
- Row cap depends on row count, not on whether hidden values are set; List limit bounds: `TestElements_RowCap`.
- Re-gate at load, and rows deleted between list and load: `TestElements_RegatesLoadedRows`, `TestElements_DropsRowGoneAtLoad`.
- Relations, and refusal for meta-gated relation types: `TestElements_Relations`, `TestElements_RefusesMetaGatedRelation`.
- Fail closed without a bound request: `TestElements_FailsClosedWithoutRequest`.
- Budget: Fingerprint and List each cost ListEntities=2 ListRelations=1 MatchingFaces=1 at 10 and 50 rows and with or without hidden values set: `TestElements_StoreCallBudget`.
- The store is never asked for per-row hidden-field verdicts: `TestElements_NeverAsksStoreWhichRowsHoldHiddenValues`.
- Unfilled and unparsable stored hashes of a stored type are recomputed, for entities and relations: `TestElements_StoredTypeWithUnfilledRows`.
- Faced type with and without a face allowlist: `TestElements_Faces`.
- ParseDigest and RelationMetaGated unit tests. `go test -race` passes on the touched packages.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
