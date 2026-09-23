---
id: REV-BWM2DX
type: review-checklist
title: 'Review: Query-driven entity lists in sidebar navigation groups'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (Go: `just test` exit 0; frontend: 199 files / 3206 tests; e2e: 319 passed, one customisation timeout under load passed 7/7 on rerun)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues; eslint clean on changed files; arch-lint OK; markdownlint 0 issues)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links; comment-report findings on touched files all predate this branch)
- [x] Coverage maintained (`just coverage-check`) (PASS, total 80.2%)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer plus rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-SLFG7U addressed)
- [x] Self-reviewed the diff for unrelated changes (prettier churn in Sidebar.vue reverted; firstNavTarget edit reverted per RR-KC17CF)

**Review Responses:** RR-SLFG7U (significant, addressed); RR-KC17CF, RR-FMWYL4,
RR-XZKVAJ, RR-52NOTZ (minor, addressed); RR-3RUOQV, RR-QOA8GX, RR-ZCNY31 (nit,
addressed); RR-XADG5E, RR-6B5A0W (nit, deferred with reason).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-5Q57ZZ)

**Acceptance Status:**

1. PASS: Sidebar.entities.test.ts renders one link per row with display name and href; e2e sidebar-entities.spec.ts opens FEAT-003 from its link.
2. PASS: TestSidebarEntities_RowsAreACLGated and TestSidebarEntities_RowsFollowThePrincipal feed the served definition to the list endpoint and get only the principal's rows; SidebarEntityQuery drops held rows on 401/403/404.
3. PASS: TestSidebarEntities_ServesDefinitionOnly passes the scope through as configured (empty when omitted), so the list endpoint's existing default/`all` handling applies; naventities_test.go accepts the implicit `all` scope.
4. PASS: naventities_test.go table cases for every rejection.
5. PASS: e2e PATCH brings FEAT-002 into the group and moving both out hides it, without reload.
6. PASS: Sidebar.entities.test.ts shows "and 103 more"; SidebarEntityQuery.test.ts overflow case.
7. PASS: empty group hidden, including while the first fetch is pending and after a reload; mixed group stays visible.
8. PASS: SidebarEntityQuery.test.ts world param and link query.
9. PASS: TestNavPermission_ConfigUnfiltered asserts `/_config` is identical across principals; TestSidebarEntities_ServesDefinitionOnly asserts the sidebar carries only the definition.
10. PASS: GUIDE-data-entry "Entity lists in a group"; docs/ regenerated.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-54NM44

## Final Checks

- [x] Commit message explains the why, not just what (minimal subject naming the ticket, per user preference)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the user asked for a commit; no PR requested)

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
