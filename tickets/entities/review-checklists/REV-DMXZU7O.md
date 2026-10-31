---
id: REV-DMXZU7O
type: review-checklist
title: 'Review: Bump vitest to 5.0.5 for GHSA-p458-7748-mcp3 (GH #1989)'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend: 5480 tests pass; component library: 640 tests pass; no Go code changed, CI runs the full suite)
- [x] Lint clean (`just lint`) (frontend lint 0 errors, typecheck clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed)

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

**Review Responses:** RR-H2NYPIJ (minor, addressed). The reviewer found no
critical or significant issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. osv-scanner reports no findings: PASS (local osv-scanner 2.7.0 run).
2. vitest, @vitest/ui and @vitest/browser at 5.0.5 only; follow-redirects
1.16.2, glob 14.0.3, minimatch 11.0.3: PASS (`npm ls`).
3. Frontend typecheck, lint, tests, build and component library checks and tests: PASS.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: chore)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** N/A.

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
