---
id: PLAN-Q1S9VPL
type: planning-checklist
title: 'Planning: tenant: scheduler, CLI and stdio MCP in a multi-tenant deployment'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: tenant selection for the operator entry points of the `rela` binary: every
command that opens project data (`show`, `list`, `create`, ..., `scheduler`,
`flow`, `validate`, `mcp`) and the postgres `db` commands. A global `--tenant
<org_id>` flag (env `RELA_TENANT`) names the tenant. The scheduler is scoped as
one process per tenant (`rela --tenant X scheduler`).

Out: a single scheduler loop over all tenants (needs tenant residents that
outlive eviction, TKT-N3X235N); `rela-docs` (a docs builder, not an operator
entry point for tenant data); remote MCP over HTTP (goes through the tenant host
already); provisioning (TKT-TNPRV8).

**Acceptance Criteria:**
1. A scheduled task in a two-tenant deployment writes only into the tenant it
ran for, and its run state lands in that tenant's schema only. Postgres test
drives `rela scheduler`'s command over the tenant opener.
2. In tenant mode, a data command without `--tenant` fails before any store is
opened; an unknown or erased org fails closed with `ErrUnknownTenant`. Unit
tests on the opener and on the command gate; stdio MCP goes through the same
opener.
3. No new flag carries a DSN; the tenant DSN never reaches a log or error.
4. Without `tenants.yaml` every command behaves as today; `--tenant` without
`tenants.yaml` is refused rather than ignored.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: RES-D54281 already covers it)
- [x] ~~Searched for existing libraries~~ (N/A: internal wiring only)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal wiring)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-D54281

**Existing Solutions:**
- `tenant.SharedBaseOpener` (pins the schema, takes the schema lock shared,
per-tenant audit sink under `.rela/audit/tenants/<schema>/`), reused as is.
- `appbuild.DiscoverSharedBase` + `SharedBase.OpenAndAssemble`, reused.
- `cmd/rela-server/tenantmode.go`: `tenantsPath` and the base-DSN-from-env
rule move into `internal/tenant` so the server and the CLI share them.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `internal/tenant/entry.go`: `ConfigPath(root)`, `LoadDeployment(path,
getenv)` (base DSN from `$RELA_DATABASE_URL` when the file has none),
`Select(path, orgID, getenv)` (fails closed: `ErrTenantRequired`,
`ErrUnknownTenant`), `OpenOne(ctx, base, t)` (one `SharedBaseOpener` call).
- `internal/cli/tenant.go`: `openProject(startDir, org, opts...)` returns the
services and the closer. No `tenants.yaml`: `appbuild.Discover` exactly as today
(and `--tenant` refused). With it: select, `DiscoverSharedBase`, `OpenOne`;
close via `Opened.Close` so the schema lock is released after the store. Every
`appbuild.Discover` in `internal/cli` goes through it; a source scan test pins
that.
- A gate in `runKong` refuses a data command in tenant mode before dispatch,
so a command with its own store access (`db`, sqlite `db`) cannot skip it.
- `db` (postgres): `resolveDSN` returns the selected tenant's DSN in tenant
mode.
- Identity: `appbuild.IdentityBinder(svc)` over the tenant's services, so a
principal is resolved in that tenant's graph only. No org claim is minted:
isolation is the store handle, and the audit sink is per tenant.

Alternative rejected: one scheduler loop over all tenants via the registry. Each
run would enqueue on a tenant's durable queue, whose worker lives only while the
tenant is resident; eviction mid-run is the open TKT-N3X235N.

**Files to modify:** internal/tenant/{entry.go,dsn_*.go}, internal/cli/{kong.go,
tenant.go,mcp_wiring.go,mcp.go,flow.go,validate.go,db_postgres.go},
cmd/rela-server/tenantmode.go, docs-project guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `--tenant`/`RELA_TENANT` (operator shell) is
looked up in the validated tenant map; nothing else is derived from it.
`tenants.yaml` is validated by the existing loader (no passwords, schema
pattern, unique schemas).

**Security-Sensitive Operations:** opening a tenant store goes through
`SharedBaseOpener` (schema check + shared schema lock). Errors name the org id,
never the DSN.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: postgres test, two tenants, scheduler command on tenant
A, note and run state only in A. AC2: tenant + cli unit tests, MCP opener test.
AC3: test that errors do not contain the DSN. AC4: cli test that the no-tenants
path equals Discover and `--tenant` alone is refused.

**Edge Cases:** erased tenant, empty org, `tenants.yaml` on a non-postgres
build, flow script outside the cwd project.

**Negative Tests:** missing tenant, unknown tenant, `--tenant` without a map.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a new CLI path that calls `appbuild.Discover` directly would skip
tenant mode; mitigated by the source scan test and the runKong gate. Effort m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-postgres-backend tenant section (operator entry
points); CLI `--tenant` flag.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** recorded on the ticket as review-responses.
