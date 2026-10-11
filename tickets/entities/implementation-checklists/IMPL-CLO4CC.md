---
id: IMPL-CLO4CC
type: implementation-checklist
title: 'Implementation: Realtime 2/1c: lease MCP tools, show_entity lease, watch wake'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (argument parsing, error text mapping, ClaimRequest.Type ordering in the service)
- [x] Integration tests written (test full flow, not just units) (tools_lease_test.go drives the MCP server over a temp project with acl.yaml and a real lease service)
- [x] Happy path implemented
- [x] Edge cases from planning handled (absent vs empty status, null and fractional ttl, ID@face and malformed addresses, nil Leases, reload)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (shared lease server fixture and principal helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the integration tests drive the real MCP server end to end; a CLI demo lands with TKT-B610IP)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (watch-on-release test checked to fail without the lease wake case)

**Verification Evidence:** AC1: claim not-found text equals show_entity for
hidden, nonexistent, type mismatch, ID@face and malformed. AC2: forbidden text
without holder fields. AC3: lease held JSON with holder, since, expires_at,
status. AC4: one lease-lost text for expiry, revocation, delete,
delete+recreate; release returns released. AC5: show_entity lease without
expires_at via one GetReadable. AC6: watch ends on claim, status renew, release
and lapse; a plain renew times out. AC7: log capture shows no lease id. AC8:
golden tool list updated; descriptions flag status as untrusted holder text.
AC9: nil Leases registers no tools, no lease field, watch still wakes on writes.
AC10: Claim runs Family, type check, WriteTarget, authorize.

## Quality

- [x] Code follows project patterns (check similar code) (Comments optional-dep pattern; consumer-side LeaseService interface)
- [x] Checked for DRY opportunities (claim not-found reuses entityReadFailed)
- [x] No security issues introduced (security review: one nit, fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
