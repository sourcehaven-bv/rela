---
id: REV-OUCO8W
type: review-checklist
title: 'Review: Add web URLs to MCP entity results'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (internal/mcp, internal/dataentry, internal/cli and cmd pass; `TestChromeStyle_TargetsShippedClasses` needs the built SPA in `internal/dataentry/static`, which a fresh clone lacks)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: new code is covered by new tests; CI runs the check)

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

**Review Responses:** RR-IQUWSY (significant, addressed), RR-X8QW7G (minor,
addressed), RR-IM5I5R (minor, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. HTTP show_entity carries the origin url for the entity and its relation targets: PASS (`TestHTTPHandlerEntityURL`).
2. Faced rows link as `id@face`: PASS (`TestEntityURL`, `TestEntityResultsCarryURL`).
3. No base omits `url`: PASS (`TestEntityResultsCarryURL`, existing summary JSON tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: one paragraph in GUIDE-mcp-server, planned in PLAN-7D520K)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** N/A.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (PR created with `gh pr create`)

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
