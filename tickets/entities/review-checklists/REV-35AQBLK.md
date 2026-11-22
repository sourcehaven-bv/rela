---
id: REV-35AQBLK
type: review-checklist
title: 'Review: Flows panel offers Start without the start permission'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: the changed tests pass, `go test ./internal/appbuild/ ./internal/dataentry/` for the flow tests and the frontend flow tests; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on internal/appbuild, internal/dataentry and internal/flowview: 0 issues; eslint and vue-tsc clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: adds tests and no new package; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** no critical or significant findings. RR-KPG4LGT and
RR-4W5SH73 (minor, addressed), RR-414T397 and RR-MMD0N6B (minor, wont-fix),
RR-CCX47JB, RR-V2A4LNC and RR-6CKL33Z (nits, addressed), RR-MGJQNRR (nit,
wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: the runs of one entity name the flows the caller
may start on it, and Start agrees (TestFlowControl_Startable,
TestFlowsAPI_SubjectRunsStartable). PASS: the panel offers Start only for those
flows, and none when the load fails (FlowRunsPanel.test.ts).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; docs/flows.md and GUIDE-flows name the permission and the `startable` field)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)
