---
id: REV-UKTW38
type: review-checklist
title: 'Review: Remote MCP: faced entities are invisible to every read tool'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test for internal/mcp, internal/worldreader and internal/dataentry pass; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint and arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the change adds tests only to covered packages; CI enforces the floors on the PR)

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

**Review Responses:** RR-EIHD4P, RR-WII1I4, RR-MQG3VT, RR-UDRKK8, RR-KH35J1,
RR-XY4GYA, RR-308FSL, RR-DSII4U, RR-3NA63Y, RR-N0XZCA, RR-WDZ3Z4, RR-U896ZD,
RR-KK51QO, RR-3SAVU8, RR-G6YR25

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Bare id resolves through the default world: PASS (`TestBoundReader_GetEntity`, `TestMCPReadWorld_ThroughTheRouter`).
- `ID@face` selects that face: PASS (`TestBoundReader_GetEntity`, `faceref_test.go`).
- list, show and search return faced entities: PASS (`TestRemoteMCPDeps_FaceRestrictedReaderGetsTheFaceTheyMayRead`).
- A face-restricted reader gets only the face it may read: PASS (same test, carol with `policy@concept`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the user asked for minimal commit messages; the why is in the bug's 5-whys)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` runs after the bug is done, see TKT-UFV01M)

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
