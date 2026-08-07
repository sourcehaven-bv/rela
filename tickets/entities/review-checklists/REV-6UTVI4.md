---
id: REV-6UTVI4
type: review-checklist
title: 'Review: Unify the two entity-ID validators into one enforced rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: small refactor; human review on PR #1272)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] ~~All significant review-responses addressed~~ (N/A: none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None created. The cranky-code-reviewer agent was not
run for this change; review is by the human reviewer on PR #1272. The diff is
4 files and the behaviour change is pinned by 30 test cases plus three fuzz
targets.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. One implementation — PASS. `storeutil.ValidateID` is a 3-line delegation;
   no independent rule remains.
2. Every write path gated — PASS. storetest conformance across backends plus
   the bidirectional fuzz oracle.
3. Leading dash rejected — PASS.
   `TestValidateID/invalid/leading_dash{,_word}`.
4. Conformance passes per backend — PASS. `TestConformance` green in fsstore
   and memstore.
5. No existing project breaks — PASS. 2030 real entities scanned, zero
   rejected.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change; rationale lives in the ValidateID godoc)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

**Docs Checklist:** N/A — internal refactor.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1272
