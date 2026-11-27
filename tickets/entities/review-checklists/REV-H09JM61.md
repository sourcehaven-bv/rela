---
id: REV-H09JM61
type: review-checklist
title: 'Review: Pre-commit full race run times out on the sqlite packages'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (deferred to CI: local runner overloaded; no Go code changed, the hook was run by hand against internal/principal and internal/calfeed)
- [x] ~~Lint clean (`just lint`)~~ (deferred to CI: local runner overloaded)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed)

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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CI9ZGR5 (significant, addressed), RR-0GZTRB6, RR-3FSJWN3, RR-BGKBE50, RR-6B17EW7, RR-DL1ZWO3, RR-LETMK8U, RR-6EMTQOE (addressed), RR-L9NLZS2, RR-R32CJOI (wont-fix nits)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a ticket-only commit skips lint and tests.
- PASS: only staged packages are tested, with a named timeout.
- PASS: failing packages and failure lines are printed; a timeout names the stuck test.
- PASS: installed hooks run the checked-out scripts.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

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
