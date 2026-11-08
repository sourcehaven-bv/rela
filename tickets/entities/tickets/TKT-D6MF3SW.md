---
id: TKT-D6MF3SW
type: ticket
title: 'rela-server: multi-tenant host that routes a verified org_id to its tenant''s App'
kind: enhancement
priority: high
effort: l
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

## Description

TKT-TNT9RS built `tenant.Resolver` and `tenant.Registry`, but nothing outside
tests uses them. `rela-server` still serves one store, so Pratique's verified
`org_id` selects nothing. TKT-TNT9RS left the HTTP host out of scope on purpose:
`dataentry.App` serves one store and sits at its plimsoll line, and RES-D54281
says not to put per-request tenant logic on `App`.

This ticket builds the multi-tenant host.

## Approach

- A front handler in `rela-server` that verifies the assertion once, reads
`org_id`, acquires a `tenant.Lease`, and passes the request to that tenant's
`dataentry.App`.
- One `App` per resident tenant, built on first use and closed when the
registry evicts the tenant. The lease is held for the whole request.
- Tenant mode is on when `tenants.yaml` exists. It is supported on the
postgres build only; the fs, memory and sqlite builds refuse to start when
`tenants.yaml` is present.
- Fail closed: no `org_id`, an unknown org or a failed lookup returns an error
response and never reaches a store. Log unknown-org and lookup failure
differently (`tenant.ErrUnknownTenant`).
- Routes that need no tenant (health checks, static assets, the IdP
webhook) are decided explicitly and listed.

## Out of scope

- Provisioning (TKT-TNPRV8) and erasure (TKT-TNERAS).
- Scheduler, CLI and MCP (separate ticket).
- A shared connection pool across tenants.

## Acceptance criteria

1. Two orgs in `tenants.yaml`, one process: an entity written by a user of org
A is not visible to a user of org B through the HTTP API, search, live updates
or attachments. Pinned by a postgres test.
2. A request with no `org_id` or an unknown `org_id` gets no data and opens no
store.
3. Evicting a tenant while a request is in flight does not close its store
under that request.
4. A non-postgres build with `tenants.yaml` present fails at boot.
5. `docs/acl-security.md` § "`org_id` is recorded, not enforced" describes
tenant mode: isolation comes from the store handle, and `org_id` is still not an
ACL condition.
