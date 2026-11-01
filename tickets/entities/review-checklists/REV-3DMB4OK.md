---
id: REV-3DMB4OK
type: review-checklist
title: 'Review: Autosave misjudges its own state across saves of one tab'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend: 365 files, 5480 tests; no Go changes)
- [x] Lint clean (`just lint`) (eslint 0 errors; vue-tsc clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage only; no Go changes)

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

**Review Responses:** RR-D4NH3GW, RR-7EKSAZV, RR-IPO2WZ0, RR-SGYXYY2 (significant); RR-9O5L51V, RR-829JUNG, RR-7ZWWK3C, RR-A9Y2172, RR-CQ3NUST (minor); RR-FALVI21 (nit). All addressed. The security review found nothing.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Unsaved value stays on screen, also across a host reload: PASS (unit tests; DynamicForm and EntityDetail tests).
- Hold released when a later save goes through: PASS (unit test; property's held-implies-error check).
- No false conflict with the tab's own write, merged or absorbed: PASS (two unit tests; property).
- Second 412 merges against the last checked version, body and relations: PASS (two unit tests).
- Leave guard asks while a field is held: PASS (unit test).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; frontend/CLAUDE.md rules updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per project owner; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (run after done; see note below)

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
