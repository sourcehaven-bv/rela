---
id: REV-3Z68HL
type: review-checklist
title: 'Review: Kanban column colour and column totals'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (targeted locally: `go test ./internal/dataentryconfig/ ./internal/configedit/`, Vitest KanbanView*, configure, stores, utils/format; Playwright kanban specs; `just ci` runs before the PR)
- [x] Lint clean (golangci-lint on dataentryconfig and configedit: 0 issues; eslint clean apart from the existing KanbanView max-lines warning; vue-tsc clean; rela-components `npm run check` passes)
- [x] Comment lint gate clean (part of `just ci`)
- [x] Coverage maintained (new Go code covered by validate_kanban_amounts_test.go; checked by `just ci`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (gofmt, list properties, redacted values)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-U9DSMW, RR-40HYLG, RR-YIAFYM, RR-TPOPYC, RR-WY92VE,
RR-OO92GT, RR-00UCAA, RR-F50TE4 (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Enum column colour from styles, none without: PASS (KanbanView.totals.test.ts colour test; kanban-totals.spec.ts expectColumnColor blue and green).
2. column_total under each heading, empty column 0, collapsed rail: PASS (KanbanView.totals.test.ts; kanban-totals.spec.ts "Totaal: € 11.500", "Totaal: € 0", collapsed "Totaal: € 1.200").
3. currency: EUR renders 3500 as "€ 3.500" in nl-NL: PASS (format.test.ts; kanban-totals.spec.ts card field).
4. Config load rejects invalid config: PASS (TestValidateConfig_KanbanAmounts, calendar and gantt refusal cases).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VZF7RE

## Final Checks

- [x] Commit message explains the why, not just what (minimal one-line message per the user's instruction; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after the ticket is done)
