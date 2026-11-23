---
id: REV-MQC2BNY
type: review-checklist
title: 'Review: Bound the structured search path the way free-text already is'
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): changed packages on default, memorybackend, sqlite and postgres tags; `npm run test:run`
- [x] Lint clean (`just lint`): golangci-lint on changed packages, arch-lint, plimsoll, eslint (only pre-existing max-lines warnings)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: new code is covered by new tests; CI enforces the floors)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design RR-8B4FN7V, RR-GY2RRI9, RR-7MNI7J8; code RR-S5OBY0F
(significant, deferred to TKT-PSBGQLY/TKT-C5H506O), RR-738J1TM (significant,
addressed), RR-E2VU0MO, RR-3CWYDER, RR-T4UWHGL, RR-D01C1JD, RR-IFKGBYU,
RR-MACBCJ5, RR-D0XG82I, RR-TOLQ5DU

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: unlimited structured search returns one page of 1000 with total and has_more (TestQueryBudget_SearchUnlimitedIsBounded, manual run on 9603 rows).
- PASS: callers page instead of relying on everything (DashboardView via searchAllEntities, bridge page param, SearchView notice).
- PASS: totals and flags are post-gate (TestSearchPage_HiddenRowsDoNotShiftPages, differential test with alice).
- PASS: cost pinned (searchLimitBudget) and the searchPlan.page allowlist entry removed.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-332MPBQ

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
