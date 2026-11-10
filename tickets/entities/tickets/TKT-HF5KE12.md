---
id: TKT-HF5KE12
type: ticket
title: 'tenant: derive tenant location from org_id; remove tenants.yaml'
kind: enhancement
priority: high
effort: m
started: "2026-11-09"
completed: "2026-11-10"
status: done
---

## Description

Tenant location is derived from the verified `org_id`, not read from an operator
file. Pratique is the authority on which orgs exist; rela derives where their
data lives. `tenants.yaml` duplicates that and must be kept in sync by hand.

## Approach

- `tenant.ClusterResolver` implements `tenant.Resolver`. Input is the verified `org_id`; output is `Tenant{OrgID, Schema, DSN}` or `ErrUnknownTenant`.
- Schema is `org_` plus a short hash of `org_id`, validated against `schemaNamePattern`. DSN is `RELA_DATABASE_URL` with `search_path` pinned.
- A tenant exists when its schema exists and its status in a small control table is `active`. Missing, `suspended` or `erasing` fails closed.
- Lifecycle writes go on a separate `Provisioner` interface (`Create`, `Erase`), used by the webhook and the CLI, never by the request path.
- `tenant erase` uses the `erasing` status instead of the `erase: true` marker.
- `max_open` moves to an env var or server flag.
- Remove `tenants.yaml`, `tenant.Config` and `LoadConfig`. Keep `MapResolver` as a test double only.
- A per-org DSN override for a dedicated cluster is out of scope; when needed it is a decorator around `ClusterResolver`, with the DSN in the secret store.

## Acceptance criteria

1. No `tenants.yaml` is read anywhere.
2. Two orgs resolve to distinct schemas; an org without a schema, or with a non-active status, is refused.
3. A hash collision or an invalid derived name is refused, never shared.
4. Host, erase and the scheduler/CLI/MCP entry points all resolve through `ClusterResolver`.
5. Docs (GUIDE-postgres-backend) describe the derived model.
