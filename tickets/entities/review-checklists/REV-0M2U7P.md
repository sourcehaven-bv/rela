---
id: REV-0M2U7P
type: review-checklist
title: 'Review: Authored-span property rows overlap and do not edit inline'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`just ci` exit 0; frontend 3626, library 445, e2e affected specs 33 passed)
- [x] Lint clean (`just lint`) (part of `just ci`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (part of `just ci`)

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
- [x] All significant review-responses addressed (RR-EV1RT5, RR-3IY7G7)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-EV1RT5, RR-3IY7G7 (significant); RR-QUBC4X, RR-V19MGB,
RR-JLQEE0 (minor); RR-F76CSO, RR-JYKMZD, RR-ZPHAOA, RR-XKF4OE (nit). All
addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (bug: fix plan in BUGA-JO8HNK)
- [x] Test evidence documented in implementation checklist (IMPL-IENLGE)

**Acceptance Status:**
- PASS: no label or value runs past its span cell at 1100px and 780px (e2e, fails on the old CSS).
- PASS: Status value visible in a span-2 cell (same test, plus manual check on atlas TASK-N4W5).
- PASS: inline editor keeps its 240px floor at full width and fits a narrow cell (e2e).
- Inline edit on the atlas fields needs `render: input` in atlas config (atlas PR #69), not a rela change.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no behaviour change to document)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (running now)

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
