---
id: REV-UOI88HQ
type: review-checklist
title: 'Review: Replace neoq with River on postgres and an in-house memory queue'
started: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (pre-commit `go test -race ./...` passed; postgres jobs suite passed against PostgreSQL 18)
- [x] Lint clean (`just lint`) (default and `postgres` tags; `just arch-lint` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (85.8% total)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer on the full diff)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3GFW5P4, RR-44K4WUY, RR-53AS465, RR-645I1M7, RR-AD1RQOD, RR-BIITO81, RR-GOK4Y48, RR-H5ZXU68, RR-HZDLOBK, RR-IUT2SX3, RR-J62YPKF, RR-K6FC0ML, RR-OMWKW8V, RR-QKZY3SH, RR-SL6OWBQ, RR-SPUJA8O, RR-WLBEIAZ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `TestMemoryQueue_Conformance` and `TestPostgresQueue_Conformance`.
2. PASS: `TestMemoryQueue_CloseReleasesGoroutines`, `TestNewInMemory_CloseStopsEveryGoroutine`.
3. PASS: `TestPostgresQueue_SchemaPinnedDSN`.
4. PASS: `TestPostgresQueue_SurvivesAnotherProcessClosing`.
5. PASS: `TestPostgresQueue_MovesNeoqJobs`, `TestPostgresQueue_NeoqCutoverWaitsForLockedJobs`.
6. PASS: `go list -m all` and `go.mod` show no neoq module or `replace`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore ticket)
- [x] User-facing documentation updated (GUIDE-postgres-backend: job queue tables and the neoq upgrade)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore ticket)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (minimal messages by preference; the ticket records the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: done after the ticket closes; see the note below)

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
