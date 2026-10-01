---
id: REV-S3X9VG
type: review-checklist
title: 'Review: pgstore iterators hold a pool connection across yield, deadlocking the pool under concurrent listings'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; also `just test-postgres` suites with RELA_TEST_DATABASE_REQUIRED=1 and sqlite-tagged store tests)
- [x] Lint clean (`just lint`: 0 issues; `just arch-lint` and `just plimsoll` clean)
- [x] Comment lint gate clean (`just comment-lint`; comment-report findings in touched files are on unchanged paragraphs)
- [x] Coverage maintained (`just coverage-check`: 81.0% total, all floors pass)

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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-P7WRXJ, RR-61SWYT, RR-I835DP, RR-RV3G26, RR-7VMY8U)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-P7WRXJ, RR-61SWYT, RR-I835DP, RR-RV3G26, RR-7VMY8U,
RR-KOXMZH, RR-A7J9SC (wont-fix), RR-E7U48W, RR-O19JKO, RR-05PZGK, RR-K01S8X,
RR-FS4Z60 (wont-fix), RR-KXHTJ1

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-BZIDNX)

**Acceptance Status:**

- PASS: concurrent listers with nested store calls finish on a pool smaller
than the lister count (`IteratorNesting` conformance on pg, sqlite, fs, mem;
failed with `context deadline exceeded` before the fix).
- PASS: nested calls inside a Tx iteration (`IteratorNesting/InsideTx/*`).
- PASS: manual end-to-end on the perf project with `pool_max_conns=2`: eight
concurrent scheduled list tasks succeed in about 3.3 s each, HTTP probe 23 ms
(old build: no task finished, probe timed out).
- PASS: handler context carries a deadline (`HandlerContextHasDeadline`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A here: nothing is committed until the user asks; the commit follows this checklist)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: /pr runs after the bug is done; PR and CI status are recorded on GitHub per TKT-UFV01M)

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
