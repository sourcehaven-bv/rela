---
id: REV-LNHGKDB
type: review-checklist
title: 'Review: Detail action order and danger style; hide Shortcuts on phones'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: `go test` on dataentryconfig, dataentry, apiwire and configedit; full vitest suite (6710); Playwright space-landing-builtins (7). The full Go suite runs in CI
- [x] Lint clean: golangci-lint on the changed packages (0 issues), `just arch-lint`, `just plimsoll`, frontend typecheck and eslint
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained: new code is covered by unit and handler tests; CI enforces the floors

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

- [x] Ran cranky-code-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (prettier churn reverted)

**Review Responses:** RR-KMCEDAJ, RR-J6C80OQ, RR-0PCXYV5, RR-H1PIQUS,
RR-HOUC0G4, RR-AC50CJ5 (addressed), RR-U04YVMZ (wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Order and label sort: PASS (detailActions.test.ts, EntityDetail.actions.test.ts).
2. Danger tone on button and menu entry: PASS (EntityDetail.actions.test.ts).
3. Bad style and fractional order refused: PASS (TestValidateActions_Style, TestAction_OrderRejectsFraction).
4. Shortcuts hidden and hidden_on_phone: PASS (Sidebar.builtins.test.ts, Playwright space-landing-builtins).
5. `phone_hidden_builtins` per space: PASS (TestSidebarBuiltins_HiddenOnPhone, _InSpace).
6. Bad builtins entries refused: PASS (TestValidateBuiltins).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-4XB8FLD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR opened with `gh pr create` per the task brief and CI watched

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
