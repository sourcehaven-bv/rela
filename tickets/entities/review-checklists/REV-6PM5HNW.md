---
id: REV-6PM5HNW
type: review-checklist
title: 'Review: Push desktop notification conditions into the store'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (scoped per the task's load limits: `go test -race` on internal/desktopnotify, internal/archguard and the cmd/rela-desktop notifier tests)
- [x] Lint clean (`just lint`) (scoped: golangci-lint on the touched packages)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: not run under the task's load limits; CI enforces the floors and the package gained tests)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OVR1166, RR-CP03XVT, RR-GVU860D (significant,
addressed); RR-250E8F6, RR-NMN18W9 (minor, addressed); RR-4HLJUAT, RR-H5XG8TI
(nit, addressed); RR-EFYUJHE (nit, wont-fix). rela-security-reviewer: no
findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestEvaluate_LoweredConditionIsScaleFree.
2. PASS: TestEvaluate_PrefilteredConditionSkipsExcludedRows.
3. PASS: TestEvaluate_MatchesAndBadge, TestTracker.
4. PASS, revised after review: a rule over the cap reports nothing and restarts silently (TestEvaluate_CapsMatchesPerRule, TestEvaluate_LoweredRuleReadsAtMostTheCap, TestTracker_TruncatedRuleStartsOver).
5. PASS: TestNoNewUnpagedReads with the Config.Evaluate entry removed.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-Y7YCNPC

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create against the scaling branch, as the task asked; CI not awaited)
