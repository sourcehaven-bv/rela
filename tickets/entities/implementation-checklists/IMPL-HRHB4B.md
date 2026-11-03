---
id: IMPL-HRHB4B
type: implementation-checklist
title: 'Implementation: Kanban column colour, column summaries and currency display'
started: "2026-10-14"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (Go: validate_kanban_summary_test.go, metamodel display_test.go; Vitest: KanbanView.summary.test.ts, utils/summary.test.ts, formatDisplayNumber in format.test.ts, NumberWidget display in widgets.test.ts, configDraft rename of a column_summary property)
- [x] Integration tests written (e2e/tests/kanban-summary.spec.ts against the built rela-server: colours, summaries, empty column, collapsed rail, card amount, entity page amount)
- [x] Happy path implemented (enum column colour, column_summary, display: {currency} on the property)
- [x] Edge cases from planning handled (empty column shows 0 for count and sum and – for avg, min and max, missing and non-numeric values add nothing, partial marker, collapsed rail shows the total, columns_from boards supported, swimlanes refused)
- [x] Error handling in place (every invalid config is a load error with the key path)

## Test Quality

- [x] Using fixture builders or factories for test data (deal(), board(), e() helpers)
- [x] No hardcoded values in assertions when object is in scope (view tests compare against formatAmount of the summed fixture values)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (e2e spec drives the real server and browser in nl-NL)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence (redesign, 2026-10-15):** `go test` on
dataentryconfig, metamodel, configedit, apiwire and dataentry pass. Full
Vitest suite passes (303 files, 4251 tests); `vue-tsc -b` clean. Playwright:
kanban-summary.spec.ts and every kanban*.spec.ts pass (24 tests).

## Quality

- [x] Code follows project patterns (styleStatusColor and stylesForProperty as useRelationColumns; display format on PropertyDef next to the other property options; library Section stays data-agnostic)
- [x] Checked for DRY opportunities (one Summary type and validateSummaries for later surfaces; formatDisplayNumber shared by cells, widget, view rows and summaries)
- [x] No security issues introduced (client-side sum over cards already gated and redacted for the reader)
- [x] No silent failures (swimlanes plus column_summary, count with a property and display without a format are load errors, not ignored keys)
- [x] No debug code left behind
