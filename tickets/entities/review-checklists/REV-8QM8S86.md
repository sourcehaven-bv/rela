---
id: REV-8QM8S86
type: review-checklist
title: 'Review: Verified fake API and property tests for SPA editing'
started: "2026-10-29"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: Go tests; the diff has no Go. Frontend: `npm run test:run` 353 files / 5319 tests pass; `just contract-fake` passes)
- [x] ~~Lint clean (`just lint`)~~ (N/A: Go lint; the diff has no Go. `npm run typecheck` clean; eslint on changed files: 0 errors)
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

**Review Responses:** RR-ZL35G2Q, RR-MIV014E, RR-U8F6O6X, RR-Y44QXXS,
RR-6VJY4FX, RR-C8Y08X1, RR-FDV7NRG, RR-ONXZKIX, RR-PDK8UPM, RR-95XDRW3,
RR-T0NBN2W, RR-A4Y9RXY, RR-02Z9TYB, RR-1ZMWZH4, RR-0RDPT4V, RR-F9R98EF,
RR-MT57DFL, RR-OPOSN24, RR-7AZTN03.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Differential test passes and fails on mutants (no 412, token ignores value, no `_versions`, no PATCH events): PASS.
- Entity-switch property reproduces the bug unaided and passes after the fix: PASS (`[deliver, deliver, edit, select 1]` on develop; the title property finds the pin bug on develop with `[select 1, deliver, deliver, type, commit]`).
- Deterministic per seed, seed logged on failure: PASS (fast-check prints seed and path; replayed with seed -628858808, path 135:9:12).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: test infrastructure and a bug fix; developer guidance is in frontend/CLAUDE.md)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

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
