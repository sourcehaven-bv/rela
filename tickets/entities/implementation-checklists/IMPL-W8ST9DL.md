---
id: IMPL-W8ST9DL
type: implementation-checklist
title: 'Implementation: idp-sync.lua keeps roles of a deactivated user and calls a missing Pratique route'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestIdpSyncAction`, `TestIdpSyncAction_ClearsRolesUnlessActive`, `TestIdpSyncAction_ErrorKeepsRoles` in internal/lua/idpsync_test.go)
- [x] ~~Integration tests written~~ (N/A: the tests run the shipped script in the real Lua runtime against an HTTP stub; the receiver's handling of an empty `asserted_roles` list is pinned by `TestWebhook_StoresAssertedRoles` "empty list clears")
- [x] Happy path implemented (active member: person upserted, roles returned)
- [x] Edge cases from planning handled (inactive, pending, missing status, absent subject, empty list, org 404)
- [x] Error handling in place (non-200, bad JSON, missing members array return an error result without roles, so the IdP retries)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: JSON member lists are inline literals; a stub helper builds the server)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (member JSON built from the userID/email constants)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no live Pratique available; the stub mirrors Pratique develop's operator routes and HMAC check)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (table test cases)

**Verification Evidence:** On develop the new tests fail with "fetch returned
405" for every case. With the fix all three tests pass; `go test
./internal/lua/` and the dataentry webhook tests pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one `clear` helper for every role-clearing path)
- [x] No security issues introduced (unknown status fails closed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
