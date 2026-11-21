---
id: IMPL-GEB9IRQ
type: implementation-checklist
title: 'Implementation: Tenant management API behind the edge'
started: "2026-11-21"
completed: "2026-11-21"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no pratique edge and hub stack locally; TestTenantAPIOperatorAssertion drives real ES256 operator assertions through the server's verifier and tenant host, and the postgres tests run every control-table change against a real database)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./internal/tenant/ ./internal/tenanthost/ ./cmd/rela-server/`: pass.
- `go test -tags postgres` with a local PostgreSQL for `internal/tenant`, `internal/tenanthost`, `internal/appbuild`, `cmd/rela-server`: pass, except `TestTenantMode_FailsClosedOverRealTenants`, which fails locally with 404 on `/` because the worktree has no built frontend (unrelated; CI builds it).
- Covered: operator auth (missing role, wrong principal type, operator with an org), create/status/PATCH/DELETE/usage, every refusal mapping, read-only method and path rules including escaped and unclean paths, archive and operator resume, usage without an audit log, control-table upgrade from the previous shape, lease carries the current state.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
