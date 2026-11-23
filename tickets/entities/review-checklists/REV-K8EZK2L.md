---
id: REV-K8EZK2L
type: review-checklist
title: 'Review: Flows: step form from a data-entry form instead of writes:'
started: "2026-11-19"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; every package also passes under `-race` in the coverage run. dataentry and sqlitestore hit the 10m timeout under machine load and pass when run alone)
- [x] Lint clean (golangci-lint 0 issues on the changed packages; frontend eslint, prettier and vue-tsc clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check` thresholds pass; total 85.8%)

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
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3IMPOH2, RR-YNSWLUM, RR-BZASHNZ, RR-707GV2F, RR-DWWR4SB
(significant, addressed); RR-JUPGGPX, RR-8UNK9P4, RR-9H2D6B3, RR-0I4QNRC,
RR-TEZ92VI, RR-5221F08 (minor, addressed); RR-VKOH5IE, RR-T7O03SU, RR-KEHU9OU
(nit, addressed); RR-EBGDKO1 (nit, wont-fix: the linter requires the blank
line).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: the appbuild step tests read `waiting.form` and `waiting.fields`
from the run view (`TestFlowControl_StepFormReadPerStep`).
2. PASS: `TestFlowControl_CompleteStep` "property outside the form" answers 422.
3. PASS: `TestResolveStepForms` and `TestCheckFlowsData_StepForms` refuse
missing, wrong-type, wizard, relation, body and bad-property forms.
4. PASS: the flowcfg `load_test.go` table refuses `form:` on final, unwaited
and job states.
5. PASS: `TestCheckTriggerACL_StepFormEdits` and the triggeracl locked-property
cases refuse a trigger or data-entry edit against a locked form field.
6. PASS: `FlowStepForm.test.ts` covers widget, label, help, `visible_when`,
required blocking, defaults, unchanged values, load error and 409.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-C1BB5XB

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; the commit is made on request)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: `/pr` runs after done, on request)

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
