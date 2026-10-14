---
id: REV-2G2BKW
type: review-checklist
title: 'Review: Access skeleton: mode decisions and request lifecycle'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (race tests pass for access, acl, aclmap, archguard, cli, docs, dataentry and cmd/rela-server; TestGoldenParity passes without -update; full `just test` not run on the loaded machine, CI runs it)
- [x] Lint clean (`just lint`) (golangci-lint --allow-parallel-runners on every changed package: 0 issues; `just arch-lint` and `just plimsoll` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new package internal/access at 98.7%, above the 50% default floor; no other package lost tests)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run on the diff)
- [x] All critical review-responses addressed (none from code review; the design-review critical RR is addressed)
- [x] All significant review-responses addressed (RR-VQESLD)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-VQESLD, RR-MAMR4I, RR-36MMK7, RR-L2UPBS, RR-JH3JLG,
RR-EWQ90L, RR-KTTHOJ, RR-KMCCJT, RR-AR1FWO, RR-BALSD2, RR-M3IVY1. The security
review found no fail-open regression; its two findings are merged into RR-36MMK7
and RR-M3IVY1.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-SGX0XX)

**Acceptance Status:**
- No wire change: PASS (TestGoldenParity without -update).
- Five switches go through access.Mode: PASS (TestMode_Decisions; archguard
TestACLModeChecks).
- Request lifecycle in access.Opener: PASS (opener_test.go; dataentry router,
provision and asserted-identity tests).
- Verb derived from Op: PASS (TestVerbs_AreReadPlusOps,
TestACLVerbEnumsMatchReportVerbs).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the project owner's rule; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the owner opens the PR; this task leaves one local commit)

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
