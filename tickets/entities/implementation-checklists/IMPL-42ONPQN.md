---
id: IMPL-42ONPQN
type: implementation-checklist
title: 'Implementation: Tenant review follow-ups: validate org_id on the request path, fix upgrade guide and plan scope'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the change is inside SchemaFor; tenanthost tests already pin the ErrUnknownTenant mapping, and the postgres-tagged tenant tests still pass)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: plain org id strings)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test ./internal/tenant/ ./internal/tenanthost/
./cmd/rela-server/` passes, with and without `-tags postgres`. Reverting
SchemaFor to the empty-only check makes TestSchemaFor and
TestClusterResolver_RefusesInvalidOrgID fail. The guide's shell recipe gives
the same 16 hex characters as SchemaFor (sha256 of the UTF-8 bytes); the SQL
uses the same digest. `just docs` regenerated docs/postgres-backend.md.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
