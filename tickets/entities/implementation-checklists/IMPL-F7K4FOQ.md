---
id: IMPL-F7K4FOQ
type: implementation-checklist
title: 'Implementation: Read-only workspace wording and create form'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (workspaceReadOnly.test.ts; DynamicForm.readonly.test.ts; the wording assertion in TestReadOnly)
- [x] Integration tests written (test full flow, not just units) (the form test sends the dry-run through the real API client and its interceptor)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a successful write clears the flag; the subscriptions PUT does not; the keyboard shortcut cannot create)
- [x] Error handling in place (errors surfaced, not swallowed) (the dry-run failure is still logged; the refusal is shown as a notice)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing form mount pattern and schema store)
- [x] No hardcoded values in assertions when object is in scope (the literal sentence pins the user-facing text on purpose)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (each fix reverted once to see its test fail)

**Verification Evidence:**
Reverting the catch change, the submit guard or the keystroke guard each makes DynamicForm.readonly.test.ts fail. Focused vitest (97 files, 1313 tests), vue-tsc, eslint and go test ./internal/tenanthost/ pass. Not checked in a browser against a read-only tenant.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors tenantProvisioning.ts: a module ref set by the API client interceptor)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (the flag only hides UI; the server still refuses every write)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
