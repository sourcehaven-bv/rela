---
id: REV-IRPTB5
type: review-checklist
title: 'Review: MCP tools for running actions'
started: "2026-10-11"
status: done
completed: "2026-10-11"
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): all Go packages pass except `cmd/rela-desktop` `TestChromeStyle_TargetsShippedClasses`, which needs the built SPA (not built in this worktree; unrelated to this change, CI builds it)
- [x] Lint clean (`just lint`): 0 issues; arch-lint and plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): thresholds pass, total 82.7%

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-1GATWK, RR-8LNWD1, RR-IE9AD4 (significant, addressed); RR-4N0I3K, RR-TXL5AH, RR-9TRMRB, RR-7IA7PD, RR-4IAQR3, RR-4RLR97 (minor, addressed); RR-Y706D3, RR-7V3KY5, RR-HEBJFQ (nit, addressed); RR-9W7IA1, RR-CLFLYK (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1-AC9 PASS; evidence per AC in IMPL-BQ22Y2. AC8 changed during implementation (see PLAN-K5DPR1).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-6XR477

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
