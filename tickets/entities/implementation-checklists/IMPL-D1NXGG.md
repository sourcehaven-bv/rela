---
id: IMPL-D1NXGG
type: implementation-checklist
title: 'Implementation: Realtime 2/1b: lease service with access checks, rename/delete hooks, wake'
started: "2026-10-11"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (service with fake store/gate/authorizer/clock; wake hub; Watch)
- [x] Integration tests written (test full flow, not just units) (appbuild TestLeases_RealGate over a temp fs project with acl.yaml; backend wiring tests for memory, sqlite and postgres builds)
- [x] Happy path implemented
- [x] Edge cases from planning handled (re-claim by holder, renew at expiry, concurrent rename during renew, multi-face renew, cancelled ctx in delete hooks, failing NextExpiry)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (fake gate/store helpers, principal helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no user surface until TKT-27XVYS/TKT-4YPG4Q; the real-gate integration test drives the service through appbuild)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (postgres lease and appbuild tests run against a local database with the required flag)

**Verification Evidence:** AC1-AC2: hidden and nonexistent claims return the
same store.ErrNotFound; ambiguity names readable faces only; read-without-update
returns acl.ForbiddenError before any store call. AC3: renew after revocation,
delete, delete+recreate returns ErrLost and releases. AC4: release returns nil
for own/foreign/unknown. AC5: crypto/rand 128-bit ids, store gets sha256, log
capture has no lease id or status. AC6: validators shared with the store. AC7:
rename moves, delete drops (also with a cancelled ctx), face delete keeps. AC8:
Subscribe fan-out and coalescing; Watch wakes on a remote write and on a real
lapse, not on a plain renew; started only by rela-server, desktop and rela mcp.
AC9: constructors reject nil. Mine is row-gated against the caller's ceiling.

## Quality

- [x] Code follows project patterns (check similar code) (piles.Service hooks, consumer-side Gate/Authorizer, appbuild package accessor)
- [x] Checked for DRY opportunities (validators shared with the store; one wake hub reused across reloads)
- [x] No security issues introduced (security review: 1 significant and 3 minor findings fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
