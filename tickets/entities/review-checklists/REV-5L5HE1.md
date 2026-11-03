---
id: REV-5L5HE1
type: review-checklist
title: 'Review: Enforce relation cardinality at write time and add an atomic replace operation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just ci`)
- [x] Lint clean (`golangci-lint` 0 issues on changed packages; `vue-tsc` clean)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`, part of `just ci`)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer, two rounds: origin/develop...6f21af1dc and 6f21af1dc..4884f92a7; plus an automated security review of 93f44f514)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 21 findings (2 critical, 8 significant, 8 minor, 3 nit). All addressed except where marked.
- RR-GP31IH (R1-1, critical): addressed
- RR-CRUYKL (R1-2, critical): addressed
- RR-VOCBBC (R1-3, significant): addressed
- RR-L1PU7O (R1-4, significant): addressed
- RR-GSDD5O (R1-5, significant): addressed
- RR-2Z7RT8 (R1-6, significant): addressed
- RR-GACCSA (R1-7, minor): addressed
- RR-04WHGO (R1-8, minor): addressed
- RR-X9AYH3 (R1-9, minor): wont-fix
- RR-J8V6TX (R1-11, nit): addressed
- RR-25W6J3 (R1-12, minor): addressed
- RR-GU8XVZ (SEC, significant): addressed
- RR-J323N8 (R2-1, significant): addressed
- RR-WDUWDE (R2-2, significant): addressed
- RR-BPSZLB (R2-3, significant): addressed
- RR-ZA4KZJ (R2-4, minor): addressed
- RR-1DMTRO (R2-5, minor): addressed
- RR-5DV8B2 (R2-6, minor): addressed
- RR-XLK127 (R2-9, nit): addressed
- RR-EKMV8H (R2-10, nit): addressed
- RR-NH28MV (R2-12, minor): addressed

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 PASS: TestPatchRelations_AddOverMaxOutgoingIs422, TestCreateRelation_OverMaxOutgoingIs422.
- AC2 PASS: TestReplaceRelations_RepointsToExactlyOneEdge, TestReplaceRelations_CreatesWhenRemovedEdgeIsGone, manual re-point on the demo.
- AC3 PASS: TestReplaceRelations_RefusedCreateKeepsOriginalEdge, TestPatchRelations_RepointToMissingTargetKeepsEdge.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-DCCW4U)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DCCW4U

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (the coordinator opens the PR for the whole branch)
