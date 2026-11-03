---
id: REV-QU4AH5U
type: review-checklist
title: 'Review: GET /api/v1/_analyze runs thousands of statements per request on Postgres'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** Code review by cranky-code-reviewer, 13 findings.
Critical: RR-8URBLSF (goldens) and the archguard finding, both addressed.
Significant: RR-SS939U9, RR-XFGYWND, addressed. Minor and nit: RR-FP6KKM0,
RR-EFEFANB, RR-8OLTGRE, RR-JOQYNXK, RR-DZ3QMVG, RR-YZTGR0D, RR-UT9PBBC
addressed; RR-LN4ICA7 and RR-R2G65MJ wont-fix with reasons. Design review
findings are recorded separately (11 addressed, RR-GU22EQZ and RR-Z2T4LL1
deferred).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: 2,037 statements as jvloothuis (limit 2,300; develop 12,058),
response byte-identical to develop (also for wwarendorff).
- AC2 PASS: TestScriptReader_PushdownPrimesLocalRolesUnderPolicyRedactor
(equal at 10 and 50, no GetRelation) and
TestQueryBudget_PushdownOnPrimedCtxAddsNoPrimeReads.
- AC3 PASS: TestQueryBudget_AnalyzeIsSizeIndependent (9 reads) and
TestQueryBudget_AnalyzeWithFieldRulesIsSizeIndependent (26 reads; 122 and 522
without the page prime).
- AC4 PASS: TestAnalyzeValidations_HiddenViolatorTakesNoSlot,
TestACLAnalyze_FiltersHiddenIssues, TestACLAnalyze_RedactsHiddenViolatorTitle,
TestACLAnalyze_RedactsHiddenPrimaryTitle, TestAnalyze_FacedValidationViolators.
- AC5 PASS: TestAnalyzeValidations_CapBoundary (100, 101, across rules, past
one batch).

Automated: just lint, comment-lint, arch-lint, plimsoll, lint-md, coverage-check
(85.6%, all tests), build and docs-check pass.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor, no user-facing change)
- [x] ~~User-facing documentation updated~~ (N/A: refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor)

**Docs Checklist:** N/A (kind=refactor; the response is byte-identical).

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
