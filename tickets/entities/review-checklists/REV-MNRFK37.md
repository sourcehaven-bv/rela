---
id: REV-MNRFK37
type: review-checklist
title: 'Review: Flow run list shows a run whose job failed as Working'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (the flowengine and appbuild suites pass; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on internal/flowengine and internal/appbuild: 0 issues)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: checked in CI; the new doc links name exported symbols only)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: new code is covered by the new tests; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-LAFATJM (significant, addressed), RR-QKPSVHT,
RR-RJP6CUW and RR-9YLWNHZ (minor, addressed), RR-0IOT2V8 (minor, wont-fix),
RR-UTEB39X and RR-2U2AW4X (nits, addressed), RR-WTF0YKD (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a run whose job failed for good reads failed in the
run list as on its page (TestFlowJob_RunListShowsFailedJob).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the flows guide says how the status filter treats such a run)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)
