---
id: REV-JGV72W2
type: review-checklist
title: 'Review: Flow run lists show subject ids and unstyled links'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: the changed packages pass, `go test ./internal/appbuild/ ./internal/dataentry/ ./internal/metamodel/` and the frontend flow tests; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on internal/appbuild, internal/dataentry, internal/metamodel and internal/flowview: 0 issues; eslint and vue-tsc clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: adds tests and no new package; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7VD1YOL (significant, addressed), RR-VWZXRC8
(significant, addressed), RR-98KSVQO (minor, addressed), RR-CSGFVEB (minor,
wont-fix), RR-S0AHJZ2 (minor, wont-fix), RR-AO2WDXA, RR-DMF50LO, RR-FBZT7V9,
RR-H4YAPC4 (nits, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: the run list, the task list and the run page name
the subject by its display title (TestFlowControl_SubjectTitle,
FlowLists.test.ts). PASS: a caller who cannot read a title property sees the id
(TestFlowControl_SubjectTitleHidden, TestWholeDisplayTitle). PASS: the subject
links use the app link style (FlowLists.test.ts asserts the flow-list__link
class).

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
