---
id: REV-A5K3ZH
type: review-checklist
title: 'Review: Job queue Close returns before running handlers stop; log-capture tests leave log output on a dead buffer'
started: "2026-10-18"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (run by `just coverage-check`; locally only cmd/rela-desktop TestChromeStyle_TargetsShippedClasses fails, because the SPA is not built in this worktree; CI builds it)
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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-X1YR00, RR-PTXIBO, RR-1QB18O)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-X1YR00, RR-PTXIBO, RR-1QB18O, RR-UIRSOY, RR-M0GG6Y,
RR-ZTAQQF, RR-LYTQZL, RR-UTM1MF, RR-9K25HR (addressed); RR-RW2HCO, RR-HQ25E2
(wont-fix, nits). Security review: no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Close waits for running handlers: PASS. `CloseWaitsForRunningHandlers` fails
20 of 20 (memory) and 3 of 3 (postgres) without the fix; passes with it.
- No write into a finished test's capture: PASS. CI-sequence probe 23 of 100 on
develop, 0 of 100 with the fix. `TestCapture_RestoresLogOutput` passes.
- Same pattern elsewhere: PASS. All tests that swapped the default logger use
logtest; forbidigo bans new swaps.

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
