---
id: REV-JNRB1A2
type: review-checklist
title: 'Review: Record entity and relation audit rows inside the write transaction (fail closed)'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (default, sqlite and postgres suites of entitymanager, appbuild, audit and pgstore pass; the full default suite passed; rela-desktop TestChromeStyle and pg piles goldens fail identically on develop)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages for default, sqlite and postgres tags; arch-lint, plimsoll, archplan)
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR batching and tx ctx fixed; automation property writes deferred to TKT-LM1EOUI because they predate this ticket)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-EX35HSI, RR-J2IWTO9, RR-4M0GM6L, RR-4UTB8MK,
RR-H886EAU, RR-GRC0U9D, RR-60NRN4K, RR-NAKT5JH, RR-UNVHA37, RR-QLPK7XV,
RR-BLZG05Z

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestAuditFailureRollsTheWriteBack (sqlite, postgres), create case.
2. PASS: same test, patch case; delete case added.
3. PASS: same test, Verify rows = 1.
4. PASS: TestCreateBatch_MidBatchFailureRollsBack, TestCreateLinked_InTxErrorRollsBack.
5. PASS: kept-entity tests on fs/memory.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (docs/audit-log.md, docs/content-states.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9IB92XB

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened after done, per TKT-UFV01M; the app monitors CI)

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
