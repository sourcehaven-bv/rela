---
id: IMPL-ITQT
type: implementation-checklist
title: 'Implementation: Add shebang support to Lua scripts'
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

1. **AC1 - Scripts with shebang execute**: Tested with `scripts/test-shebang.lua` containing `#!/usr/bin/env -S rela script` - executed successfully with output `{"args":["arg1","arg2"],"message":"Hello from shebang script!"}`

2. **AC2 - Shebang only stripped from first line**: Unit test `TestStripShebang_ShebangInMiddle` verifies shebang-like content in middle of file is NOT stripped

3. **AC3 - Scripts without shebang work**: All existing tests pass (78.8% coverage), no regressions

4. **AC4 - All entry points covered**: `RunString()` now calls `StripShebang()` - all paths (MCP, validation, automation) funnel through it

5. **AC5 - Error line numbers accurate**: Test `TestRunFile_ErrorLineNumbers_WithShebang` verifies error on line 2 reports as `line:2`

6. **Filename in errors**: Test `TestRunFile_ErrorIncludesFilename` verifies filename appears in error messages

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
