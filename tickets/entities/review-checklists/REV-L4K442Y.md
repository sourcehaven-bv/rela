---
id: REV-L4K442Y
type: review-checklist
title: 'Review: Add a decimal property type'
started: "2026-11-03"
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [ ] All tests pass (`just test`) (not run as a whole: `go test -race` passed on the 23 touched packages, storetest DecimalText passed on sqlite and postgres, `npm run test:run` 5587/5587, `npm run typecheck` clean)
- [x] Lint clean (`just lint`): 0 issues on all four build tags
- [x] Comment lint gate clean (`just comment-lint`)
- [ ] Coverage maintained (`just coverage-check`) (not run)

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

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [ ] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: metamodel TestParse_DecimalOptions.
2. PASS: storetest DecimalText on fs, memory, sqlite, postgres.
3. PASS: filter TestMatchDecimal/TestSortDecimal, predicatefns decimal tests, manual CLI run.
4. PASS: computed TestEvaluate_DecimalIsExact.
5. PASS: vitest decimal, format, summary and widgets tests (not checked in a browser).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CSD6FDM

## Final Checks

- [ ] Commit message explains the why, not just what
- [ ] No TODOs or FIXMEs left unaddressed
- [ ] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

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
