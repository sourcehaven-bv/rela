---
id: PLAN-ELBG27K
type: planning-checklist
title: 'Planning: Tenant management API behind the edge'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: replace the tenant API's own token check with the edge-assertion
and role model; PATCH state (active, read_only, suspended), plan and
entitlements; DELETE archives; GET usage; read-only enforcement at the tenant
host; plan fields on GET /api/v1/_tenant; control-table columns and upgrade;
docs. Out: hard delete through the API, enforcing entitlements (max_users is
documented as a candidate), listing tenants, pratique changes (separate PR),
saas-dev wiring.

**Acceptance Criteria:**
1. Only an operator assertion (principal_type operator, no org_id, role held) passes: tenanthost TestTenantAPI_Auth, cmd TestTenantAPIOperatorAssertion with real ES256 tokens.
2. PATCH rules and held suspensions: TestProvisioner_Apply (postgres), TestTenantAPI_Patch, TestTenantAPI_PatchRefusals.
3. Read-only: TestReadOnly, TestRegistry_LeaseCarriesCurrentState.
4. Archive: TestProvisioner_Archive, TestTenantAPI_Archive.
5. Usage: TestClusterResolver_Usage, TestTenantAPI_Usage.
6. Plan on the status path: TestTenantAPI_StatusShowsPlan.
7. Docs: docs/postgres-backend.md and its guide.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: design decided by the product owner before the ticket)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: extends the in-tree tenant host and control table)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the edge model already serves users)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Tenant API of TKT-84CD0PY (internal/tenanthost/tenantapi.go): routing, create through the provisioning runner.
- IdP suspension (TKT-TNIDPS): suspended_by causes and compare-and-set transitions in provision_postgres.go; the new api and archive causes follow it.
- Registry.RevalidateOrg closes a tenant at once after a host-made change, as status sync does.
- The data-entry JWT adapter already projects principal_type and roles.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Control table gains read_only, plan, entitlements
(jsonb), archived_at and the causes api and archive; the upgrade adds them and
rebuilds the CHECK. Provisioner.Apply and Archive change a row in one
transaction with SELECT FOR UPDATE. ClusterResolver.Usage reads the schema's
user entities, audit log and relation sizes. The tenant host verifies the API's
assertion with the host verifier and checks principal_type, org and role;
TenantControl is a consumer-side interface the wiring implements with lifecycle
logging and RevalidateOrg. The lease carries the freshly resolved tenant, so the
wiring wraps the handler in tenanthost.ReadOnly per request. Alternatives
rejected: a second token audience (nothing can mint it); per-tenant ACL for
read-only (would need every tenant's acl.yaml to cooperate); a separate state
column (status plus cause and flags keeps one served status).

**Files to modify:** internal/tenant (cluster*.go, tenant.go, manage*.go,
provision_postgres.go, registry.go, dsn_nopostgres.go), internal/tenanthost
(host.go, tenantapi.go, readonly.go, provision_http.go),
internal/appbuild/appbuild.go, cmd/rela-server (tenantmode.go, tenantapi.go,
tenantcmd.go), docs/postgres-backend.md, docs-project guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The assertion (verified by the host verifier;
then allowlist checks on principal_type, empty org and role); the org id in the
path (tenant.SchemaFor); the PATCH body (16 KiB cap, unknown fields refused,
states from a closed set, entitlement names by regex, scalar values only, size
bounds); the flag -tenant-api-role (regex, needs provisioning).

**Security-Sensitive Operations:** Tenant suspension, archive and plan writes.
Only the role-holding operator principal reaches them; an operator assertion
carries no org, so the host refuses it on every tenant path. The API cannot undo
an operator, IdP or archive suspension. Database errors are logged, never sent.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria. Host tests go through
Host.ServeHTTP; the postgres tests run the provisioner, resolver and registry on
a real database.

**Edge Cases:** Suspending a suspended tenant keeps its cause; archiving twice
is a no-op; a provisioning tenant takes a plan but not a state; an erased tenant
refuses changes; a recreated tombstone starts with no plan; an old control table
reads as it is and is upgraded on the first write; path traversal around the
read-only exceptions.

**Negative Tests:** User, operator-in-org and role-less assertions get 403; bad
or missing tokens 401; malformed PATCH bodies 400; held and archived tenants
409; absent orgs 404; database errors 503 without detail.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Control-table upgrade on a live cluster: additive columns, IF NOT
EXISTS, under the control DDL lock. Old servers read the table through to_jsonb,
so a mixed rollout works. Read-only scope may refuse a write a UI needs
(presence, leases): documented, and a read-only tenant is a billing state.
Effort l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/postgres-backend.md (tenant API section, webhook-mode row) and GUIDE-postgres-backend

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None recorded; the design was set by the product
owner, and the cross-check with the pratique contract is in the PR.
