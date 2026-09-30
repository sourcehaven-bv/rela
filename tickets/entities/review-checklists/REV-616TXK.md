---
id: REV-616TXK
type: review-checklist
title: 'Review: View table cells render every typed value as a badge'
started: "2026-09-30"
completed: "2026-09-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend vitest 3626 passed; go test ./internal/dataentry/... ok; Go elsewhere untouched)
- [x] Lint clean (`just lint`) (eslint 0 errors, warning count unchanged; golangci-lint ./internal/dataentry 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new Go branches in propertyToStrings are covered by TestPropertyToStrings; frontend has no coverage gate)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (three rounds)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-QMS200, RR-OQJ5UF, RR-VEZNX1 (critical); RR-NYA037,
RR-VCQQFO, RR-JLQIZO, RR-4C7NF5 (significant); RR-OB4A1W, RR-VDOANN, RR-3JR09P,
RR-XFEMIM, RR-1PO232, RR-F4N9AA (minor); RR-BZDRXP (nit). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (bug Expected section)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Title renders as text (link when configured): PASS (table test; atlas demo PROJ-30ZY)
- Date renders formatted: PASS (table test; demo shows "26 sep 2026")
- Only an enum renders as a badge: PASS (table test; demo status badges)
- Empty cell stays blank: PASS (table test)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no config or API change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1747 updated after this checklist)

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
