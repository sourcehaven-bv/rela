---
id: REV-OJELUP5
type: review-checklist
title: 'Review: Body task counts as a roll-up (rollup body_tasks)'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Tests and lint were run on the changed packages (rollup, metamodel, visibility,
dataentry, appbuild, mcp, archguard); CI runs the full suite and the coverage
floors. Comment lint gate: no findings.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-HGBSWRR, RR-1GFHGJV (significant, addressed);
RR-3Q1SV5A (minor, addressed); RR-1F3LJ7Y, RR-5PECHIO (minor, deferred);
RR-F73MJLK (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** declaration shape PASS (TestParse_BodyTasksShape,
TestCompile_BodyTasks); parser PASS (TestCountTasks); visibility PASS
(TestRollups_BodyTasks, TestBodyTasks_ListColumn); budget PASS
(TestRollups_BodyTasksCostIsScaleFree); filter/sort refusal PASS
(TestBodyTasks_ListColumn); docs PASS (GUIDE-metamodel).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-EL0MMYS

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
