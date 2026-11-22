---
id: REV-GH7SNPL
type: review-checklist
title: 'Review: Flow failure cause, notices and page follow-ups, and a flow runs panel gate'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full vitest; Go tests of the changed packages; the full suite runs in CI)
- [x] Lint clean (eslint, vue-tsc, golangci-lint)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-CZCCEOG, RR-6RRBRXK)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CZCCEOG, RR-6RRBRXK, RR-B9NPWZ3, RR-698LP8F, RR-RKYR2CK

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Failure entry on the timeline: PASS (job_test, flowView.test)
- Banner hidden on its own target: PASS (nextActionTarget.test, NextActionCard.test)
- Empty read-only group left out: PASS (Sidebar.groupAdd.test)
- Notices read with the view: PASS (useNextAction.test, EntityDetail.actions.test)
- Phone columns line up: PASS (e2e mobile-layout)
- Menu bar title: PASS (e2e mobile-layout)
- Checkbox name: PASS (InlinePropertyValue.test)
- Flow runs panel gate: PASS (chrome_test, flows_handler_test, FlowRunsPanel.test)
- Documented: PASS (docs/flows.md, docs/data-entry.md and their guides)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/flows.md, docs/data-entry.md and their guides)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there
