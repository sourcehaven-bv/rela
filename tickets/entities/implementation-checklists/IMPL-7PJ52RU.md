---
id: IMPL-7PJ52RU
type: implementation-checklist
title: 'Implementation: Fork the ygo relay and stamp awareness identity on the server'
started: "2026-11-05"
completed: "2026-11-06"
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

- [x] ~~Feature manually tested end-to-end~~ (not clicked through in a browser: real WebSocket tests through rela's handler and the relay cover the flow)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test -race ./third_party/...
./internal/dataentry/` passes, including the upstream suite. New tests:
`TestStampAwareness` (table), `..._RoomStoresTheServerName`,
`..._PeersReceiveTheServerName`, `..._NilStampIsRefused`,
`TestAwarenessOwnership_*` (with and without owner, owner updates and removes,
binding outlives the connection), `TestCollab_PresenceCarriesThePrincipal`. The
ownership and binding tests fail when their rule is disabled.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
