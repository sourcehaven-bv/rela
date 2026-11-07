---
id: TKT-M3SHYE0
type: ticket
title: 'tenant: scheduler, CLI and stdio MCP in a multi-tenant deployment'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

In tenant mode the HTTP path gets its org from the verified assertion. The other
entry points have no org: the scheduler stamps a plain principal
(`scheduler.go`), and the CLI and stdio MCP build principals from literals.
RES-D54281 asks for a deliberate decision on what each means in a multi-tenant
deployment.

## Approach

- **Scheduler**: one loop over all tenants, not one loop per tenant. Each run
acquires that tenant's lease and runs as a verified principal carrying the
tenant's `org_id`. Run state stays in the tenant's schema.
- **CLI**: an operator command selects a tenant by `org_id` through the same
resolver (for example `--tenant <org_id>`). It never takes a DSN on the command
line. Without a tenant in tenant mode, a command that touches data fails.
- **stdio MCP**: same rule as the CLI.
- **Remote MCP over HTTP**: goes through the multi-tenant host, so the org comes
from the assertion.

## Acceptance criteria

1. A scheduled task in a two-tenant deployment writes only into the tenant it
ran for. Pinned by a postgres test.
2. The CLI and stdio MCP refuse data access in tenant mode without an explicit
tenant, and an unknown tenant fails closed.
3. No new command-line flag carries a DSN.
