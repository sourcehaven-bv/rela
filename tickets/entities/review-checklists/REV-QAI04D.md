---
id: REV-QAI04D
type: review-checklist
title: 'Review: /relations endpoints leak hidden-neighbor type + edge meta past the read gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (CI green: Test, Postgres Backend, Fuzz, E2E)
- [x] Lint clean (CI green: Lint, Architecture, God-object lint)
- [x] Coverage maintained (dataentry above floor)

## Code Review

- [x] Ran cranky-code-reviewer on PR #1113
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] ~~All significant review-responses addressed~~ (N/A: no significant findings)
- [x] Self-reviewed the diff for unrelated changes (fix + tests only; verified diffstat)

**Review Responses:** RR-78Q9S0 (nit, deferred), RR-QD5BSI (nit, deferred) —
both non-blocking, tied to the existing TODO(BUG-ABXMAV) shared-chokepoint
follow-up.

## Acceptance Verification

- [x] Each acceptance criterion tested: hidden neighbor's id/type/meta dropped on both `/relations` and `/relations/{type}`, both directions; visible neighbor still returned
- [x] Test evidence: `TestACLRelations_HiddenNeighborExcluded` (reproduction, failing-first confirmed) + `TestACLNeighborReadLeakInvariant` (P4 cross-endpoint invariant); demo_c flips to NOT-REPRODUCED

**Acceptance Status:** PASS — reviewer verdict "correct, complete, ship it", no
security false-negative.

## Documentation (enhancements only)

- [x] ~~Docs section~~ (N/A: security bugfix, no user-facing API change)

## Final Checks

- [x] Commit message explains the why (the leak + the mirrored list-path fix)
- [x] No TODOs left unaddressed (the one TODO(BUG-ABXMAV) is an intentional deferred follow-up, tracked via RR-78Q9S0/RR-QD5BSI)
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1113
- [x] All code CI checks pass (Rela Tickets gate clears on this bug reaching done)
- [x] PR URL documented above

**PR:** https://github.com/sourcehaven-bv/rela/pull/1113
