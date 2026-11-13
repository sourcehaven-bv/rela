---
id: REV-S5ZZRAJ
type: review-checklist
title: 'Review: Flows M5b: compile automations onto entity-change triggers'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Race tests pass on the changed and consuming packages; golangci-lint is clean on
`internal/automation` and `internal/flow`; `just arch-lint`, `just comment-lint`
and `just plimsoll` are clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Code review (cranky-code-reviewer): RR-L9R82V0, RR-JZJH5K2
and RR-VGVS90C (minor, addressed); RR-H1XG79A and RR-0NXZLN8 (nit, addressed);
RR-OGWC2FH and RR-ZW8X9B0 (nit, wont-fix with reason). Security review
(rela-security-reviewer): no critical or significant findings; its one nit is
the same as RR-JZJH5K2. Design review: RR-YI23YFP, RR-4DEP7S7, RR-2CG178Z,
RR-P7H2HQ4, RR-AAQSM35, RR-P1GUQXB, RR-TURW9FJ and RR-KT5LKCD, all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: every existing test in `internal/automation` and
`internal/autocascade` passes; two fixtures moved to the public construction
path or a recompiling helper, assertions unchanged.
2. PASS: the golden tests recorded before the refactor pass; the synthetic one
differs only by the intended dropped warnings (verified by script).
3. PASS: `internal/flow/fire_test.go`.
4. PASS: race tests on `internal/entitymanager`, `internal/appbuild` and
`internal/dataentry`, plus a CLI smoke run of the tickets checklist workflow.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor, schema syntax unchanged)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor)

**Docs Checklist:** N/A

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
