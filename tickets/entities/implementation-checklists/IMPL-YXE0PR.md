---
id: IMPL-YXE0PR
type: implementation-checklist
title: 'Implementation: Computed properties: reject enum literals outside the enum at load'
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
- Scratch project `/tmp/selproj` (TKT-WQJGPS schema) with `'hgh'` in one branch: `rela validate` reports `computed: invalid definitions: entity "access_point" property "level" computed: "hgh" is not one of the enum values [low medium high]`. Before the change it loaded and failed only on a hardware_key write.
- Tests: `TestCompile_EnumLiterals` (bad branch, default, root, nested, custom type, several bad literals; valid mapping, condition literals, attribute branch, plain string, string with inline values, custom type without values, concatenation), `TestProgram_ResultLiterals`, `TestProgram_ResultLiteralsAfterCoercion`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
