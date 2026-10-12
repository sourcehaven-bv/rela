---
id: IMPL-RFHDCK
type: implementation-checklist
title: 'Implementation: Realtime 2/2: presence over live views, avatars and claim badge'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (internal/presence tracker: refcount, grace, caps, sweep, LeaveConn; 98.9% coverage)
- [x] Integration tests written (test full flow, not just units) (live SSE tests through the router with a fake clock and the hub tick counter)
- [x] Happy path implemented
- [x] Edge cases from planning handled (stale replays, churn by one identity, overflowed rounds, skipped replays rescheduled, face moves, Unknown principal)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (presence env helpers, counting replay handler)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the SPA lands with TKT-IMBF7U; router-level SSE tests drive the full stack)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (presence tests pass 50 of 50 runs under race; the reschedule test fails without its fix)

**Verification Evidence:** AC1: 404 parity for hidden, absent, wrong plural,
malformed and world cases; no viewer recorded. AC2: join within two ticks; grace
leave at 15 s on the fake clock; rejoin keeps since. AC3: revocation removes the
viewer when its replay next runs; skipped replays are rescheduled within
ceil(n/256) ticks. AC4: TestPresence_RoundReplaysOnlyThatFace; hidden subscriber
gets no frame. AC5: merging, caller exclusion across tools, cap 20 with more.
AC6: Unknown never joins; 64 per identity including grace before 4096. AC7:
Retire discards presence. AC8: TestPresence_ViewEntryLease. AC9: router walk and
OpenAPI. AC10: S1, S2, M1, M2, M4 tests present (security review verified).

## Quality

- [x] Code follows project patterns (check similar code) (live hub rounds, readAddressedOr404, writeTargetOr404)
- [x] Checked for DRY opportunities (shared read gate; one tracker per hub)
- [x] No security issues introduced (security review: two minor fixed; commit check fail-open fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
