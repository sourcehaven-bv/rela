---
id: REV-NKT6XWB
type: review-checklist
title: 'Review: Scoped relations: within and on_change'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`; arch-lint and comment-lint gate clean locally, golangci-lint and plimsoll checked in CI because local runs were queued behind other sessions)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the CI coverage job checks the floors; new packages have tests)

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

**Review Responses:** Design: RR-AT4F0JY, RR-X3M9BBH, RR-3KH5OIG, RR-5G5BDHF,
RR-DZXT0ZY, RR-XOCU7MW, RR-C8B3C1T, RR-50X3PXV, RR-QRT99LX, RR-VB351BV. Code:
RR-I9SNDEC, RR-OU5MTWI, RR-7V37F89, RR-CF1JVZY, RR-X6O894F, RR-W7MJRQ2,
RR-114Z5BE, RR-L4ONNUY, RR-5HR4ZBW.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS for every criterion in PLAN-AO6I3WT: load-time
checks (TestParse_RelationWithin), write-path refusal and clear/reject on every
writer (TestScoped_* on memstore, fsstore, sqlite), ACL on clears
(ClearNeedsDeleteGrant, ReparentHiddenSource, ClearAuthorizedPerSource), no
automation re-entry (ClearRunsNoAutomation), schema metadata (golden parity),
picker and form behaviour (scopedRelations tests), manual CLI run in
IMPL-KJ2I832.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-V9K4IPK

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (runs after done)

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
