---
id: REV-AB482V
type: review-checklist
title: 'Review: Rename and delete errors reveal hidden entities and their relations'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./... passes except cmd/rela-desktop TestChromeStyle_TargetsShippedClasses, which needs the built SPA embedded and fails on develop too)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages: 0 issues; the repo-wide run timed out loading packages while other sessions linted)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (entitymanager 84.9%, mcp 78.8%, dataentry 85.6%, all above their floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-A70TGL, RR-NGY3LF, RR-KEFG4D, RR-AOZU1S, RR-HUDHKE,
RR-O13RR9, RR-RIWM94, RR-1XBLRL, RR-6PWLW1, RR-Z97VV2, RR-R5KO11, RR-4Q56DS,
RR-BL45ZV

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Cascade denial over a hidden edge names nothing hidden: PASS (TestCascadeDelete_DenialOverHiddenEdgeNamesNothingHidden, TestCascadeDelete_EdgeFromHiddenFaceNamesNothing, TestCascadeDelete_VisibleDenialWinsOverHidden)
- Rename counts visible relations only: PASS (TestRename_RelationsUpdatedCountsReadableNeighborsOnly, TestRename_RelationsUpdatedSkipsEdgesOfHiddenFaces, TestACL_WriteCounts_OmitHiddenEdges)
- Rename only for id_type manual: PASS (TestRename_RefusedForGeneratedIDs, TestComputeActions_NoRenameForGeneratedIDs, CLI check in IMPL-A6QFPQ)
- Accepted collision bit documented: PASS (docs/acl-security.md; TestRename_OntoHiddenIDRevealsOnlyTheCollision)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; acl-security and cli-reference were still updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; the user decides when)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: runs after done, on the user's request)
