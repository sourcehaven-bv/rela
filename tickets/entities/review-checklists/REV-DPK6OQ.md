---
id: REV-DPK6OQ
type: review-checklist
title: 'Review: Automation action that enqueues a Lua script as a background job'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): default, sqlite and postgres builds pass. The only local failure is TestChromeStyle_TargetsShippedClasses in cmd/rela-desktop, which needs the built SPA; CI builds it.
- [x] Lint clean (`just lint`): 0 issues; `just arch-lint` OK (automation may now use canonical); plimsoll OK.
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): local run stops at the desktop SPA test above; CI enforces the floors.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design review: RR-6SU18P, RR-KCD05H, RR-LE7AMY, RR-LBYZB1,
RR-HRSM1I, RR-MCEU1Y, RR-OVRN8W, RR-W3N0OE, RR-NUBHXP, RR-S85IXN. Code and
security review: RR-UVTJ3W and 19 more linked via has-review-response. Two minor
findings are deferred with reasons (tenant opener options, MCP reload options);
all others are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (runs after the save, in queue or foreground mode): PASS. TestBackgroundAction_QueueMode, TestBackgroundAction_ForegroundByDefault.
- AC2 (saves coalesce): PASS. Idempotency key plus TestAutomationJobs_SaveDuringRunRunsAgain and TestAutomationJobs_CollapsedSaveIsFollowedUp.
- AC3 (identity, audit, grants): PASS. TestAutomationJobs_ForegroundIdentity, TestBackgroundAction_IdentityNeedsGrants, TestBackgroundAction_IdentityWithGrants.
- AC4 (payload selects only): PASS. TestAutomationJobs_PayloadSelectsOnly.
- AC5 (own write runs once): PASS. TestAutomationJobs_OwnWriteDoesNotReschedule, TestAutomationJobs_ForegroundJobsTriggeringEachOther.
- AC6 (retry): PASS. TestAutomationJobs_Retry, TestAutomationJobs_PayloadRoundTrip.
- AC7 (load validation): PASS. TestValidateBackgroundActions, TestJobRetry_Unmarshal.
- AC9 (edit during a run): PASS. TestAutomationJobs_SaveDuringRunRunsAgain.
- AC10 (rename): PASS. TestBackgroundAction_RenameRetriggers, TestBackgroundAction_RenameReschedulesPendingCreate, TestBackgroundAction_RenameDecidesOnStoredValues.
- AC11 (A→B→A stops at 8): PASS. TestAutomationJobs_QueuedChainStops, TestAutomationJobs_HopLimit.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D84SP5

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
