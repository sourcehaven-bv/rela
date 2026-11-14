---
id: IMPL-UC5ECZO
type: implementation-checklist
title: 'Implementation: Bound the remaining Lua read bindings'
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `go test -race` on lua, visibility, script and archguard: pass. `TestGoldenParity` on the default and sqlite builds: pass (one golden rerecorded: the elevated document now reads through ListEntitiesPage; same body).
- Built `rela` from the branch and ran a script against `tickets/`: `get_relations({type="affects"})` returned 1821 edges, `limit = 5` returned 5, `entity_refs({content=...})` returned only the two mentioned ids, `limit = 0` raised.
- `TestReadBindings_CostIsScaleFree` pins the six reads at 100 vs 1000 padding rows.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
