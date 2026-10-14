---
id: IMPL-HRHB4B
type: implementation-checklist
title: 'Implementation: Kanban column colour and column totals'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (Go: validate_kanban_amounts_test.go, calendar and gantt refusal cases; Vitest: KanbanView.totals.test.ts, formatAmount/numericValue in format.test.ts, configDraft rename of column_total.property)
- [x] Integration tests written (e2e/tests/kanban-totals.spec.ts against the built rela-server: colours, totals, collapsed rail, card amount)
- [x] Happy path implemented (enum column colour, column_total, card currency)
- [x] Edge cases from planning handled (empty column shows 0, missing and non-numeric values add nothing, collapsed rail shows the total, columns_from boards supported, swimlanes refused)
- [x] Error handling in place (every invalid config is a load error with the key path)

## Test Quality

- [x] Using fixture builders or factories for test data (deal(), board(), total(), field() helpers)
- [x] No hardcoded values in assertions when object is in scope (view tests compare against formatAmount of the summed fixture values)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (e2e spec drives the real server and browser in nl-NL)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test ./internal/dataentryconfig/
./internal/configedit/` pass. Vitest: KanbanView*, configure, stores,
utils/format all pass (27 files, 318 tests plus format). `npm run typecheck`
clean; rela-components `npm run check` passes. Playwright: kanban-totals.spec.ts
passes; kanban.spec.ts, kanban-collapse.spec.ts and kanban-card-counts.spec.ts
pass (two drag tests timed out on page.goto under a load average near 100 and
passed on rerun).

## Quality

- [x] Code follows project patterns (styleStatusColor and stylesForProperty as useRelationColumns; refuseCardCounts for kanban-only keys; library Section stays data-agnostic)
- [x] Checked for DRY opportunities (checkIntegerProperty and checkCurrencyCode shared by column_total and card fields; formatAmount shared by totals and cards)
- [x] No security issues introduced (client-side sum over cards already gated and redacted for the reader)
- [x] No silent failures (swimlanes plus column_total and currency on calendar or gantt are load errors, not ignored keys)
- [x] No debug code left behind
