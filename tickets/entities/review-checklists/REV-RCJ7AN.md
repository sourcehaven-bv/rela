---
id: REV-RCJ7AN
type: review-checklist
title: 'Review: Detail panel goes stale after a relation edit elsewhere'
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

**Review Responses:** 2 findings (2 minor). All addressed except where marked.
- RR-JZ6VC3 (R1-10, minor): addressed
- RR-6Z5ZRC (R2-8, minor): addressed

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Detail panel follows a relation write elsewhere: PASS (demo drag; EntityDetail.events.test.ts).
- Relation events reach both end types: PASS (TestRelationEndTypes).
- No reload storm: PASS (debounce and max-wait tests).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing change)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs checklist for this bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (the coordinator opens the PR for the whole branch)
