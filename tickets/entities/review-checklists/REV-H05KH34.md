---
id: REV-H05KH34
type: review-checklist
title: 'Review: Erase deleted-entity state from automation job payloads'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: race tests of appbuild, entitymanager, jobs and cli, plus the postgres queue tests through a schema-pinned DSN; CI runs the full suite
- [x] Lint clean: golangci-lint on the changed packages (default and postgres tag), arch-lint, plimsoll
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained: new code has table-driven tests; CI enforces the floors

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

- [x] Ran cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed, except RR-9LOXI2N, deferred with reason
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OAPQ1HH, RR-F9KMYG0, RR-KJ724BT, RR-SGWDJ0W,
RR-07EJTC1, RR-D9BC823, RR-35AE3JW, RR-MD39ZBI, RR-9LOXI2N, RR-P1JQINL,
RR-LLSWFI2, RR-1WWC0S8, RR-X2FOAL9, RR-5K81T7U, RR-4CCTJ5C, and the
defense-in-depth nit

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PASS: no clear state in the payload (TestDeletedState_PayloadHoldsNoClearState)
- PASS: state unreadable after the job ends (TestDeletedState_ErasedWhenJobEnds)
- PASS: state unreadable after a purge or history-purge (TestDeletedState_ErasedOnPurge, _PurgeWithoutIndexEntry)
- PASS: finished postgres rows swept (TestPostgresQueue_RetentionSweep, _RetentionSweepBacklog)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-96LCFRD

## Final Checks

- [x] ~~Commit message explains the why~~ (N/A: the user asks for minimal commit messages; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR opened after the ticket is done

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
