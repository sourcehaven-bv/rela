---
id: IMPL-LVKUU1
type: implementation-checklist
title: 'Implementation: Realtime 2/1d: lease HTTP routes, _lease on responses, live wake'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (error mapping, strict decoding, ttl and status handling, attach points)
- [x] Integration tests written (test full flow, not just units) (NewRouter with a real lease service over a temp project with acl.yaml; live SSE tests for lease wakes)
- [x] Happy path implemented
- [x] Edge cases from planning handled (wrong plural, faced bare address, absent vs empty status, not-wired service, misspelled fields, token clients)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (lease router fixture, installLeases helper)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the router-level tests drive the full HTTP stack; the SPA badge lands with TKT-IMBF7U)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (stale 304 after release; cross-origin cookie POST refused; origin-less token request allowed)

**Verification Evidence:** AC1: TestLeaseClaim_MissIsTheEntityGet404 covers
hidden, nonexistent, wrong plural, malformed and not wired; faced bare address
gives 422 face_required. AC2: 403 body without holder fields; affordance
contract case. AC3: 409 lease_held with holder, since, expires_at, status. AC4:
lease_lost uniform; release 204; limit 409; full 503 with Retry-After 30;
malformed JSON and unknown fields 400. AC5: _lease only at the two attach
points, pinned by a grep guard; views, history, includes absent. AC6: counting
lease store shows one Get per page at 10 and 50 rows. AC7: plain renew keeps the
live hash; claim, status change, release and expiry each one live:changed. AC8:
TestLive_LeaseOnHiddenEntitySendsNoFrame. AC9: no 304 while leases are wired;
If-Match unaffected. AC10: TestLease_IDNeverLeaks over GET, list, SSE and the
access log. AC11: cookie plus foreign Origin refused on all lease POSTs;
/api/v1/_leases exempt only for the non-browser shape (decision recorded on the
review response). AC12: router walk and OpenAPI tests cover all four routes.

## Quality

- [x] Code follows project patterns (check similar code) (pilesHandler, serveListRowExtras, writeWriteTargetError)
- [x] Checked for DRY opportunities (shared 404 writer; one GetReadable per page)
- [x] No security issues introduced (security review: one minor, fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
