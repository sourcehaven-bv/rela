---
id: REV-QA622O
type: review-checklist
title: 'Review: Dashboard cards: list source, server-side values, matrix and ratio'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test on dataentry, dataentryconfig, configedit, apiwire; npm run test:run 4961 passed)
- [x] Lint clean (`just lint`) (golangci-lint, arch-lint, plimsoll clean; eslint 0 errors)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new code covered by dashboard_card_test.go and dashboard_cards_validate_test.go)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and rela-security-reviewer; security found nothing)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-NBALPP, RR-9DL1H4, RR-AJOVY9, RR-JVY411, RR-LGFQX9,
RR-PI95PX, RR-Z4Q59I, RR-KE0KI2, RR-WBX0LY, RR-U0GVB6 (deferred), RR-UCP683
(wont-fix), RR-3XN6R5, RR-GWXH25, RR-E8DSH3 (wont-fix), RR-4DZFG0, RR-P8XC7U,
RR-OLPB07; design review: RR-38G262, RR-LQ78QX, RR-NCLPZA, RR-ZUX3FW

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-GCU87I)

**Acceptance Status:**

- list: card source: PASS (TestDashboardCard_* handler tests, SPA list-card tests)
- server values with gate-before-fold, partial, no cross-principal cache: PASS (hidden-row and hidden-field tests)
- budget test: PASS (TestDashboardCard_ReadBudget, per shape)
- matrix with tone_by and colors, severity by declared order, unplaced: PASS (fold tests, DashboardMatrix.test.ts)
- ratio with match and group_by: PASS (fold and handler tests, SPA ratio test)
- frontend tiles keyboard accessible with click-through: PASS (router-link cells with aria-label; manual check in light and dark mode)
- docs: PASS (GUIDE-data-entry)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-LLY9H2

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per project convention; ticket id carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (runs after done, see note below)

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
