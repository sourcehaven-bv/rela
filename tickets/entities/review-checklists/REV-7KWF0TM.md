---
id: REV-7KWF0TM
type: review-checklist
title: 'Review: Natural-language date entry in date fields'
started: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): frontend vitest 7056/7056; no Go change, CI runs the Go suite
- [x] Lint clean (`just lint`): eslint, vue-tsc and prettier clean on the frontend; no Go change
- [x] ~~Comment lint gate clean~~ (N/A: commentlint checks Go; no Go change)
- [x] ~~Coverage maintained~~ (N/A: Go coverage floors; no Go change)

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

**Review Responses:** RR-SEK0GV5, RR-DY3S20J, RR-M81OQYC (design); RR-59IHVK0, RR-OT6TB7M, RR-3O22Q6G, RR-W6Q385D, RR-RF4N215, RR-N3U3B6H, RR-HGC2BT2, RR-BQNBO2D, RR-CGO7Q1C, RR-SYQT2C1, RR-JHI3UEE (code)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all four planning criteria pass: naturalDate.test.ts (criteria 1, 2, 4), widgets.test.ts phrase box (criterion 3), manual check in IMPL-5JDH2C2.
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-F58WJEU

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: runs after done; GitHub records it)

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
