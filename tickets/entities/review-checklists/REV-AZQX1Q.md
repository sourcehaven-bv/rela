---
id: REV-AZQX1Q
type: review-checklist
title: 'Review: Named dashboards: dashboards: in data-entry.yaml and dashboard: <name>'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): run inside `just ci` and `just coverage-check`
- [x] Lint clean (`just lint`): golangci-lint 0 issues; eslint 0 errors; plimsoll clean after pinning `apiwire/v1.Config` at 21 fields
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): 12 findings
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed (5 of 5)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Addressed: RR-WHEJNX, RR-AOH7CV, RR-TTC4RD, RR-MC0I9K,
RR-U7TF4J (significant); RR-05V02V, RR-OTNDE3, RR-3WS1KB, RR-CPP788 (minor).
Won't fix: RR-9UVWPL, RR-9XTP00 (nits). Deferred: RR-BX0T9X (Configure view
component tests; no harness exists yet).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: existing Go, Vitest and e2e dashboard tests pass with only the `DashboardRef` type change.
2. PASS: `dashboard_named_test.go` sidebar, page tab and space home hrefs; `dashboardRoutes.test.ts`, `sidebarNav.test.ts`, `PageView.test.ts`; e2e `named-dashboards.spec.ts`.
3. PASS: `dashboard_named_test.go` serves by name, filters cards per principal and returns 404 `dashboard_not_found`.
4. PASS: `dashboard_test.go` validation table covers unknown names in nav, home, tab and `available_on`, invalid names and `dashboards.default`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CYUDLI

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the user asked for one-line commit messages; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (runs after this checklist; see the note below)

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
