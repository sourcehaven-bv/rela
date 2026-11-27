---
id: REV-GURWSLH
type: review-checklist
title: 'Review: Extract next-action handler from dataentry.App'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Lint clean (`just lint`)~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)

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

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~All critical review-responses addressed~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~All significant review-responses addressed~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Self-reviewed the diff for unrelated changes~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] ~~Each acceptance criterion tested (reference planning checklist)~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~User-facing documentation updated~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Docs-checklist marked as done~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~No TODOs or FIXMEs left unaddressed~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)
- [x] ~~Ready for another developer to use~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: retroactive closure; the work shipped in #2493 before this item was tracked)

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
