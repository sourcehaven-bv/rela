---
id: IMPL-YXVG1
type: implementation-checklist
title: 'Implementation: Surface Lua errors from validation rules'
status: in-progress
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

Built rela from this branch and ran `rela analyze validations` against a temp
project containing:

- `validations/broken.lua` (runtime error: `local foo = nil; return foo.bar`)
- inline `lua: |\n  if oops invalid` (compile error)
- `lua_file: missing.lua` (load error)

Output:

```
✗ Validation script errors (2):
validations/broken.lua:4: attempt to index a non-table object(nil) with key 'bar'
       1 | -- A small validation script that fails at runtime
       2 | -- so the Source slice has good context lines around it.
       3 | local foo = nil
  >    4 | return foo.bar
validations/broken-inline:1: validations/broken-inline line:1(column:15) near 'invalid':   syntax error
✗ Validation load errors (1):
  missing-script: script not found: missing.lua (must be in validations/ directory)
```

Confirms: AC1 (compile error envelope), AC2 (runtime error w/ source slice), AC6
(LoadError categorization), and the path-line-message render.

AC3, AC4, AC5, AC7, AC8 covered by unit tests in `internal/validation/`:
`lua_scripterror_test.go`, `lua_lifecycle_test.go`, `lua_timeout_test.go`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
