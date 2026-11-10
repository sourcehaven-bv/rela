---
id: PLAN-W11X48M
type: planning-checklist
title: 'Planning: tenant: derive tenant location from org_id; remove tenants.yaml'
started: "2026-11-09"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: derived schema and DSN from org_id (ClusterResolver), control
table `rela_control.tenants` with status, Provisioner and `rela-server tenant
create|show|suspend|resume|erase`, tenant mode switch (flag/env), max_open and
max_resident as flags/env, revocation of open tenants, removal of tenants.yaml
and its readers, docs. Out: webhook provisioning (TKT-TNPRV8), per-org DSN
override (documented as a future decorator with the DSN in the secret store),
revalidation in long-running CLI processes (follow-up).

**Acceptance Criteria:**
1. Two orgs resolve to different schemas: TestSchemaFor, TestTenantMode_CreateServeSuspendErase.
2. Unknown, suspended, erasing orgs are refused: TestClusterResolver_Resolve, TestProvisioner_Lifecycle, TestTenantSelect_RefusesUnservedTenants.
3. A collision is refused: TestClusterResolver_Collision, TestProvisioner_CollisionRefused.
4. Create, serve, erase end to end on postgres: TestTenantMode_CreateServeSuspendErase.
5. Nothing reads tenants.yaml: TestNothingReadsTheTenantMap, TestRefuseLegacyMap.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design was decided by Jeroen in the ticket)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: schema-per-tenant on pgx is already in the codebase)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Reuses pinSchema/dropSchema and the advisory lock class
from TKT-D6MF3SW, pgstore.MigrateDSN, the Registry, and MapResolver as a test
double. No new library.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Schema `org_` + 16 hex of sha256(org_id); DSN is
RELA_DATABASE_URL with search_path replaced. The control table lives in
`rela_control`: not in public (on every tenant's search_path; erase refuses
while public holds tables), not in a tenant schema (circular; erase would drop
it), same database (no second credential). Status changes are compare-and-set.
The registry resolves on every Acquire and on a 30s Revalidate; a refusal
revokes the resident and ends its streams. Erase sets `erasing` first.
Alternatives rejected: keeping tenants.yaml (manual sync), status in a tenant
schema (circular), a cached resolver (stale status after suspend).

**Files to modify:** internal/tenant/*,
cmd/rela-server/{main,tenantmode,tenantcmd}.go,
internal/cli/{tenant,kong,db_postgres,mcp_wiring}.go, cmd/rela-docs/main.go,
internal/dataentry/webhook.go, internal/tenanthost/host.go, docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** org_id from a verified JWT or the operator CLI:
checked by CheckOrgID (printable, bounded length), never used in SQL text; the
schema name is a hash checked against schemaNamePattern. RELA_DATABASE_URL from
env only.

**Security-Sensitive Operations:** DROP SCHEMA under the exclusive advisory
lock, with refusal of public and reserved schemas, the symlink check on the
audit directory, typed confirmation and the erasure record. No DSN on a command
line or in a log.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. Postgres tests use
backendtest.Database with a schema-pinned DSN.

**Edge Cases:** Empty org id; existing schema without row (needs -adopt); resume
of an erasing tenant; in-use resident revoked; status change during open; lookup
failure (no revocation, 503).

**Negative Tests:** Bad org ids, legacy tenants.yaml present, tenant database
reached without tenant mode, --tenant without tenant mode.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Upgraded deployments silently falling back to public: refused by
RefuseLegacyMap and RefuseTenantDatabase. Legacy schema names: documented rename
plus `create -adopt`. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] GUIDE-postgres-backend "Tenant mode"
- [x] GUIDE-acl-security tenant mode paragraph

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-093RB8L, RR-6X65228, RR-7EKSK1I, RR-7IKDW48,
RR-CIT6N1T, RR-F000CFO, RR-J1CFYRR, RR-JU11R8Q, RR-KK0FWWO
