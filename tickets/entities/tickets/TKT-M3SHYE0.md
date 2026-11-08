---
id: TKT-M3SHYE0
type: ticket
title: 'tenant: scheduler, CLI and stdio MCP in a multi-tenant deployment'
kind: enhancement
priority: medium
effort: m
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

## Description

In tenant mode the HTTP path gets its org from the verified assertion. The other
entry points have no org: the scheduler stamps a plain principal
(`scheduler.go`), and the CLI and stdio MCP build principals from literals.
RES-D54281 asks for a deliberate decision on what each means in a multi-tenant
deployment.

## Approach

- **CLI**: an operator command selects a tenant by `org_id` with `--tenant
<org_id>` or `$RELA_TENANT`, through the same resolver as the server. It never
takes a DSN on the command line. In tenant mode a command that touches data
fails without a tenant, and an unknown or erased tenant fails closed. The tenant
opens through `tenant.OpenOne`, as the server opens it.
- **stdio MCP**: same rule as the CLI.
- **db commands**: pin the tenant's schema (`tenant.Pin`) before they act.
- **rela-docs**: refuses a project with `tenants.yaml`.
- **Resolver boundary**: the entry points see only `tenant.Resolver`, through
`tenant.Select` and `tenant.OpenOne`. Whether a project serves tenants is
decided in one function, `tenant.ForProject`, the only new code that reads
`tenants.yaml`. TKT-HF5KE12 replaces it with a resolver derived from `org_id`.
- **Remote MCP over HTTP**: goes through the multi-tenant host, so the org comes
from the assertion.

## Deviations from the first plan

- **Scheduler**: one process per tenant (`rela-postgres --tenant <org_id>
scheduler`), not one loop over all tenants. A single loop needs tenants that
stay open with their queue workers past eviction, which belongs to TKT-N3X235N.
Run state, queue and audit sink are the tenant's own.
- **No `org_id` claim on CLI, scheduler or MCP principals.** The principal is
bound at the edge with `appbuild.IdentityBinder` over the tenant's services, so
it resolves in that tenant's graph. The store handle is the isolation boundary.

## Acceptance criteria

1. A scheduled task in a two-tenant deployment writes only into the tenant it
ran for. Pinned by a postgres test.
2. The CLI and stdio MCP refuse data access in tenant mode without an explicit
tenant, and an unknown tenant fails closed.
3. No new command-line flag carries a DSN.
