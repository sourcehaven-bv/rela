---
id: REV-YAO1DP
type: review-checklist
title: 'Review: Comments name capabilities, not vendor libraries or backends'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (one local-only failure, TestChromeStyle_TargetsShippedClasses, also fails on develop without a built SPA)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: comments only; no statements added or removed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-T7D86B RR-VSLLXC RR-MQLMOS RR-2QF3HK RR-DBHN6P
RR-TV67OM RR-JHIA8E RR-AAHUGG RR-VQO9A9 RR-MHILS5 RR-ILIXEK RR-1JNWA7 RR-DLGL6C
RR-M594RW RR-RZNAWN RR-N2VBK6 RR-WL8AHB RR-PPQZDD (cranky-code-reviewer: 4
significant, 9 minor, 3 nit; rela-security-reviewer: 2 nit; all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Only comments change, plus the rename: PASS (comment-stripped AST comparison).
2. Three builds compile: PASS.
3. Comment gates pass: PASS.
4. No Lowered identifier remains: PASS.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: chore)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
