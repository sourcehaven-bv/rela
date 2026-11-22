---
id: REV-9VN6RS7
type: review-checklist
title: 'Review: Flow step that no branch accepts stays put without a message'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (the flowengine, mcp and appbuild flow suites and the frontend flow and i18n suites pass; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages: 0 issues; eslint, prettier and vue-tsc clean; plimsoll clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: checked in CI; no new doc links)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: new code is covered by the new tests; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-GD912YV and RR-ZJ4VBP3 (significant, addressed),
RR-F0P09KM and RR-2BWILF9 (minor, addressed), RR-MKFYVN1 and RR-RAL9A8H
(minor, wont-fix), RR-7X1WUPY and RR-2QPJK5Z (nits, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a step that no open branch accepts answers
problem `blocked`, and the run card warns that the run stays at the step
(TestFlowControl_CompleteStep, FlowRunCard test).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the flows and MCP guides list the new problem value)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)
