---
id: REV-WRZPUYJ
type: review-checklist
title: 'Review: Finished flow run page hides its subject'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: the changed packages pass, `go test ./internal/appbuild/ ./internal/flowview/...`; the full suite runs in CI)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: adds tests to appbuild and no new package; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5K8XH4C, RR-FTQ9URJ, RR-VJJO281 (one minor and two
nits, all wont-fix with a reason; no critical or significant findings)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a finished run read through Run and SubjectRuns
names its subject for a reader of the subject and hides it from a non-reader
(TestFlowControl_FinishedRunNamesSubject).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; docs/flows.md and GUIDE-flows got one paragraph anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)
