---
id: REV-A6EU739
type: review-checklist
title: 'Review: _nav_status and _nav_items probe local roles per row (slow on atlas postgres)'
started: "2026-10-31"
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

**Review Responses:** RR-5YCSFP6 (significant, addressed), RR-VSC90JL,
RR-0EHQECF, RR-Y9PMCXQ, RR-8GOLX1W, RR-IZH0Y5W (minor, addressed),
RR-YR0K53E, RR-8HQIEIA (nit, addressed), RR-NLN6FQY (nit, wont-fix). The
reviewer found no correctness defect.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 PASS: `TestQueryBudget_NavStatusIsSizeIndependent` (8 reads) and
  `..._NavStatusRuleOnlyIsSizeIndependent` (7) at 10 and 50 rows; 664 and 503
  at 50 rows without the fix.
- AC2 PASS: `TestQueryBudget_NavItemsRedactionIsSizeIndependent` (8, and 12
  with a list condition); 504 and 168 without the fix.
- AC3 PASS: existing nav status, nav items, spaces and dashboard tests pass;
  `TestNavStatus_PrimedCountHonorsLocalRoleVisibility` gives the same count
  with and without the fix, and fails without it only on the per-row read.
- AC4 PASS: 891 to 10 statements on the atlas copy, body byte-identical
  (implementation checklist).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist DOCS-NAVPRM1 created, linked and done; nothing user-facing

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
