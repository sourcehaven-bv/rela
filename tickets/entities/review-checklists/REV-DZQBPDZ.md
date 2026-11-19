---
id: REV-DZQBPDZ
type: review-checklist
title: 'Review: Configurable dashboard validation card and entity header History and Add to pile'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: `go test` on dataentry, dataentryconfig, apiwire and cli; vitest DashboardView and EntityDetail.world (126); Playwright page-chrome (4). The full suite runs in CI
- [x] Lint clean: golangci-lint on the changed packages (0 issues), `just plimsoll`, `just arch-lint`
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

- [x] Ran cranky-code-reviewer and rela-security-reviewer (the security review found nothing)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (prettier churn reverted)

**Review Responses:** RR-Z6Q3I00, RR-KDOUBIQ, RR-9I3N1AK, RR-8FENUUI,
RR-669BAAZ, RR-Y7HYKT1, RR-I730GYV, RR-BCQ3WUD, RR-TMPGOJI (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Defaults unchanged: PASS (TestPageChrome default case, NoDashboardShowsValidation, vitest older-server case).
2. `false` hides: PASS (Go, vitest, Playwright).
3. Permission gate per principal: PASS (Go holder, non-holder, no acl.yaml, named dashboard; Playwright with and without the permission).
4. Bad shapes refused: PASS (TestChromeToggle_UnmarshalYAML, TestValidateConfig_PageChrome, TestValidateEntityDashboards).
5. `rela acl audit` counts the permissions: PASS (TestDataEntryPermissions cases).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-E5LHOYM

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
