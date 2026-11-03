---
id: REV-SQFVJHI
type: review-checklist
title: 'Review: SPA edit form saves an edit onto a different entity after in-form navigation'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend: 355 files / 5397 tests; 48 related e2e specs; no Go changes)
- [x] ~~Lint clean (`just lint`)~~ (N/A: Go linter, no Go changes; frontend eslint 0 errors and typecheck clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: scans ./internal and ./cmd, no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage floors; the frontend has no coverage enforcement)

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

**Review Responses:** RR-CE2ZX5D (critical), RR-D9F4UL3, RR-HCXIGJC
(significant), RR-JLBRP03, RR-B6TUJ2L, RR-KUWLS41, RR-OQRIY6W, RR-EQ2KCQV
(minor), RR-JQFF1II, RR-PLD7ZY3, RR-22N4BA6 (nit)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- An edit on X never reaches Y after an in-app switch: PASS (form-entity-switch.spec.ts fails on develop and passes here; DynamicForm.entitySwitch.test.ts).
- The pending edit on X is saved to X: PASS (the same spec polls X's stored value).
- Server state about another entity is refused: PASS (useAutoSave target identity tests).
- EntityDetail toggle, body and title stay on their row: PASS (EntityDetail.world.test.ts).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; frontend/CLAUDE.md rule added)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

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
