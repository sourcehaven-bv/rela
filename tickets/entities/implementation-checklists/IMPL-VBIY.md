---
id: IMPL-VBIY
type: implementation-checklist
title: 'Implementation: Add Lua action type to automation engine'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1 (Inline Lua): `TestEngine_LuaInline` - lua action in automation adds code to `LuaToExecute`
- AC2 (Script file): `TestEngine_LuaFile` - lua_file action adds filepath to `LuaToExecute`
- AC3 (Entity context): `EntityToTable` exported, workspace sets `entity` global
- AC4 (Old entity context): workspace sets `old_entity` global for update events
- AC5 (Safe interpolation): `TestEngine_LuaInlineDoesNotInterpolateEntityProperties` - entity props NOT interpolated
- AC6 (Mutation via bindings): workspace `executeLuaCode` creates runtime with full workspace access
- AC7 (Error handling): Lua errors captured in result, not panics
- AC8 (Security): `loadLuaScript` uses `os.OpenRoot` pattern, sandbox preserved

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
