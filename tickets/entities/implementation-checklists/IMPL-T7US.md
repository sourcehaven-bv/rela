---
id: IMPL-T7US
type: implementation-checklist
title: 'Implementation: Add server-side actions to data-entry (Lua scripts with redirect/message responses)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (lua runtime, script.Engine, dataentryconfig, dataentry handler)
- [x] Integration tests written (concurrent action execution, end-to-end via puppeteer)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced via correlation ID)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: validation tests use literal RRULE strings)
- [x] ~~Property comparisons use original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: Config tests verify actions parse and validate
- AC2: Puppeteer screenshot confirms button rendering in sidebar
- AC3: Live POST returned `{"redirect":"/entity/daily-note/DAILY-ZR1E",...}`
- AC4: Tests verify rela.params populated from config
- AC5: Click navigated to entity detail page
- AC6: Toast appeared on completion
- AC7: Script error tests verify 500 + correlation ID
- AC8: Path traversal tests + symlink rejection via os.OpenRoot
- AC9: TestHandleV1Action_Concurrent verifies serialization
- AC10: Action ID regex tests cover invalid formats
- AC11: app.go startup check via CheckActionScriptExists
- AC12: actionInFlight Set tracks per-action state
- AC13: validateRedirect tests cover //evil.com rejection

## Quality

- [x] Code follows project patterns (matches clone handler lock pattern)
- [x] No security issues introduced
- [x] No silent failures (errors logged with correlation ID AND returned)
- [x] No debug code left behind
