---
id: REV-1Z9GMT
type: review-checklist
title: 'Review: Deleting a face with a content-scoped edge is refused: relation check reads the zero face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `just ci` after rebase onto faces-intrinsic; entitymanager and archguard pass; the new tests pass on memstore, fsstore and sqlite
- [x] Lint clean (`just lint`): golangci-lint 0 issues; arch-lint clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): run inside `just ci`; edgeSourceType's branches are covered by TestDelete_CascadeSourceFallback and TestDeleteEntityFace_SourceReadErrorAborts

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-G0GG9L addressed; RR-P12I7P deferred as out of scope, with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-G0GG9L, RR-P12I7P, RR-8RA7WV, RR-WE3IQK, RR-JMCE3S,
RR-1MSSBA, RR-WPW89C. The security reviewer reported no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: deleting a face with a content-scoped edge succeeds
under a sufficient grant (TestDeleteEntityFace_ContentEdgeAuthorizedByItsTail;
e2e faces-backlog face-delete spec). PASS: denied when the edge delete is not
granted, nothing written (TestDeleteEntityFace_ContentEdgeDeniedWritesNothing).
PASS: family delete and faceless-target cascade resolve the source type
(TestDelete_CascadeResolvesEdgeSourceType); existing faceless cascade tests
unchanged.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: restores documented behaviour; no user-facing change to describe)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened against faces-intrinsic after this checklist, per TKT-UFV01M)
