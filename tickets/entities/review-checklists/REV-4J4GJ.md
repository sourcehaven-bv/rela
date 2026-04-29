---
id: REV-4J4GJ
type: review-checklist
title: 'Review: Show Lua error details for data-entry action failures'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — 542 frontend + Go suite green
- [x] Lint clean (`just lint`) — 0 errors (67+ pre-existing warnings unchanged)
- [x] Coverage maintained (`just coverage-check`) — 74.1% (package floors satisfied)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none flagged)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- RR-7HDS3 (significant): Confirm-modal flow drops trigger element — addressed
- RR-1EY20 (significant): Trigger element detached by row removal — addressed
- RR-1CE17 (significant): Test gap: confirm-modal flow — addressed
- RR-OGRML (significant): Test gap: triggerEl null vs undefined — addressed
- RR-0RLYP (nit): clearAllMocks vs resetAllMocks — addressed
- RR-YXX7C (nit): Inert vi.useFakeTimers in success-path test — addressed
- RR-CDK7C (nit): Triple HTMLElement narrowing — addressed
- RR-CJ41O (minor): Cancelled fetches counted as failed — deferred (pre-existing)
- RR-VZA4X (nit): executeAction has too many responsibilities — deferred (inherited)
- RR-THTVI (nit): Multiple ScriptErrors discarded — deferred (explicit v1 decision)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 | PASS | unit test "opens the script-error dialog when one rejection is a ScriptError" |
| 2 | PASS | same test asserts the count toast still fires |
| 3 | PASS | unit test "shows only the first ScriptError when multiple rejections are script errors" |
| 4 | PASS | unit tests "does not open the dialog when rejections are not ScriptErrors" + "skips ScriptError dispatch for set-only actions" |
| 5 | PASS | unit test "passes triggerEl through to the dialog store for focus restore"; detach guard verified by scriptError.test.ts "dismiss skips focus restore when the trigger has been detached"; confirm-flow plumbing covered by "forwards triggerEl through onRequestConfirm" |
| 6 | PASS | unit test "does not open the dialog on the all-success path"; full test suite green (542/542) |

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: TKT-LR5YC docs already describe ScriptErrorDialog UX; this ticket extends the existing surface)
- [x] ~~User-facing documentation updated~~ (N/A as above)
- [x] ~~Docs-checklist marked as done~~ (N/A as above)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/619
