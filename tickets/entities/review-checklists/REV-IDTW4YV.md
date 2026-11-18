---
id: REV-IDTW4YV
type: review-checklist
title: 'Review: Audit read API and Changes tab in data-entry'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `just coverage-check` runs the whole suite and passed; the touched packages also pass under `-tags postgres` (except the pile goldens, which this branch does not touch and CI does not run under that tag)
- [x] Lint clean (`just lint`): golangci-lint on the touched packages, 0 issues; eslint, prettier and vue-tsc clean on the changed frontend files; `just arch-lint` and `just plimsoll` clean
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky and security reviewers, plus the automated commit security review
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-MQ6IJTI, RR-N4KH2NE, RR-0SSR2KU, RR-GZGCNGS,
RR-ISL5B1W, RR-HQUMW6H, RR-BK213O9, RR-CLXHVRD, RR-O1RIDOJ, RR-KGVTVLQ,
RR-GN224QL, RR-3J01KKZ, RR-OX3FQN8, RR-XGVU1GS, RR-ZS0S018, RR-PHDYUZ2,
RR-XTXF7VZ, RR-NU92AMI, RR-BP9R3G2, RR-DD8340D

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: sqlaudittest paging on sqlite and postgres; `TestHandleV1Changes` pages with a cursor; bad limits and cursors give 400.
2. PASS: `TestHandleV1Changes` cross-type and deleted-without-history:read are the uniform 404; the address resolves through the history read path.
3. PASS: `TestChangeOpsClassifyEveryOp`; `TestEntityChanges_HidesRecordsNamingHiddenEntities`, `_RenameAwayChecksNewID`, `_OtherFaceLeftOut`, `_StartFence`.
4. PASS: `TestHandleV1Audit` (deleted and denied records, audit-read records); `TestPermitsAuditRead_ClosedSwitch` (no policy and read-only deny).
5. PASS: `TestAuditRoutes_AbsentWithoutLog`; panel tests for absent flags.
6. PASS: `TestQueryBudget_Changes`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RLVY25I

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: opened after done, per TKT-UFV01M

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
