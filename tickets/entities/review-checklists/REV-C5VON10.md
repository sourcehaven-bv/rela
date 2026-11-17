---
id: REV-C5VON10
type: review-checklist
title: 'Review: Conditional grant edges: when: on role_relations and membership'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `just coverage-check` ran the full suite. Every package passed except two local environment failures: cmd/rela-desktop TestChromeStyle needs a built SPA (no frontend dist in this worktree), and internal/dataentry hit the 10-minute package timeout under coverage at load average 46. internal/dataentry passes on its own without coverage. CI is the final judge.
- [x] Lint clean (`just lint`): 0 issues on the changed packages after fixing gofmt, mnd, revive, whitespace, govet and two gocognit findings; `just arch-lint` and `just plimsoll` clean.
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): changed packages above their floors (acl 87.3%, aclaudit 94.2%, affordances 91.5%, appbuild 78.1%, predicate 85.0%, scheduler 85.3%, readtime 100%).

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer.
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-GMMLWUF, RR-8D6TEWE, RR-N6X26MF, RR-DKUJUBU,
RR-ZWDVMLM, RR-276MY36, RR-GLJ6GBS, RR-0WZLV0V, RR-T3NTM68 (addressed);
RR-UO5XPSB, RR-JXTSX7C, RR-NT49296 (wont-fix, nits); RR-IX05LQY (deferred,
minor). Design review: RR-3A6B31G, RR-9CJES4V, RR-D2UZU0V, RR-D8X02UC,
RR-FMT4AFT, RR-HN6I1U7, RR-O9NYUF7, RR-WF0WDIS, RR-Y5XSQ2Y.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- PASS: `when:` on a role_relations entry and on the membership relation; an edge grants only while the condition holds (TestConditionalGrant_GoAndStoreAgree, TestConditionalMembership, e2e `rela acl can`).
- PASS: the condition lowers exactly into a store query; anything else fails the load (TestValidateACLPolicy_GrantConditions; store differential on naive, sqlite and postgres).
- PASS: fail closed on a missing or malformed date and without a read time (TestGraph_EdgeCondition, TestBindGrantConds_UncompiledWhenFailsClosed).
- PASS: no runtime deny; identity is resolved once per request; the read time is stamped at every edge (TestTaskContext_StampsAFreshReadTime, TestSSE_NewDayRegates, TestSSE_FailedNewDayRegateWithholds).
- PASS: `rela acl audit` reports conditional grants as info (TestAudit_A16_ConditionalGrant).
- PASS: docs explain that access ends at the next request after expiry, with no job (GUIDE-acl-overview, GUIDE-acl-security).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-7AAS994

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: opened after this checklist (TKT-UFV01M).

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
