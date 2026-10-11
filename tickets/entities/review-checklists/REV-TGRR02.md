---
id: REV-TGRR02
type: review-checklist
title: 'Review: Gantt: drag bars to move and resize'
started: "2026-10-10"
completed: "2026-10-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
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
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Code review: 6 significant (RR-4HQBQ0, RR-T9IRL0,
RR-D3VFQ4, RR-YA3WLA, RR-OKQ3LF, RR-5JE9KM), 12 minor and 1 nit group, all
addressed with regression tests. Security review: no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** (criteria from PLAN-1N0DKJ)

1. PASS: e2e drag +2 days writes start and end in one PATCH; unit test holds
the preview until the reload resolves.
2. PASS: e2e end-edge drag changes only the end; `shift` clamp tests.
3. PASS: canDrag table (update refused, read-only, redacted, empty); view
test shows no handles when update is refused; one-sided sources never probe.
4. PASS: threshold test, click suppression test, view test that a drag does
not drill, Escape-then-release test.
5. Delivered in TKT-DN0S6O.
6. PASS: 403 and 412 tests report and reload; changed-since-load conflict
test; warnings test.
7. PASS: GUIDE-data-entry "Rescheduling by dragging".

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZHRF8O

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
