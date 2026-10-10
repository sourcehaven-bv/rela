---
id: REV-UGG69B
type: review-checklist
title: 'Review: Face move loses edges when a carry fails on fs/mem'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (only TestChromeStyle_TargetsShippedClasses fails locally: it needs the built SPA embedded; unrelated)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: CI runs the coverage floor gate)

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

**Review Responses:** RR-2H3J4N, RR-QCO6SP, RR-J1AND1, RR-5R0DRE, RR-FE0FV0,
RR-D3DTKG, RR-PKN3T9, RR-0966HW, RR-S86XJL, RR-O378LX, RR-RLQOCO, RR-NP7N4N,
RR-3TRJM5, RR-0XR01C, RR-UGFWZ8. One significant finding is deferred to
BUG-KOTY66.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a failed edge carry is recovered by a re-run on
memstore and fsstore (TestMigrateFace_RerunRecoversAFailedEdgeCarry). PASS:
identity edges stay on the entity
(TestMigrateFace_CarriesContentEdgesAndKeepsIdentityEdges, storetest
ZeroFaceDeleteWithSiblingsKeepsZeroTailEdges on mem, fs, sqlite and postgres).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug, not an enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: bug, not an enhancement)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: runs after the ticket is done, see the note below)

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
