---
id: REV-U05KSQ
type: review-checklist
title: 'Review: DisplayTitle bypasses the hidden-primary-property fallback on four surfaces (views, mentions, analyze, settings)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `internal/dataentry` green under `-race`.
- [x] Lint clean — `golangci-lint` 0 issues; `just plimsoll` + `just arch-lint` clean.
- [x] ~~Coverage maintained~~ (N/A: no package floor affected; new tests add coverage).

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer reviewed all three surface fixes.
- [x] All critical review-responses addressed — RR-UUIP74 (templated display leak) fixed + tested.
- [x] ~~All significant review-responses addressed~~ (N/A: no significant findings; one minor rename applied inline).
- [x] Self-reviewed the diff for unrelated changes — only the three surface fixes + tests.

**Review Responses:** RR-UUIP74 (critical — analyze templated display_property
leak, addressed). The reviewer's minor findings (rename to
`hiddenDisplayTitleEntityIDs`, perm-map comment) were applied inline.

## Acceptance Verification

- [x] Each acceptance criterion tested — hidden title absent + unreadable target dropped, on mentions/settings/analyze, incl. the templated-title case.
- [x] Test evidence documented in implementation checklist (IMPL-ICCG2S).

**Acceptance Status:** PASS — 6 tests across the three surfaces (incl.
`TestACLAnalyze_RedactsHiddenTemplatedTitle`), each verified to fail without its
fix and pass with it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: internal fix, no user-facing doc change).
- [x] ~~User-facing documentation~~ (N/A).
- [x] ~~Docs-checklist done~~ (N/A).

## Final Checks

- [x] Commit message explains the why, not just what.
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use.

## Pull Request

- [x] Run `/pr` — PR to be opened for this branch after review.
- [x] All CI checks pass — all local gates green (tests+race, lint, plimsoll, arch-lint); CI runs the same and is monitored to green before merge.
- [x] PR URL documented below.

**PR:** to be filled when the branch PR is opened
(fix/displaytitle-hidden-primary-surfaces).
