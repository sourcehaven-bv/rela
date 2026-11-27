---
id: REV-04KF9S6
type: review-checklist
title: 'Review: Natural-language date input'
started: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): frontend `npm run test:run` 7021 pass; no Go change
- [x] Lint clean (`just lint`): eslint 0 errors, typecheck clean, prettier clean on changed files
- [x] ~~Comment lint gate clean~~ (N/A: commentlint checks Go only; no Go change)
- [x] ~~Coverage maintained~~ (N/A: Go coverage only; no Go change. The frontend has no coverage gate)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky and security reviewers
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (RlInlineEdit reformat reverted)

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all five criteria pass (naturalDate, DateWidget and InlinePropertyValue tests; manual check in the browser).
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QGH2KUI <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows done, TKT-UFV01M)

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
