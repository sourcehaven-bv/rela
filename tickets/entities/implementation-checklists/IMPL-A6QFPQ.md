---
id: IMPL-A6QFPQ
type: implementation-checklist
title: 'Implementation: Rename and delete errors reveal hidden entities and their relations'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (hidden_write_errors_test.go; TestComputeActions_NoRenameForGeneratedIDs)
- [x] Integration tests written (MCP golden and dispatch cover rename_entity end to end; data-entry affordance tests through the app)
- [x] Happy path implemented
- [x] Edge cases from planning handled (gate error treated as hidden; dry run counts like the real rename; hidden entity still reads as missing before the id-type check)
- [x] Error handling in place (count fails the rename before any write; gate error fails closed)

## Test Quality

- [x] Using fixture builders or factories for test data (seedHidden, createComp)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

CLI in a temp project (component id_type manual, decision short):
- rela rename id api gateway: "Renamed: api → gateway (1 relations updated)"
- rela rename id DEC-0E5C DEC-NEW: "rename is only available for types with id_type: manual (type \"decision\")"
- rela rename id gateway db: "entity already exists: db"

go test ./... passes except cmd/rela-desktop
TestChromeStyle_TargetsShippedClasses, which needs the built SPA embedded
(unrelated).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (hiddenFromCaller shared by the cascade denial and the rename count)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
