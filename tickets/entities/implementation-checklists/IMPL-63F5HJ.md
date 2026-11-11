---
id: IMPL-63F5HJ
type: implementation-checklist
title: 'Implementation: Follow a retired id to its successor on the entity detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestResolver_Successor; retired_redirect_test.go; EntityDetail owner/world tests)
- [x] Integration tests written (test full flow, not just units) (handler tests run the real routes through the ACL gate, policy reader and world stub)
- [x] Happy path implemented
- [x] Edge cases from planning handled (hidden successor, other type, face dropped, query kept, write not redirected, world-absent 422, lookup failure, deleted successor)
- [x] Error handling in place (errors surfaced, not swallowed) (a lookup failure is logged and answered as the normal miss; gate errors keep writeGateError)

## Test Quality

- [x] Using fixture builders or factories for test data (newRetiringApp, retireIDs)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Atlas on rela develop + this branch, PostgreSQL backend, port 8891, after
converting topics in TASK-WVDXT:

- Browser: /s/projecten/entity/topic/TOPIC-AOWJ ends on /s/projecten/entity/initiatief/INIT-P2HD with heading "Website vernieuwen".
- GET /api/v1/topics/TOPIC-AOWJ: 307, Cache-Control: no-store, Location: ../initiatieven/INIT-P2HD.
- GET /api/v1/_views/topic/TOPIC-33YU?world=actueel: 307, Location: ../opdracht/OPDR-HRYW?world=actueel.
- GET /api/v1/_views/topic/TOPIC-NEVER1?world=actueel: 422 as before.
- go test ./internal/dataentry/... ./internal/archguard/... ./internal/visibility/...: pass. Golden parity: misses add only Successors=1, bodies unchanged.
- Frontend: 6013 tests pass, typecheck clean.

## Quality

- [x] Code follows project patterns (check similar code) (guard-annotated helpers with a miss hook, like readAddressedOr404)
- [x] Checked for DRY opportunities (readAddressedOr404 and the GET share readAddressedOr; views share gateReadOr)
- [x] No security issues introduced (successor gated by row, face and world; hidden successor answers byte-identical miss; 307 no-store)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A as written: a failed retired-id lookup is logged and deliberately not returned, because the redirect is a convenience and the miss answer is always correct; pinned by TestRetiredRedirect_LookupFailureIsAMiss)
- [x] No debug code left behind
