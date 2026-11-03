---
id: REV-K4N61C
type: review-checklist
title: 'Review: Client field ceilings are ignored when no role declares field grants'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./... green except the known local-only cmd/rela-desktop TestChromeStyle failure from the unbuilt frontend)
- [x] Lint clean (`just lint`) (0 issues on all four build tags; arch-lint OK; plimsoll OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage: package and total thresholds PASS, total 84.7%)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-1ZCRPH and RR-4V7SMT addressed; RR-FCZTWT deferred to BUG-UO72G2 because it changes the documented DR-M3 precedence)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-E7VFNK (design), RR-1ZCRPH, RR-FCZTWT, RR-4V7SMT,
RR-DIBDU7, RR-APOTWP, RR-8ON4Q7, RR-9KM3JO, RR-HL6DZI

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: reproduce with a ceiling-only policy (both integration tests fail on develop).
- PASS: the declarative resolver is chosen when a client ceiling hides or redacts a field (TestCeilingE2E_FieldCeilingWithoutRoleFieldGrants, TestPolicy_HasAffordanceGrants_ClientFieldCeiling).
- PASS: a test pins the ceiling under that policy on reads, lists and search (TestCeilingOnlyPolicy_RedactsField).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (docs/webhooks.md: the mapping check reports ceiling-hidden match fields)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; see TKT-UFV01M)
