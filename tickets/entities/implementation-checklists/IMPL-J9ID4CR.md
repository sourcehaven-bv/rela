---
id: IMPL-J9ID4CR
type: implementation-checklist
title: 'Implementation: Structured questions in realtime channels'
started: "2026-11-11"
completed: "2026-11-11"
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

- Store conformance (`channelstest` batch and asked tests) passes on the file, SQL and SQLite backends.
- `internal/realtime` batch tests: questions move on the last answer, refusals, the asked lock, charging only when accepted, removal of the last open question, and no asked set without a session.
- HTTP `_batch`, MCP `post_messages` and CLI `post-batch` tests pass; OpenAPI and MCP goldens updated.
- Frontend: 6054 unit tests, typecheck, lint and the library checks pass; `groupQuestions` covers a notice inside a batch.
- e2e `channels.spec.ts`: 18 of 18 pass, including the asked-questions test (5 of 5 on repeat).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
