---
id: REV-FYRLMA
type: review-checklist
title: 'Review: Version sweep starves: edits after the first Batch settled rows are never captured'
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (all pass except `cmd/rela-desktop` TestChromeStyle_TargetsShippedClasses, "no stylesheets in the embedded SPA": needs a frontend build, fails identically without this change; store suites also run with -race on postgres and sqlite tags)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages with default, postgres and sqlite tags; arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: stops on the same rela-desktop test; CI evaluates the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (two rounds: the column gate, then the stored-hash redesign)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-JCH2B1, RR-BB01GI, RR-NQWJ6G, RR-OZ0829, RR-R5WUI5,
RR-PWKM1W, RR-WWJCNY, RR-MC03GX, RR-BRNVX9, RR-FI7XAU, RR-N2MXHR (deferred),
RR-IWVFO2, RR-11V2N6, RR-NWTSZH

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Never-captured backlog larger than a batch drains: PASS (SweepBacklog/NeverCapturedBacklogDrains, both backends)
- Edit behind a full batch of captured rows is captured in one tick: PASS (EditBehindAFullBatch, five columns, both backends)
- Unchanged save after force-live purge neither re-captures nor blocks: PASS (UnchangedSaveAfterForceLivePurge)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug; .claude/rules/versioning.md updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist, see TKT-UFV01M)
