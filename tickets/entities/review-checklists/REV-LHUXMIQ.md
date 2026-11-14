---
id: REV-LHUXMIQ
type: review-checklist
title: 'Review: Search time grows linearly with the data on postgres'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide run not allowed for this task; touched packages pass with -race: pgstore with -tags postgres, tools/scalerun, TestGoldenParity default and sqlite)
- [x] ~~Lint clean (`just lint`)~~ (N/A: lint run serially on the changed package only; pgstore with the postgres tag reports 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: needs a repo-wide run; pgstore gained tests and lost none, CI checks the floor)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (RR-4LK6O2A)
- [x] All significant review-responses addressed (RR-LPQD349, RR-K70031Z, RR-JR5CKO5 addressed; RR-SZ2EJKZ deferred to TKT-2EST91Z with measured reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4LK6O2A, RR-SZ2EJKZ, RR-LPQD349, RR-K70031Z,
RR-JR5CKO5, RR-FBANTJ0, RR-DM4HZR8, RR-B97R7TQ, RR-9HRL7YF, RR-NWKEJM6

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Search p50 no longer grows linearly: PASS, 2.8 s to 28-42 ms at scale 5,
ratio 1.9-3.1 (scale run, IMPL-S5WVD39).
- Visibility contract kept: PASS, RunFieldScopeSearchTests and
RunVisibleSearchTests in the pgstore suite.
- EXPLAIN with actual arguments and generic plans: PASS, TestSearchExplain.
- ratio_max tightened: PASS, 8 to 5.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-2M84JRN

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR created after done; CI not awaited per task)

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
