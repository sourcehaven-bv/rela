---
id: REV-VSRT0B
type: review-checklist
title: 'Review: Store API takes entity.Ref and RelationKey; queries must select their faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Each of #1725, #1726, #1728, #1729, #1730, #1732, #1734, #1735 and #1733 merged
with the Test (race, coverage thresholds), Lint, Comment lint, Architecture,
God-object lint, Postgres Backend, SQLite Backend and E2E jobs green.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 79 linked to TKT-KQXVF7 (design review and per-PR code
review). No critical findings; all 20 significant findings addressed, including
RR-QUXMAF by the section 12 ruling. Minor and nit findings are addressed,
wont-fix or deferred with a reason. Deferred items are tracked in TKT-7IZHP0
(default world: RR-45DZM2, RR-HKVULG, RR-Z6GS2D), TKT-JAD5M9 (RR-W0H7J6) and
TKT-5W4ISW (RR-O7YJGC). `rela-security-reviewer` ran on PR 8.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- storetest on all four backends: PASS (Test, Postgres Backend, SQLite
Backend jobs).
- No implicit zero-face read: PASS (`GetEntity(Ref)`, `RelationKey` on
relation methods, `RunFaceSelectionTests` for `ErrInvalidQuery`).
- Guards: PASS (`internal/archguard` bareref, tailless, faceselect,
directread).
- Planning AC1 to AC10: PASS, evidence in IMPL-83ZDL5.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-PEV4A3

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
