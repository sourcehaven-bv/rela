---
id: PLAN-9RTK3LE
type: planning-checklist
title: 'Planning: tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: eager provisioning from the IdP `organization.created` webhook,
lazy provisioning on a verified request of an unknown org, single-flight, 202
plus a polled status endpoint, rate limit, erased-org tombstone, the written
trust assumption. Out: a second issuer (the lazy path must be switched off
before one exists).

**Acceptance Criteria:**
1. Webhook for a new org creates the tenant and answers 200 only once it exists (`TestHost_WebhookProvision`).
2. A verified request of an unknown org starts provisioning and gets 202 with Location `/api/v1/_tenant/status` (`TestHost_LazyProvision`, `TestTenantMode_LazyProvision`).
3. Concurrent requests share one create (`TestAutoProvisioner_SingleFlight`, `TestAutoProvisioner_Concurrency`).
4. An erased org is never provisioned automatically (provision postgres tests, ErrTenantErased).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (RES-D54281 specifies the shape)
- [x] Searched for existing libraries that solve this problem (none needed; standard library and existing rela packages)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-internal lifecycle work)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-D54281 specifies the shape

**Existing Solutions:** Builds on the tenant registry
(`internal/tenant/registry.go`), `tenant.Provisioner`, `state.KV` and the flow
run stores.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `tenant.AutoProvisioner`
(internal/tenant/autoprovision.go) single-flights creates per org, rate-limits
them and reports ready/provisioning/failed/throttled. Modes come from
`-tenant-provision`: off, webhook or lazy. tenanthost answers 202 with a
`tenant_provisioning` problem and serves `GET /api/v1/_tenant/status`; the SPA
polls it (frontend/src/api/tenantProvisioning.ts). The webhook waits up to 10s
and answers 503 while the create is not done. `rela_control.erased` records
erased orgs; only operator `tenant create` clears it. Rejected: holding the
request open across the migration (timeouts leave a half-created tenant).

**Files to modify:** internal/tenant/autoprovision.go, provision_postgres.go,
cluster*.go, registry.go; internal/tenanthost/host.go;
cmd/rela-server/tenantmode.go; frontend/src/api/tenantProvisioning.ts,
client.ts, App.vue; GUIDE-postgres-backend.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The org id comes only from a verified assertion
or a verified webhook JWT; it is checked against the tenant id rules before use.
Trust assumption (one issuer, pinned audiences, org_id always present) is
written on `tenant.AutoProvisioner` and in the guide. Rate limit bounds schema
creation. Tombstone stops re-creation of an erased org.

**Security-Sensitive Operations:** See above. Errors name the failing step; DSNs
and tokens are not logged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Unit tests on AutoProvisioner outcomes, wait, rate limit,
close; tenanthost tests for lazy, status and webhook; postgres tenant-mode test
end to end; vitest for the SPA poller.

**Edge Cases:** Create fails (status failed, retry after backoff); another
server creates the same tenant (ErrTenantInUse, job held 10s); erased org (403);
bad org id; provisioner closed during a create; 401/403 while polling (SPA
reloads).

**Negative Tests:** Covered in the edge cases above: failing stores, refused
orgs, closed provisioner or sweeper.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Schema creation by anyone who can mint an assertion: mitigated by the
documented single-issuer assumption, rate limit and tombstone. Slow migrations:
202 plus polling instead of a held request.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-postgres-backend: provisioning modes, status
endpoint, tombstone, trust assumption.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (skipped: no separate design review was run; the finished change went through /code-review, findings recorded as review-responses)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review findings; code review findings are all addressed)

**Design Review Findings:** None (see review checklist for code review
findings).
