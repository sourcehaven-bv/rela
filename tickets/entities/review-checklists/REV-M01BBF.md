---
id: REV-M01BBF
type: review-checklist
title: 'Review: Push down query scopes built from related() and equalities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XEM47P, RR-IROC3N, RR-DZ87RX, RR-RHBIT6, RR-5RYODF,
RR-QIE80I, RR-957LI1 (minor, addressed); RR-8TNTL0, RR-RXJH05 (nit,
addressed); RR-62V1Z6, RR-3H5SUV (nit, wont-fix). The security review found
nothing. No critical or significant findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Same store-query count at 10 and 50 rows: PASS
  (`TestQueryBudget_TraversalScopeIsPushedDown`).
- pg EXPLAIN uses the derived indexes: PASS
  (`TestExplainPushedTraversalScopeListUsesDerivedIndexes`, no Seq Scan on
  entities or relations for page and count).
- Non-lowering scopes keep today's behaviour: PASS
  (`TestListPushdown_ScopeMatchesGoPath` on memstore and postgres,
  `TestListPushdown_RefusedTraversalFailsTheRequest`,
  `TestQueryBudget_TraversalScopeIsSizeIndependent`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-44CH2B

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design.)

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
