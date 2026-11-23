---
id: REV-39VCVHF
type: review-checklist
title: 'Review: Remaining data-entry UI gaps in New button and column Add and create forms and lists'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (only frontend tests were run: the full vitest run had 3 load flakes that pass alone, and the Go suite was not run because no Go code changed)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed; `just coverage-check` timed out at 600s on appbuild, dataentry and sqlitestore under machine load, so it did not complete; the frontend has no coverage gate)

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

**Review Responses:** RR-8CJE5V8 (deferred) RR-CBV7UU5 (addressed) RR-22KT3NB RR-HVUTOD6 RR-URNV75T (deferred) RR-5CBWMNF (addressed). Review was by the cranky-code-reviewer agent on the merged PR 2512.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** 1 PASS, 2 PASS, 3 PASS, 4 PASS: see the implementation checklist for hand-test evidence and the vitest cases for each.
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs were updated in the same change and a separate docs checklist was not made)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR is opened by this session after the ticket is done)

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
