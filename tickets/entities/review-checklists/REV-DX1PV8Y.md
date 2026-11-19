---
id: REV-DX1PV8Y
type: review-checklist
title: 'Review: Role-relation roles count for menu and dashboard permission gates'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): touched packages acl, aclstore, access, affordances, scheduler, visibility, archguard, aclmap, dataentry, appbuild pass locally; full suite runs in CI
- [x] Lint clean (`just lint`): golangci-lint 0 issues on changed packages; arch-lint OK; plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): new code is covered by unit and handler tests; CI enforces the floors

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer both ran
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-T42JNTL)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-T42JNTL, RR-XDIWFFG, RR-NTOGIBP, RR-SEQ55RE,
RR-3I27KMQ, RR-ZFJR3RV, RR-H8JKXBP, RR-J29KW2M, RR-R5T4BJ1, RR-64Y2H72

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Role-relation role shows nav entry, page tab, space, dashboard card: PASS (TestRoleRelationPermission_ShowsGatedUI)
- Commands and other security gates stay global-only: PASS (same test; HoldsPermission unchanged in TestHoldsPermissionOnAnyEntity)
- One bounded read per Request, none when no conferred role could answer: PASS (ReadsOncePerRequest; golden parity read counts unchanged)
- Read error fails closed: PASS (ReadErrorFailsClosed)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VEB8AX4

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see TKT-UFV01M)

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
