---
id: IMPL-U5X173M
type: implementation-checklist
title: 'Implementation: Gated paged reads for MCP list_entities and search with no text'
started: "2026-11-13"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by the MCP handler tests and the dataentry golden mcp-list run through the full stack)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `TestHandleListEntities_ScaleFree`, `_PagedTotals`, `_PushedFilterMatchesGoFilter`, `_UnloweredFilterKeepsGoPath` pass.
- `TestScriptReader_ListEntityPage` passes on memstore and on postgres (`_Postgres`).
- `TestSearch_NoTextIsScaleFree`, `TestSearchVisible_NoTextIsScaleFree`, `TestSearch_NoTextAdmitsWholeFamilies` pass.
- pgstore conformance, graph differential and field-scope conformance pass.
- `TestGoldenParity` (default and sqlite): only `reads:`/`breadth:`/`bodies:` lines of the four `mcp-list` goldens changed; regenerated.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
