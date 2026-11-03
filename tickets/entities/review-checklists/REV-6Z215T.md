---
id: REV-6Z215T
type: review-checklist
title: 'Review: Reload schedules and subscriptions without a restart'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (targeted locally: `go test -race ./internal/scheduler/`, appbuild TestSubscription_; full suite runs in CI per the user's load constraint)
- [x] Lint clean (golangci-lint on scheduler, cli, appbuild, rela-desktop: 0 issues; full lint in CI)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (checked in CI)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-F0RSZ9, RR-GPP93S, RR-3KCDVM)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-F0RSZ9, RR-GPP93S, RR-3KCDVM, RR-5ZYB4B (deferred),
RR-1CHQTK, RR-SP324Q, RR-YGWGEA, RR-VF4CRI (wont-fix), RR-ABYHPB (wont-fix),
RR-FCV3K7, RR-Y9XMRB, RR-W01GYR

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- An edit to schedules.yaml or subscriptions.yaml applies on the next tick: PASS (TestWatchConfig_AppliesEdits, TestSubscription_AddedWhileRunning).
- A broken file keeps the running tasks and is logged once: PASS (TestWatchConfig_AppliesEdits, TestWatchConfig_BrokenSubscriptionsKeepSchedulesLive).
- A server started with no tasks picks up a later file: PASS (TestStartBackground_RunsWithoutTasks, TestSubscription_AddedWhileRunning).
- An unchanged config is not republished: PASS (TestWatchConfig_UnchangedKeepsTheConfig).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-XRWDR1

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after the ticket is done)
