---
id: IMPL-X1JVUXV
type: implementation-checklist
title: 'Implementation: Tenant API for an external provisioning job'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (tenanthost/tenantapi_test.go; TestValidateTenantAPI, TestTenantAPIVerifier)
- [x] Integration tests written (test full flow, not just units) (host tests go through Host.ServeHTTP, the provisioning runner and the control fake; TestTenantAPIVerifier signs real ES256 tokens against a loopback JWKS)
- [x] Happy path implemented
- [x] Edge cases from planning handled (bare prefix, nested path, long org id, running create, part-way create, suspended, erased, API off)
- [x] Error handling in place (errors surfaced, not swallowed) (rejections and lookup failures are logged; clients get a problem code)

## Test Quality

- [x] Using fixture builders or factories for test data (newAPIHost, callAPI, apiIssuer)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (real ES256 tokens through the production verifier; host flow through ServeHTTP)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
go test ./internal/tenanthost/ ./cmd/rela-server/ pass, and go test -race -run TestTenantAPI ./internal/tenanthost/. TestTenantAPIVerifier signs tokens with a throwaway ES256 key served from a loopback JWKS: a tenant API token is accepted with its org_id and jti; a user assertion, a webhook token, another issuer, a missing exp and an expired token are refused. Host tests: PUT gives 201 then 200, a slow create 202 with Location and Retry-After then GET provisioning then active; refusals never reach the provisioner. A full Postgres run was not done locally.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors the IdP webhook receiver and its verifier)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (security review recorded as review responses)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
