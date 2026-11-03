---
id: REV-6PQG6I
type: review-checklist
title: 'Review: sqlite short-needle search folds case for ASCII only'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`). Only the known `TestChromeStyle_TargetsShippedClasses` fails; the sqlite-tagged suite passes.
- [x] Lint clean (`just lint`) on all four tag sets.
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent). Ran the cranky-code-reviewer on dc3fdc096; no critical or significant findings.
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-49D3KF (minor, deferred), RR-AG4B89 (minor,
addressed), RR-5JETUG (minor, addressed), RR-C7XPVH (nit, addressed), RR-191L6G
(nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist). Bug: the fix plan in BUGA-KUGLRL.
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a hidden-field match is not admitted for a short non-ASCII needle
  (`BackendFoldsLikeGo`, "é").
- PASS: the same for a long needle (`BackendFoldsLikeGo`, "ԩab").
- PASS: an upgraded v18 database is reindexed, including a row restored from
  soft delete (`TestSearch_MigrationRebuildsIndex`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix with no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the branch is not pushed yet; the PR covers the whole architecture arc)

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
