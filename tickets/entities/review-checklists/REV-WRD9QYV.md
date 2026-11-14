---
id: REV-WRD9QYV
type: review-checklist
title: 'Review: Store-side paging for list_entities filters that do not lower'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide tests not run by request; go test -race passes on the touched packages mcp, visibility, appbuild, archguard, cli, dataentrywire, plus mcp with sqlite and mcp, visibility, appbuild with postgres against a scratch server. CI runs the full suite)
- [x] Lint clean (`just lint`): golangci-lint run serially on the changed packages with default, sqlite and postgres tags, 0 issues; just arch-lint clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: not run locally; the change adds tests to every touched package and CI enforces the floors)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer run; the security review had no findings
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: RR-O8TE9TN and RR-GQ84MYF addressed; RR-CY6WIFE deferred with reason (the field only narrows, its one producer gates every entry; follow-up under TKT-9BKVQ9I)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CY6WIFE, RR-O8TE9TN, RR-GQ84MYF, RR-KWQPV6C,
RR-63L9SOA, RR-S1RROIE, RR-NIY92UB, RR-DEEC72H, RR-EV38III

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestHandleListEntities_ConditionScaleFree, 9 lowered filters; fails with the lowering disabled.
2. PASS: TestListEntities_ConditionMatchesGoPath on memstore, sqlite and postgres.
3. PASS: dave on reported filters reads nothing (differential path assertion); TestQueryConditions_LowerDeniedTraversal.
4. PASS: frank on status filters stays on the Go path (differential path assertion).
5. PASS: TestHandleListEntities_CurrentUserFilterAnswersForCaller.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-mcp-server and docs/mcp-server.md
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-KGKYTPI

## Final Checks

- [x] Commit message explains the why, not just what: minimal message with the ticket id, per the user's commit style; the why is in TKT-Z099FGY
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh pr create after pr-guidance-check, and CI is not monitored, per the task brief)
