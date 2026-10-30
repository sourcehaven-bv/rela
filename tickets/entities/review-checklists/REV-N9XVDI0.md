---
id: REV-N9XVDI0
type: review-checklist
title: 'Review: False concurrent-edit warning when one user edits a body inline'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: Go tests; no Go changed. Frontend `npm run test:run`: 359 files / 5405 tests pass; contract 6/6)
- [x] ~~Lint clean (`just lint`)~~ (N/A: Go lint; no Go changed. vue-tsc clean; eslint 0 errors on changed files)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: commentlint checks Go comments only)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage floors; no Go changed)

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

**Review Responses:** RR-4TY69BA, RR-7WATKO4, RR-B3F8XF8, RR-BCOTPJH,
RR-C7SO6H2, RR-CR7ZYVP, RR-H9B2J2P, RR-JQUPMZP, RR-KIHV4Q0, RR-M6R8NQ9,
RR-P6CG9Q6, RR-TEFANWN, RR-Z2UW4YA.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- One user never gets a conflict from a late reload: PASS (`EntityDetail.bodyConflict.test.ts`; fails on the original code and with the session's write rule disabled).
- A read never lands older than a completed write, or for another address: PASS (`useEntitySession.prop.test.ts`, each rule's fault caught with a counterexample).
- Another user's revert still merges or conflicts, never silently overwritten: PASS (three revert tests in `useAutoSave.conflicts.test.ts`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; developer notes in frontend/CLAUDE.md)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

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
