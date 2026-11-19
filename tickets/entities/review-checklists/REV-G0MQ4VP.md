---
id: REV-G0MQ4VP
type: review-checklist
title: 'Review: tenanthost: a request racing the end of a lazy provisioning run starts a second run'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: change adds tests to one package and no new package; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-FXH5PYZ, RR-5F5VN72, RR-NAOSUDY, RR-U75DLX5,
RR-NI5HBG2, RR-ITFXD7M (all addressed; no critical or significant findings)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: TestProvision_LazySingleFlight,
TestProvision_NoRerunAfterSuccess and TestProvision_SuccessHoldEnds pass 1000
runs under -race; the new tests fail without the fix. The tenanthost package
passes under -race; golangci-lint reports 0 issues.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR is opened only after the requester approves)
