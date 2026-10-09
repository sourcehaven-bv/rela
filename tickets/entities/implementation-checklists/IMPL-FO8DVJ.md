---
id: IMPL-FO8DVJ
type: implementation-checklist
title: 'Implementation: Wire a policy-backed FieldWriteGate so MCP and Lua callers cannot write fields their policy hides'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
  - affordances: `TestCheckFieldWrite` (13 cases) and `NewWriteGate` nil handling.
  - entitymanager: `TestCreateEntity_FieldGate`.
  - dataentry: rule equality and the wrapped-denial 403.
  - entitymanager: `TestFieldGated_DefaultHandleSkipsGate`, `TestFieldGate_DenialIsAudited` and `TestGated_DropsFieldGate`.
- [x] Integration tests written: `internal/appbuild/fieldgate_test.go` runs a real Discover with acl.yaml. It covers the gated handle on patch, unset, create and undeclared fields; scheduled Lua `rela.update_entity`; and the default handle and CLI Lua deps staying ungated while row grants still apply. A mutation check (wiring AllowAll) fails two of the tests. `cmd/rela-server/mcp_deps_test.go` pins MCP to the gated handle.
- [x] Happy path implemented. After the code review the gate moved from the shared Manager onto an opt-in `entitymanager.FieldGated` handle. That handle goes to rela-server MCP and scheduled Lua, so dataentry and the CLI keep their behaviour (RR-OX8G0N).
- [x] Edge cases from planning handled: unset, enum lists, undeclared fields, name-ordered first denial, typed-nil guard, elevation and cascade writer skipped.
- [x] Error handling in place: a construction failure is returned, never downgraded; a denial is a typed error mapped to 403 by dataentry.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: in a temp fs project with acl.yaml (`fields: task: [title]`, alice an editor), `USER=alice rela update TASK-001 -P status=done` succeeded (the CLI is ungated). `USER=mallory` was refused by the row grant.
- [x] Each acceptance criterion verified with test scenario from planning
  1. and 2. `TestFieldGate_PolicyHoldsOnManager` and `TestFieldGate_LuaUpdateRaises`.
  2. The manual CLI run and `TestFieldGate_OperatorSurfacesUngated`.
  3. The patch_test constraint tests pass unmodified, and `TestCreateEntity_FieldGate` passes.
  4. The dataentry suite passes.
  5. `TestFieldGate_*` covers the case with grants, and the existing appbuild suite covers the case without them.
  6. arch-lint is OK.
- [x] Edge cases manually verified

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities: the dataentry check now delegates to the shared one instead of keeping a second copy.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
