---
id: REV-YVP0AK
type: review-checklist
title: 'Review: govulncheck auto-update picks wrong Go minor and duplicates issues'
started: "2026-10-25"
completed: "2026-10-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (only TestChromeStyle_TargetsShippedClasses fails locally: unbuilt frontend, known local-only artifact)
- [x] Lint clean (`just lint`)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-69EQUL, RR-9Y2DYS, RR-RTFGRI, RR-10J4QZ (minor);
RR-YB6LGA, RR-EHCC6U, RR-79L4RF, RR-HOFBBH (nit). All addressed. No critical or
significant findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- GO-2026-6809 cleared: PASS (`just govulncheck` clean on go1.26.10).
- Fix picker chooses the fix for the version in use: PASS (real JSON gives
`stdlib 1.26.10`; govulncheck-fixable-test.sh 10/10).
- Dedup finds the existing issue: PASS (query returns #1986).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
