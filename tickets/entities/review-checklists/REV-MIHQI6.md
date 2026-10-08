---
id: REV-MIHQI6
type: review-checklist
title: 'Review: Path-scoped agent rules instead of one large CLAUDE.md'
started: "2026-10-03"
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

**Review Responses:** 19 (9 significant, 7 minor, 3 nit), all `addressed`: RR-2SF2L3, RR-475RJL, RR-4OQ6U3, RR-8DW523, RR-BXAZGK, RR-FFLIXZ, RR-K4N41Q, RR-L4XRIX, RR-MAYB1W, RR-OYVTT4, RR-P1D4BQ, RR-PNPFZP, RR-PS10LH, RR-QZDKHO, RR-V4CQ4S, RR-VGN26H, RR-VR5S4T, RR-WK1O10, RR-ZXVQJG.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: scripted line check against origin/develop; the only differences are three intended cross-reference edits.
2. PASS: `TestRulePathsMatchFiles`; a misspelled glob fails it.
3. PASS: `TestGlobRegexp`, `TestListItem`.
4. PASS: `git check-ignore`: nested `.claude/` directories and `.claude/settings.json` are ignored; `.claude/rules/*.md` is not.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore ticket)
- [x] ~~User-facing documentation updated~~ (N/A: chore ticket)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore ticket)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (in progress; see the note below)

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
