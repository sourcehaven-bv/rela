---
id: PLAN-58AJ320
type: planning-checklist
title: 'Planning: Tenant API for an external provisioning job'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: a host-level `PUT`/`GET`/`HEAD /api/v1/_tenants/<org_id>` in internal/tenanthost, a `-tenant-api-audience` flag, boot checks, docs. Out: listing tenants, suspend or erase through the API (operator CLI only), a new credential scheme, client tooling.

**Acceptance Criteria:**
1. PUT creates: TestTenantAPI_CreateThenRead (201, then 200), TestTenantAPI_SlowCreateIsAccepted (202 with Location and Retry-After).
2. GET reads: TestTenantAPI_States, TestTenantAPI_CreateThenRead (404 before the create).
3. Auth: TestTenantAPI_Auth (no token, bad token, user assertion, other org; the provisioner never runs).
4. Boot checks: TestValidateTenantAPI, TestTenantAPI_Config, TestRefuseTenantOnlyFlags.
5. Shared runner: TestTenantAPI_Failures (a retry inside the hold starts no run).
6. Docs: docs/postgres-backend.md and GUIDE-postgres-backend.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (the change reuses existing parts only)

**Existing Solutions:**
- jwtauth.NewWebhookVerifier (internal/jwtauth/verifier.go): ES256, issuer, JWKS, required exp, own audience. Reused as is with a third audience.
- The IdP webhook receiver in internal/tenanthost: create through provisioning.start with a bounded wait. The API follows its outcomes.
- StatusPath (`/api/v1/_tenant`) for the status vocabulary.
- RFC 7231 PUT semantics for an idempotent create; 202 with Location for an async create.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Host.ServeHTTP routes TenantsPath to serveTenantAPI before the assertion check when Config.TenantAPI is set. serveTenantAPI verifies the bearer token through a TenantAPIVerifier, binds it to the path org, validates the org id with tenant.SchemaFor, then reads status through Acquire or creates through provisioning.start and waits up to the webhook wait. cmd/rela-server builds the verifier from jwtauth.NewWebhookVerifier with -tenant-api-audience.

Alternatives rejected: a static bearer secret (a new scheme; the task says reuse operator auth); the operator secret of the IdP org API (that secret authenticates rela to the IdP, not the other way); mounting the API in each tenant router (no router exists before the tenant does).

**Files to modify:**
internal/tenanthost/tenantapi.go (new), host.go, provision.go; cmd/rela-server/tenantmode.go; tests; docs/postgres-backend.md and its guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Authorization header: verified JWT (ES256 only, issuer, audience, exp); otherwise 401.
- Path org id: must equal the token's org_id (otherwise 403) and pass tenant.SchemaFor (otherwise 400) before any lookup.
- Method: allowlist GET, HEAD, PUT; otherwise 405.

**Security-Sensitive Operations:**
Schema creation, guarded by the token, the per-org binding and the runner's rate limit and running cap. Database errors are logged only; the client gets 503 tenant_unavailable.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
See the acceptance criteria above. The host tests run through Host.ServeHTTP with the control-table fake, so routing, auth and the runner are tested together.

**Edge Cases:**
Empty org (bare prefix), nested path, 300-character org id, a create still running (202 then provisioning on GET), a create that stopped part way (status provisioning, finished by PUT), suspended and erased tenants, API off (path goes to the tenant as before).

**Negative Tests:**
No token, forged token, end-user assertion, token for another org, DELETE, provisioning off at boot, audience equal to the identity or webhook audience.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Token reuse across audiences: refused at boot when audiences match.
- Mass schema creation by a leaked token: one token acts on one org; the runner's start rate and cap apply.
- Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
docs/postgres-backend.md and docs-project GUIDE-postgres-backend: a "Creating tenants through the tenant API" section and the webhook-mode row.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self review against the security considerations above; no critical or significant findings. Review responses are recorded at the code review.
