---
id: REV-YJP03V7
type: review-checklist
title: 'Review: Two-tab autosave property test misses its out-of-attempts floor at random'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend `npm run test:run`: 394 files, 5974 tests pass; no Go code changed)
- [x] Lint clean (`just lint`) (eslint and prettier clean on the changed file; one warning predates this change)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: commentlint checks Go; no Go code changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test-only frontend change; the frontend has no coverage gate)

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-YLU90IF)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YLU90IF, RR-0O0O8GM, RR-JC9M202, RR-7AJRMUU, RR-2641JW7

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- No seed under the floor: PASS (seeds 1 to 200, minimum 46; 40 seeds rerun after review fixes, minimum 46).
- Floor failure is replayable: PASS (messages name PROP_SEED and PROP_RUNS; PROP_SEED=8, under the floor on the old code, passes).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

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
