---
id: IMPL-WXAUMB
type: implementation-checklist
title: 'Implementation: Gate _views read path through the ACL read gate (TKT-VQGN follow-through)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestACLViews_GatesHiddenEntity)
- [x] Integration tests written — the test drives handleV1Views end-to-end through the gated context
- [x] Happy path implemented (gateReadOrNotFound before executeView)
- [x] ~~Edge cases from planning handled~~ (N/A: speed-run refactor, no planning phase; the gate's own edge cases — denied/missing/visible — are covered in the test)
- [x] Error handling in place (deny → 404, store failure → writeGateError — same as handleV1GetEntity)

## Test Quality

- [x] Using fixture builders or factories for test data (seedEntity, mustNewACL)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: assertions check a fixed not_found code + leak markers)
- [x] ~~Property comparisons use original object~~ (N/A: test asserts absence of leaked title/content, not property equality)

## Manual Verification

- [x] Feature manually tested end-to-end (live pen-test `.ignored/acl-pentest`: mallory `_views` → 404 after fix; full disclosure before)
- [x] Each acceptance criterion verified with test scenario
- [x] Edge cases manually verified (visible→200, denied→404 no leak)

**Verification Evidence:**
Before fix: `GET /api/v1/_views/ticket/TKT-1` as a zero-grant principal returned
the full entity incl. content body. After fix: 404 not_found, no title/content
in the body. Confirmed against the live Caddy-fronted server and the unit test.

## Quality

- [x] Code follows project patterns (reuses gateReadOrNotFound, the established read-chokepoint helper)
- [x] ~~DRY opportunities~~ (N/A: the shared helper already exists; this calls it)
- [x] No security issues introduced (this closes one)
- [x] No silent failures (deny → explicit 404; store error → writeGateError)
- [x] No debug code left behind
