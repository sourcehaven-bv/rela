---
id: REV-7WIBXGZ
type: review-checklist
title: 'Review: Direct messages on channels: decide the DM model'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~Lint clean (`just lint`)~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)

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

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~All critical review-responses addressed~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~All significant review-responses addressed~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~Self-reviewed the diff for unrelated changes~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)

**Review Responses:** None: no code to review.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: DEC-42MOZY2 is accepted with date, context and consequences, and decides authorization and data-entry-server.
2. PASS: TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ have descriptions, acceptance criteria and effort, implement FEAT-GWF8YH, affect concepts and form a depends-on chain; the cardinality and properties gates report nothing for them.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs are planned per follow-up ticket; this ticket changes no user-facing behavior)
- [x] ~~User-facing documentation updated~~ (N/A: docs are planned per follow-up ticket; this ticket changes no user-facing behavior)
- [x] ~~Docs-checklist marked as done~~ (N/A: docs are planned per follow-up ticket; this ticket changes no user-facing behavior)

**Docs Checklist:** N/A

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: only ticket data changes; there is no code commit)
- [x] ~~No TODOs or FIXMEs left unaddressed~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)
- [x] ~~Ready for another developer to use~~ (N/A: design-only ticket with no code; the code lands in TKT-V51KJDF, TKT-FOBKUDU, TKT-8YOZ51W and TKT-EONZTMZ)

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the ticket data goes in with the user's next commit; no code PR)

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
