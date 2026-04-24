---
id: IMPL-6O8DN
type: implementation-checklist
title: 'Implementation: MCP create_entity ignores id_type — allows custom ID on short/sequential types'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes:

- `internal/workspace/workspace.go`: added `IsManualID()` guard in `createEntityCore`'s non-empty-ID branch before the charset check. Error message echoes type, `id_type`, and offending ID.
- `internal/workspace/workspace_test.go`:
  - Dropped explicit `ID: "REQ-001"` / `ID: "DEC-001"` from tests that were only using those IDs for deterministic seeding. Sequential generation produces the same IDs naturally.
  - Converted `TestCreateEntity_WithCustomID` and `TestCreateEntity_DuplicateID` to use the `stakeholder` (manual-ID) type — these tests legitimately verify custom-ID behavior, which is only valid for manual types post-fix.
  - Added `TestCreateEntity_CustomIDRejectedForSequential` and `TestCreateEntity_CustomIDRejectedForShort` covering AC1/AC2 and asserting no store side-effects.
- `internal/mcp/tools.go`: tightened the `id` tool-parameter description to signal that it's only valid for `id_type=manual`.
- `internal/mcp/tools_test.go`: added `TestHandleCreateEntity_RejectsCustomIDForShortType` covering AC5 — verifies the error surfaces through `handleCreateEntity`.
- `internal/dataentry/api_v1_test.go`: `TestV1CreateEntity_SavesRelations` no longer hardcodes an ID (it used to pass `"id":"TKT-CREATE"` on a short-ID type — precisely the bug); extracts the created ID from the `Location` header. Added missing `IDPrefix` to the inline test metamodel so auto-generation has a prefix to work with.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Notes: The dataentry test now builds `createdID` from the response `Location`
header rather than assuming a fixed string — the test no longer depends on a
specific ID value. Workspace negative tests use `strings.Contains` assertions on
the error message rather than full-string equality, so error wording can evolve
without test churn.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1 (custom ID on `id_type: short` rejected): `TestCreateEntity_CustomIDRejectedForShort` and `TestHandleCreateEntity_RejectsCustomIDForShortType` both pass; error text contains type, `id_type=short`, and the offending ID.
- AC2 (custom ID on `id_type: sequential` rejected): `TestCreateEntity_CustomIDRejectedForSequential` passes; error text contains `"requirement"`, `"sequential"`, `"REQ-042"`; store confirms no side effect.
- AC3 (custom ID on `id_type: manual` works): `TestCreateEntity_WithCustomID` passes against the `stakeholder` type with `ID: "alice"`.
- AC4 (omitted ID still auto-generates): existing `TestCreateEntity`, `TestGenerateID`, `TestGenerateID_Sequential`, and `TestGenerateID_ManualType` tests all pass unmodified.
- AC5 (MCP surfaces the error): `TestHandleCreateEntity_RejectsCustomIDForShortType` asserts `result.IsError` and the workspace error text propagates verbatim.
- Full suite: `go test -race ./...` green across all 40+ packages.
- Lint: `just lint` — 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The guard mirrors the existing `IsManualID()` check in `GenerateID` — same
pattern, opposite direction. Error is returned by value, not
logged-and-swallowed. No debug prints, no TODO comments.
