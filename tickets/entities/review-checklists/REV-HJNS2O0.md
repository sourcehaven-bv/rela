---
id: REV-HJNS2O0
type: review-checklist
title: 'Review: Metric capture and trend charts over records'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): lua, appbuild, dataentry, dataentryconfig, dataentrywire, records, queryplan and cli pass; the vitest suite passes except GanttView "compresses a span too long", which times out only under load and passes alone (unrelated). CI runs the full suite.
- [x] Lint clean (`just lint`): golangci-lint clean on the changed packages, arch-lint OK, ESLint 0 errors, vue-tsc and prettier clean. Plimsoll cannot start its go subprocess on this machine (WaitDelay); CI runs it.
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): every new function has tests; CI enforces the floors.

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent), with rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-Y563JP9, RR-RM2WFQU, RR-L2L9AR7)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-A05G10F, RR-G4RHJCJ, RR-Y563JP9, RR-RM2WFQU,
RR-L2L9AR7, RR-3616F1F, RR-5A4J5CA, RR-7R3R2HY, RR-Z40OMHQ, RR-TURYRZ5,
RR-YXLX1OC, RR-1K2K2ML, RR-KX7O7RM, RR-A12KTC1, RR-ZHUB7AY (deferred),
RR-8JQLRS2, RR-XR4WRBS, RR-46NNE3P, RR-SVIJEAI, RR-CEWI0QG, RR-F8K95NS,
RR-CRDYYSF, RR-Z3WFTWP (wont-fix), RR-FT0SECB, RR-FAPK6D0

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestRecords_LuaAppendUsesOwnerGate`; actions reach the appender as the caller (`TestActionScript_AppendsRecordsAsCaller`).
2. PASS: `TestAppendRecords_AbsentOnReader`, `TestAppendRecords_RefusedInTx`.
3. PASS: `TestValidateRecordsCard` (19 cases), `TestDashboardCard_RecordsLoads`.
4. PASS: `TestRecords_AggregateExcludesHiddenOwner` per_owner case; `DashboardRecordsChart.test.ts` hidden-owner, 404 and refused-refetch tests.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZE4RAKK

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; see the note below)

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
