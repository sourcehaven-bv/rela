---
id: IMPL-9HVADX
type: implementation-checklist
title: 'Implementation: API error messages discarded at 22 call sites (interceptor rejects plain objects)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (src/api/errors.test.ts — 18 tests pinning the boundary contract for all four failure shapes, plus getErrorMessage/getScriptError tables)
- [x] ~~Integration tests written (test full flow, not just units)~~ (covered by the existing E2E suite — forms/kanban/document-edit-button specs exercise save-failure, drag-failure, and script-error routing through the real interceptor)
- [x] Happy path implemented (interceptor normalizes every rejection to ApiError; getErrorMessage at all 22 former instanceof-Error sites)
- [x] Edge cases from planning handled (cancellation, network-without-response, unstructured 502 bodies, bare script envelopes from non-client paths, thrown strings)
- [x] Error handling in place (server detail/title now reaches toasts/error states; correlation IDs preserved in Sidebar action errors)

## Test Quality

- [x] Using fixture builders or factories for test data (axiosErrorWith builder)
- [x] No hardcoded values in assertions when object is in scope (assertions reference the problem/scriptEnvelope fixtures)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings (toEqual(problem), toEqual(scriptEnvelope))

## Manual Verification

- [x] Feature manually tested end-to-end (E2E: 37 specs across forms/kanban/document-edit-button pass against the built rela-server)
- [x] Each acceptance criterion verified with test scenario from planning (four failure shapes table-tested; consumer sweep verified by grep: zero `instanceof Error` API catch sites remain)
- [x] ~~Edge cases manually verified~~ (edge cases are unit-tested; manual browser pass deferred to PR review — behavior change is message text only)

**Verification Evidence:** 983 unit tests pass (59 files), typecheck clean, lint
0 errors / 77 warnings (baseline). E2E forms+kanban+document-edit-button: 37
passed. Two pre-existing test mocks updated to the new contract (useAutoSave 422
mock now rejects an ApiError; schema store surfaces thrown strings as-is).

## Quality

- [x] Code follows project patterns (errors.ts mirrors the consumer-side guard pattern of scriptError.ts; getErrorMessage adopted at every catch site)
- [x] Checked for DRY opportunities — deleted four divergent parsers (useAutoSave.parseError shape-bag, DynamicForm duck-typing, InlineCreateModal duck-typing, KanbanView casts); cancellation knowledge now lives in one place
- [x] No security issues introduced (server messages were already user-visible on the legacy submit path; no new data exposure)
- [x] No silent failures (console.error retained where present; messages now surface instead of generic fallbacks)
- [x] No debug code left behind
