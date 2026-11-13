---
id: REV-LM26AVI
type: review-checklist
title: 'Review: Multi-hop roll-up via'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on every touched package; storetest Rollup on memstore, fsstore, sqlitestore and pgstore; `go vet` clean with and without the sqlite and postgres tags
- [x] Lint clean (`just lint`): golangci-lint on the changed packages; `just arch-lint` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: new code in packages above their floors, each path covered by new tests; CI runs the check)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer; no critical or significant findings
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-DCEJUQ7, RR-1CV4Y7B, RR-QU6BTDN, RR-IV4YQDD, RR-2KU5586, RR-2AZGPJZ, RR-S21OCEZ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
AC1 PASS (TestRollupShape). AC2 PASS (storetest path dedup cases). AC3 PASS (TestRollups_PathGatesEveryHop; storetest gated step). AC4 PASS (TestCompile_PathRejects; TestRollupShape). AC5 PASS (TestQueryBudget_ListPathRollupsAreSizeIndependent). AC6 PASS (storetest on all four backends).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-5SWO3OL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened with gh pr create; CI is checked by the parent session)

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
