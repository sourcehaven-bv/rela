---
id: IMPL-ZBSEX7
type: implementation-checklist
title: 'Implementation: Live views 2/3: per-subscription SSE frames via poke and gated re-read'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (livepath_test.go, live_hub_test.go, useLiveSubscription.test.ts, live.test.ts)
- [x] Integration tests written (test full flow, not just units) (live_test.go drives the full router over real HTTP with real store writes)
- [x] Happy path implemented
- [x] Edge cases from planning handled (PUT before ready, closed stream 404, changed claims 409, round cap fallback, unfinished replay keeps hash, unknown plural in the SPA)
- [x] Error handling in place (errors surfaced, not swallowed) (PUT errors are typed 400/404/409/429; unfinished replays are logged)

## Test Quality

- [x] Using fixture builders or factories for test data (newLiveEnv, ticket1, feature, aclTicketsOnly)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

rela-server built after the review fixes, on a copy of the tickets project,
driven with curl:

1. `GET /api/v1/_events` gave `ready` with `{"conn":"<22 chars>"}`.
2. A PUT with `/api/v1/_events` as a path gave 400. A PUT of
`/api/v1/tickets/TKT-00SFFP` (key `detail`), `/api/v1/_views/ticket/TKT-00SFFP`
(key `view`) and `/api/v1/tickets?q=live` (key `search`) gave 204 in 0.59 s, the
baseline replays. No frame followed the PUT.
3. PATCH of the ticket's title to "Live check A": `live:changed` for
`detail`, `search` (the ticket now matches) and `view`.
4. PATCH of FEAT-72NR1, which the ticket implements: `live:changed` for
`view` only.

AC1 to AC7, AC9 and AC10 each have a test in `internal/dataentry/live_test.go`;
AC8 is the PUT-time baseline (`TestLiveHub_BaselineBeforeAnswer`) plus the
client refetch after the 204 (`useLiveSubscription.test.ts`). Replacing the
replay chain's `attachACLRequest` with nothing fails AC4, AC5 and AC10. The full
dataentry and changefeed suites pass with the race detector; the frontend suite
passes (280 files, 4043 tests).

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side liveFeed interface, atomic.Pointer for the installed handler, writeV1Error)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (replays run through the production ACL wrappers; parity test; subscriptions bound to the opening identity)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
