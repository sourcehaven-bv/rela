---
id: REV-BUDPJFI
type: review-checklist
title: 'Review: The tenant setting-up screen still ignores the app language'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend only: vitest on useShellAppConfig.test.ts and src/i18n; vue-tsc clean; the full suite runs in CI)
- [x] Lint clean (`just lint`) (no Go change; eslint clean and prettier applied to the new files)
- [x] Comment lint gate clean (`just comment-lint`) (no Go change)
- [x] Coverage maintained (`just coverage-check`) (the new composable has direct tests; the frontend has no coverage gate)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer: no critical or significant findings, five minor and nit findings, all addressed)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-MG0LMAS, RR-E317L8O, RR-ZHIW9X2, RR-WPUMNMA, RR-I27VW59

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Shell language and name hold before the config loads: PASS (useShellAppConfig.test.ts, first case, with a Dutch browser)
- Config applies once loaded, also without a language, and through a reload: PASS (second and third cases)
- A test runs the wiring, not only the module: PASS (the tests mount the composable over a store and load locale.ts from the meta tags)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: docs/postgres-backend.md already describes this behaviour; frontend/CLAUDE.md updated for developers)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the owner's rule; the why is in BUG-HZS0DGQ)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (opened with gh pr create after pr-guidance-check)

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
