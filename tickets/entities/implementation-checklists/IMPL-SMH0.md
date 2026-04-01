---
id: IMPL-SMH0
type: implementation-checklist
title: 'Implementation: Add checklist validation for markdown content'
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

1. **Define checklist validation rule**: Created test project with `content.checklist.all-checked: true` rule, metamodel loaded successfully
2. **Count checkboxes correctly**: TSK-001 with 1 checked, 1 unchecked detected violation correctly
3. **All-checked validation**: TSK-001 (done + unchecked) fails, TSK-002 (done + all checked) passes
4. **Allow-skipped option**: TSK-004 with strikethrough item passes when `allow-skipped: true`
5. **Integration with analyze_validations**: Violations appear in output: `✗ Done tasks must have all checklist items checked (1): TSK-001`
6. **Works with when conditions**: TSK-003 (pending + unchecked) not flagged due to `when: ["status=done"]`

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
