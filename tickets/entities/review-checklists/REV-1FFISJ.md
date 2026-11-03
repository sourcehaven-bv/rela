---
id: REV-1FFISJ
type: review-checklist
title: 'Review: Data-entry migrations leak a metamodel across rela migrate runs'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (locally `go test -race ./internal/migration/` and `go test -race -count=50 -shuffle=on ./internal/projectsetup/`; the full suite runs in the CI Test job)
- [x] Lint clean (`just lint`) (not run locally; CI Lint passed on 2b2016d07, and the follow-up only edits a test)
- [x] Comment lint gate clean (`just comment-lint`) (CI Comment lint passed on 2b2016d07)
- [x] Coverage maintained (`just coverage-check`) (not run locally; the change removes no tests and adds one, so package floors cannot drop)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (manual critical review of the PR diff instead; one finding, RR-3UK6Y0)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-3UK6Y0 fixed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3UK6Y0

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (bug has no planning checklist; criterion from BUGA-QG1ZH0: a run without a metamodel takes the no-metamodel path after a run with one)
- [x] Test evidence documented in implementation checklist (IMPL-VOR520)

**Acceptance Status:**
- No metamodel leak across runs: PASS (runner_test.go; fails on pre-fix commit 8d77eb1d5).
- projectsetup tests stable under shuffle: PASS (`-count=50 -shuffle=on`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing change)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix, no user-facing change)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (the shared registry is the cause; details are in the bug's 5-whys)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR already open)

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
