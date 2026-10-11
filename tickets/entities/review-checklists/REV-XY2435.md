---
id: REV-XY2435
type: review-checklist
title: 'Review: Live views 3/3: MCP watch tool'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (affected suites under -race: mcp, appbuild, cli, rela-server, dataentry; watch tests 10x under -race)
- [x] Lint clean (`just lint`, all build tags; arch-lint and plimsoll clean)
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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Security: RR-RK0R8C (significant), RR-01UDEW, RR-R4JHXQ,
RR-EC09W9 (minor), RR-D5Z2LU (nit). Cranky: RR-J6T9MV, RR-4523DU, RR-L92LME
(significant), RR-E911EP, RR-LPBH01, RR-O4Y9UA, RR-6GXZJX (minor), RR-162C6Y,
RR-F10YYO (nit). All addressed. The cranky review's principal_property finding
is recorded once, as RR-R4JHXQ.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-Q7VPH3)

**Acceptance Status:** all PASS, in `internal/mcp/tools_watch_test.go`. AC1
TestWatch_NoCursorReturnsBaseline; AC2 TestWatch_BlocksUntilChange; AC3
TestWatch_HiddenChangesDoNotWake (entity, list and relation targets); AC4
TestWatch_TimeoutBaselinesNewTargets; AC5
TestWatch_ChangeBetweenCallsReportedAtOnce; AC6 TestWatch_InvalidInput; AC7
TestWatch_PerIdentityLimit and TestWatchHandler_ServerLimit; AC8
TestWatch_CancelReleasesSubscription; AC9 TestWatch_AccessResolvedEveryPass,
plus TestMCPWorldCheck_UsesRequestOnContext for world grants. Manual stdio run
recorded in IMPL-Q7VPH3 (repeated after the review fixes).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QPJU9Z

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; the user commits on request)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR covers the whole live-views arc and is opened when the user asks)

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
