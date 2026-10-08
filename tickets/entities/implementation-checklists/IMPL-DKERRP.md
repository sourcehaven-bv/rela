---
id: IMPL-DKERRP
type: implementation-checklist
title: 'Implementation: Lua history API for entity versions'
started: "2026-10-07"
completed: "2026-10-07"
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
- sqlite build, fresh project: created DOC-001, updated it, let an `rela-sqlite mcp` process run the sweep, then `rela-sqlite script h.lua` printed `versions=1`, `1 create 728fcd1b`, `v1 title=second`, `missing=nil`. After another update and sweep: `versions=2`, `2 update 69f6ea72`. (AC1, AC2, AC4 missing id)
- Default fs build: `rela.history("DOC-001")` and `rela.get_version("DOC-404", 1)` both raise `version history is not supported on this storage backend`. (AC5)
- AC3, AC4 hidden entity, AC6: `internal/lua/history_test.go` (`TestHistory_GetVersionRedacts`, `TestHistory_MissesAnswerNil`, `TestHistory_BadVersionRaises`).
- End-to-end on the scheduled-script wiring: `TestSQLiteLuaReadsHistory`.
- Found while testing: the sqlite store exposes history through `VersionStore()`, not as a method set, so the readers take it explicitly (`WithHistory`) from `versionServiceFor` / `App.versions`.
- A short-lived CLI process does not sweep, so a CLI-only workflow sees no history until a long-running process (server, MCP, desktop) has run. Same as `rela history`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
