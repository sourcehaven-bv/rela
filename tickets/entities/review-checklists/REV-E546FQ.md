---
id: REV-E546FQ
type: review-checklist
title: 'Review: Sync PUT authorizes UPDATE against caller-supplied type, not stored type (cross-type privilege escalation)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (go test -race across cli/entitymanager/dataentry/store green; CI green)
- [x] Lint clean (golangci-lint 0 issues; arch-lint OK)
- [x] Coverage maintained (above floors)

## Code Review

- [x] Ran cranky-code-reviewer twice: initial fix, then the no-upsert rework
- [x] All critical review-responses addressed (N/A: none)
- [x] All significant review-responses addressed: RR-K995SY (CLI 409 handling) — fixed in c95947da
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-K995SY (significant, addressed), RR-VKJX4B (minor,
addressed), RR-PG8YYO (nit, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested: sync PUT authorizes+validates against STORED type; cross-type update rejected 422; upsert primitive removed (create rejects-on-conflict 409, update direct); cascade WriteRelation no-op proven safe (property-less edges); CLI halts one record on 409/412, run continues
- [x] Test evidence: TestApplyEntity_RejectsTypeChangeOnUpdate, TestSyncPut_RejectsTypeChangeOnUpdate, TestWriteSubjectTypeInvariant (P4 = AM-acl-write-subject-type-invariant), TestApplyEntity_CreateConflict_RejectsAndDoesNotClobber, TestPush_CreateConflict409_HaltsOneRecordAndRunContinues, TestSyncPut_UpdateVanishedReturns412. demo_b flips to NOT-REPRODUCED

**Acceptance Status:** PASS — re-review verdict "BUG-ZWTDH9 fully closed;
cascade no-op proven safe; upsert eliminated". The one significant finding (CLI
409) was fixed and re-tested.

## Documentation (enhancements only)

- [x] ~~Docs section~~ (N/A: security bugfix; internal godoc updated on autocascade.Host.WriteRelation)

## Final Checks

- [x] Commit message explains the why (stored-type binding; upsert removal rationale; 409 client handling)
- [x] No TODOs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1114
- [x] All code CI checks pass (Rela Tickets gate clears on this bug reaching done)
- [x] PR URL documented above

**PR:** https://github.com/sourcehaven-bv/rela/pull/1114

## Follow-up noted

The review flagged a separate `internal/rename/rename.go` with its own
package-local `upsertEntity`/`upsertRelation` — out of scope here, worth a
follow-up bug to check whether it shares the same unsound create-then-update
pattern.
