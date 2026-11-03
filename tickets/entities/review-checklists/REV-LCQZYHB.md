---
id: REV-LCQZYHB
type: review-checklist
title: 'Review: Dashboard cards render values like the rest of the app'
started: "2026-11-01"
status: done
completed: "2026-11-01"
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (via `just ci`, exit 0; frontend Vitest 5558 passed; dashboard, kanban-summary and list e2e specs pass)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues; `just lint-frontend` 0 errors, only pre-existing warnings)
- [x] Comment lint gate clean (`just comment-lint`) (part of `just ci`; no Go changes)
- [x] Coverage maintained (`just coverage-check`) (part of `just ci`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-O1S2UXP, RR-H58KU2T, RR-VHR7H8K)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-O1S2UXP, RR-H58KU2T, RR-VHR7H8K (significant,
addressed); RR-N5O4QUV, RR-FJ304BO, RR-SM2CVOI, RR-24XCLZL, RR-IAASGMM (minor,
addressed); RR-AFLXO9Q (minor, wont-fix); RR-QHRUI1M, RR-KIAFVPN (nit,
addressed); RR-X6KM8HV, RR-KE944Y7 (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: labels as styled badges, bar in style colour (Vitest query and list card; e2e).
2. PASS: declared enum order, undeclared by count, unset last (dashboardCards.test.ts, Vitest view).
3. PASS: enum badge, currency amount, formatted date in table cells (Vitest query and list card; e2e nl-NL).
4. PASS: detail links and hidden markers (existing Vitest; new link-column redaction test).
5. PASS: no map-form `sort:` left in docs; `just docs-check` clean.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-WJ7XIU5

## Final Checks

- [x] Commit message explains the why, not just what (minimal one-line messages per the user's instruction; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
