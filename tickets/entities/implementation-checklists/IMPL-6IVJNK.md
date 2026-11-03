---
id: IMPL-6IVJNK
type: implementation-checklist
title: 'Implementation: Show a related entity''s property on a kanban card'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (Go: validate_kanban_related_test.go, calendar and gantt refusal cases; Vitest: KanbanView.relatedProperty.test.ts, CardFieldList values and label, configDraft rename of a related card property)
- [x] Integration tests written (Go: TestListIncludeIncomingPeer_StripsHiddenProperty on the list endpoint; e2e: kanban-related-property.spec.ts against the built rela-server)
- [x] Happy path implemented (relation plus property on a kanban card field, both directions)
- [x] Edge cases from planning handled (no related entity, related entity without the value, unresolved id, duplicate values, several target types, title and property fields on one relation)
- [x] Error handling in place (every invalid config is a load error with the key path and the offending type)

## Test Quality

- [x] Using fixture builders or factories for test data (deal(), org(), board(), relatedPropertyMetamodel())
- [x] No hardcoded values in assertions when object is in scope (amount and date assertions use formatAmount and formatDate)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (e2e spec drives the real server and browser in nl-NL)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test` on dataentryconfig, dataentry and
configedit pass. Vitest on views, components/common, stores, configure, utils
and widgets passes (111 files, 1650 tests); `vue-tsc -b` clean. Playwright: the
full suite ran with 467 passed, 13 skipped and 1 failure in crud.spec.ts (an
undo toast timeout under load) that passes on rerun and does not touch
kanban. kanban-related-property.spec.ts shows the badges "in_progress"
(orange) and "done" (green) once each for three implementing tasks, no field
on a feature without tasks, and "Fix value: € 3.500" for an outgoing currency.
The backend is unchanged: the Go list test shows `include=*` already carries
the incoming peer with hidden properties stripped.

## Quality

- [x] Code follows project patterns (widgets resolved once per field and type as cardFieldWidgets does; validation next to the other card field checks; refusal in the shared helper for calendar and gantt)
- [x] Checked for DRY opportunities (reuses InferDirection, CheckAmbiguousDirection, formatCellValue, densePropertyRoutingHint, relationCardKey)
- [x] No security issues introduced (reads only the row-gated, field-redacted include block)
- [x] No silent failures (wrong side, missing property and the form on calendar and gantt are load errors)
- [x] No debug code left behind
