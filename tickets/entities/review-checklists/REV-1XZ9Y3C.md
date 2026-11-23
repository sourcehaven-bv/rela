---
id: REV-1XZ9Y3C
type: review-checklist
title: 'Review: Declarative metric capture: schedule task that appends a dashboard card value as a record'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`: the full run timed out under machine load in packages this change does not touch; per package with -cover: scheduler 86.7% (floor 50), configedit 82.4% (70), dataentry 88.9% (55), cli 66.1% (30))

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

**Review Responses:** design review RR-U55X4FQ, RR-EH5IDUD, RR-PKTE9FY,
RR-76AWXKG, RR-A1W117I, RR-7I2WN08, RR-8SHEV02, RR-L4ZE9SC, RR-V2EYZU8. Code
review RR-LILG17V, RR-K1EAB4H, RR-UDCFC1O (significant, addressed); RR-4YGWLSC,
RR-UR2MQI4, RR-KSX285K, RR-QF30LQF, RR-YB3V9N6, RR-4FQIQ0R (minor, addressed);
RR-8DW6KRP (minor, wont-fix); RR-A6EFGQL, RR-LDQ3HSE, RR-K3NDVUY (nit,
addressed); RR-VBFHW4K, RR-JD6UTYA, RR-YPKMRAM (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Capture task appends a card figure as one record row without Lua: PASS (TestCapture_AppendsTheRoundedFigureAsRunAs; rela-server end-to-end run stored value 2.50, matching /api/v1/_dashboard/card).
- Figure computed as run_as and gated before the fold: PASS (TestCardFigure_CountsOnlyReadableRows, TestCardFigure_HiddenValueFollowsTheBoundPrincipal).
- Bad config refused at load: PASS (TestParseConfig_CaptureRejectsBadShapes, TestValidateCaptures, TestLoadConfig_CaptureCheckedAgainstSchema).
- Failure stores nothing: PASS (TestCapture_FailureStoresNothing, TestCardFigure_Refusals).
- Runs only where cards can be computed: PASS (TestCapture_NoEvaluatorSkipsCaptureTasks, TestRefuseCaptureTasks, manual rela scheduler refusal).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RG9I1L0

## Final Checks

- [x] ~~Commit message explains the why~~ (deferred: nothing is committed yet; the user decides when to commit)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: a PR is opened only when the user asks)

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
