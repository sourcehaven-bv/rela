---
id: IMPL-E9N5S3
type: implementation-checklist
title: 'Implementation: Live views 1/3: sequence-numbered change feed with SSE resume'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (changefeed_test.go, token_test.go: 95% coverage)
- [x] Integration tests written (test full flow, not just units) (sse_resume_test.go drives handleSSE over real HTTP with real store writes)
- [x] Happy path implemented
- [x] Edge cases from planning handled (foreign/oversize/garbage token, token past head, ready token before replay, hidden types, Retire)
- [x] Error handling in place (errors surfaced, not swallowed) (Start errors logged; write errors end the stream)

## Test Quality

- [x] Using fixture builders or factories for test data (resumeApp, connect, write helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

rela-server on a copy of the tickets project, driven with curl:

1. `GET /api/v1/_events` gave `event: ready` with a 48-character `id:`.
2. Created a ticket with `POST /api/v1/tickets` (201).
3. Reconnected with `?last_event_id=<ready id>`: got `ready`, then
   `entity:changed {"type":"ticket"}` with an `id:`.
4. Reconnected with `Last-Event-ID: nope`: got `refresh {"reason":"resync"}`
   with an `id:`.

AC1 to AC6 each have a test in `internal/dataentry/sse_resume_test.go`; the
SPA reconnect is covered in `useEvents.test.ts`.

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side interfaces, constructor rejects nil, arch-lint entry)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (encrypted tokens hide write counts; resumed changes gated per new connection)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
